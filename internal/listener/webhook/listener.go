package webhook

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/internal/util"
	"github.com/go-logr/logr"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defautlIdentifierPath = "0.data.ObjectName"
const defaultServerAddress = ":8090"
const defaultPath = "/webhook"
const maxPortNumber = 65535
const defaultMaxRetries = 10

type RetryMessage struct {
	event      events.SecretRotationEvent
	currentRun int
	retryAt    time.Time
}

// WebhookListener listens for webhook notifications to handle secret rotation events.
type WebhookListener struct {
	ctx        context.Context
	cancel     context.CancelFunc
	config     *v1alpha1.WebhookConfig
	eventChan  chan events.SecretRotationEvent
	server     *http.Server
	logger     logr.Logger
	client     client.Client
	retryQueue chan *RetryMessage
}

// Start initiates the WebhookListener to begin listening for incoming webhook requests.
func (h *WebhookListener) Start() error {
	h.logger.Info("Starting Webhook Listener...")

	// Only handle errors if policy is configured
	if h.config != nil && h.config.RetryPolicy != nil {
		go h.handleErrors()
	}
	go func() {
		defer h.logger.Info("Stopping Webhook Listener...")
		err := h.server.ListenAndServe()
		if err == http.ErrServerClosed {
			return
		}

		<-h.ctx.Done()
	}()
	return nil
}

// Stop gracefully shuts down the WebhookListener by closing the stopChan channel which triggers the termination process.
func (h *WebhookListener) Stop() error {
	close(h.retryQueue)
	h.stopRetryQueue()
	h.cancel()
	return h.server.Close()
}

func (h *WebhookListener) webhookHandler(w http.ResponseWriter, r *http.Request) {
	var err error
	err = h.authenticate(r.Header)
	if err != nil {
		h.logger.Error(err, "Couldn't authenticate request")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintln(w, "Couldn't authenticate request")
		return
	}

	payload, err := parsePayloadToString(r.Body)
	if err != nil {
		h.logger.Error(err, "Couldn't parse event payload")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, "Couldn't decode webhook payload. Send a valid json")
		return
	}

	err = r.Body.Close()
	if err != nil {
		h.logger.Error(err, "Error closing the payload body")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, "Couldn't decode webhook payload. Send a valid json")
		return
	}

	identifierPath := h.getIdentifierPath()

	secretIdentifier, err := getSecretIdentifierFromPayload(payload, identifierPath)
	if err != nil {
		message := fmt.Sprintf("Secret Identifier not found on payload."+
			"Ensure that your secret is on the following path: %s", identifierPath)
		h.logger.Error(err, message)

		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, message)
		return
	}

	if event, err := h.processSecret(secretIdentifier); err != nil {
		message := "Failed to process event"
		h.logger.Error(err, message)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, message)

		if h.config != nil && h.config.RetryPolicy != nil {
			h.retryQueue <- &RetryMessage{event: event, currentRun: 1, retryAt: time.Now()}
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
	_, _ = fmt.Fprintln(w, "")
}

func (h *WebhookListener) authenticate(header http.Header) error {
	if h.config == nil || h.config.Auth == nil {
		return nil
	}

	basicAuth := h.config.Auth.BasicAuth
	bearer := h.config.Auth.BearerToken

	if basicAuth == nil && bearer == nil {
		return nil
	}

	authHeader := strings.Split(header.Get("Authorization"), " ")
	if len(authHeader) != 2 {
		return errors.New("malformed authorization header. Use `Bearer <token>` or `Basic <token>`")
	}

	if basicAuth != nil && strings.Contains(strings.ToLower(authHeader[0]), "basic") {
		return authenticateWithBasicAuth(h.ctx, h.client, authHeader[1], basicAuth, h.logger)
	}

	return authenticateWithBearer(h.ctx, h.client, authHeader[1], bearer, h.logger)
}

func (h *WebhookListener) createHandler() {
	path := defaultPath
	if h.config != nil && h.config.Path != "" {
		path = h.config.Path
		if path[0] != '/' {
			path = "/" + path
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("POST %s", path), h.webhookHandler)
	h.server.Handler = mux
}

func (h *WebhookListener) getIdentifierPath() string {
	identifierPath := defautlIdentifierPath
	if h.config != nil && h.config.SecretIdentifierOnPayload != "" {
		identifierPath = h.config.SecretIdentifierOnPayload
	}

	return identifierPath
}

func (h *WebhookListener) handleErrors() {
	maxRetries := min(h.config.RetryPolicy.MaxRetries, defaultMaxRetries)
	for message := range h.retryQueue {
		beforeOrNow := message.retryAt.Compare(time.Now()) <= 0

		if beforeOrNow {
			err := h.processEvent(message.event)
			if err == nil {
				h.logger.Info(fmt.Sprintf(
					"Message for '%s' successfully processed after %d retries",
					message.event.SecretIdentifier,
					message.currentRun,
				))
				// if message was processed, drop it
				continue
			}

			if message.currentRun >= maxRetries {
				h.logger.Error(err, fmt.Sprintf(
					"Message for '%s' was not processed after %d retries",
					message.event.SecretIdentifier,
					message.currentRun,
				))
				// if message was retried (successfully or not) up to max times, drop it

				continue
			}

			message.currentRun++
			message.retryAt = getNextRetryAt(h.config.RetryPolicy.Algorithm, message.currentRun)
		}
		h.retryQueue <- message
	}
}

func (h *WebhookListener) stopRetryQueue() {
	h.logger.Info("Processing all messages left for retry ignoring algorithm")
	for message := range h.retryQueue {

		err := h.processEvent(message.event)
		if err == nil {
			h.logger.Info(fmt.Sprintf(
				"Message for '%s' successfully processed after %d retries",
				message.event.SecretIdentifier,
				message.currentRun,
			))
		} else {
			h.logger.Info("Message for '%s' was not processed")

		}
	}
}

func getNextRetryAt(algorithm string, currentRun int) time.Time {
	if algorithm == "linear" {
		return time.Now().Add(time.Second)
	}

	duration := time.Duration(math.Exp2(float64(currentRun)))
	return time.Now().Add(time.Second * duration)
}

func parsePayloadToString(body io.ReadCloser) (string, error) {
	b, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}

func getSecretIdentifierFromPayload(payload, identifierPath string) (string, error) {
	secretIdentifier := gjson.Get(payload, identifierPath)

	if !secretIdentifier.Exists() {
		err := errors.New("secret not found on event")
		return "", err
	}

	return secretIdentifier.String(), nil
}

func (h *WebhookListener) processSecret(secretIdentifier string) (events.SecretRotationEvent, error) {
	event := events.SecretRotationEvent{
		SecretIdentifier:  secretIdentifier,
		RotationTimestamp: time.Now().Format("2006-01-02-15-04-05.000"),
		TriggerSource:     schema.WEBHOOK,
	}
	return event, h.processEvent(event)
}

func (h *WebhookListener) processEvent(event events.SecretRotationEvent) error {
	select {
	case h.eventChan <- event:
		h.logger.Info("Published event to eventChan", "Event", event)
		return nil
	case <-h.ctx.Done():
		return h.ctx.Err()
	}
}

func createServer(config *v1alpha1.WebhookConfig) (*http.Server, error) {
	address := defaultServerAddress
	if config != nil && config.Address != "" {
		err := validateAddress(config.Address)
		if err != nil {
			return nil, err
		}
		address = config.Address
	}

	return &http.Server{Addr: address}, nil
}

func validateAddress(address string) error {
	splitAddress := strings.Split(address, ":")
	lengthSplit := len(splitAddress)
	if lengthSplit > 2 {
		return errors.New("address should contain single colon. Use the format `[host]:port`, with optional host")
	}

	port := splitAddress[0]
	if lengthSplit == 2 {
		port = splitAddress[1]
	}

	intPort, err := strconv.Atoi(port)
	if err != nil || intPort > maxPortNumber || intPort < 1 {
		return fmt.Errorf("port should be an integer between 1 and %d", maxPortNumber)
	}

	return nil
}

func authenticateWithBasicAuth(ctx context.Context, k8sClient client.Client, requestToken string, basicAuth *v1alpha1.BasicAuth, logger logr.Logger) error {
	username, err := decodeSecret(ctx, k8sClient, &basicAuth.UsernameSecretRef, logger)
	if err != nil {
		return err
	}

	password, err := decodeSecret(ctx, k8sClient, &basicAuth.PasswordSecretRef, logger)
	if err != nil {
		return err
	}

	userPwOnRequest, err := base64.StdEncoding.DecodeString(requestToken)
	if err != nil {
		return err
	}

	storedUserPw := fmt.Sprintf("%s:%s", username, password)

	if storedUserPw != string(userPwOnRequest) {
		return errors.New("invalid token. unauthenticated request")
	}

	return nil
}

func authenticateWithBearer(ctx context.Context, k8sClient client.Client, requestToken string, bearer *v1alpha1.BearerToken, logger logr.Logger) error {
	token, err := decodeSecret(ctx, k8sClient, &bearer.BearerTokenSecretRef, logger)
	if err != nil {
		return err
	}

	if token != requestToken {
		return errors.New("invalid token. unauthenticated request")
	}

	return nil
}

func decodeSecret(ctx context.Context, k8sClient client.Client, config *v1alpha1.SecretKeySelector, logger logr.Logger) (string, error) {
	secret, err := util.GetSecret(ctx, k8sClient, config.Name, config.Namespace, logger)
	if err != nil {
		return "", err
	}

	secretBytes, ok := secret.Data[config.Key]
	if !ok {
		return "", fmt.Errorf("%s not found in secret %s", config.Key, config.Name)

	}

	return string(secretBytes), nil
}
