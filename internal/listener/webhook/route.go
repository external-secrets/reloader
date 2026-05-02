package webhook

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/internal/util"
	"github.com/go-logr/logr"
	"github.com/tidwall/gjson"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultIdentifierPath = "0.data.ObjectName"
const defaultMaxRetries = 10

// route holds the per-Config handler state for a single webhook endpoint.
type route struct {
	configName string
	config     *v1alpha1.WebhookConfig
	eventChan  chan events.SecretRotationEvent
	client     client.Client
	logger     logr.Logger
	retryQueue chan *retryMessage
	ctx        context.Context
	cancel     context.CancelFunc
	retryWG    sync.WaitGroup
}

type retryMessage struct {
	event      events.SecretRotationEvent
	currentRun int
	retryAt    time.Time
}

func newRoute(parentCtx context.Context, configName string, cfg *v1alpha1.WebhookConfig, k8sClient client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) *route {
	ctx, cancel := context.WithCancel(parentCtx)
	r := &route{
		configName: configName,
		config:     cfg,
		eventChan:  eventChan,
		client:     k8sClient,
		logger:     logger.WithName("webhook-route").WithValues("config", configName),
		retryQueue: make(chan *retryMessage),
		ctx:        ctx,
		cancel:     cancel,
	}
	if cfg != nil && cfg.RetryPolicy != nil {
		r.retryWG.Add(1)
		go func() {
			defer r.retryWG.Done()
			r.handleRetries()
		}()
	}
	return r
}

func (r *route) shutdown() {
	r.cancel()
	r.retryWG.Wait()
	close(r.retryQueue)
	r.drainRetryQueue()
}

func (r *route) handle(w http.ResponseWriter, req *http.Request) {
	if err := r.authenticate(req.Header); err != nil {
		r.logger.Error(err, "Couldn't authenticate request")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprintln(w, "Couldn't authenticate request")
		return
	}

	payload, err := parsePayloadToString(req.Body)
	if err != nil {
		r.logger.Error(err, "Couldn't parse event payload")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, "Couldn't decode webhook payload. Send a valid json")
		return
	}
	_ = req.Body.Close()

	identifierPath := r.getIdentifierPath()

	secretIdentifier, err := getSecretIdentifierFromPayload(payload, identifierPath)
	if err != nil {
		message := fmt.Sprintf("Secret Identifier not found on payload. "+
			"Ensure that your secret is on the following path: %s", identifierPath)
		r.logger.Error(err, message)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintln(w, message)
		return
	}

	if event, err := r.processSecret(secretIdentifier); err != nil {
		r.logger.Error(err, "Failed to process event")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintln(w, "Failed to process event")

		if r.config != nil && r.config.RetryPolicy != nil {
			select {
			case r.retryQueue <- &retryMessage{event: event, currentRun: 1, retryAt: time.Now()}:
			case <-r.ctx.Done():
			}
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (r *route) authenticate(header http.Header) error {
	if r.config == nil || r.config.Auth == nil {
		return nil
	}

	basicAuth := r.config.Auth.BasicAuth
	bearer := r.config.Auth.BearerToken

	if basicAuth == nil && bearer == nil {
		return nil
	}

	authHeader := strings.SplitN(header.Get("Authorization"), " ", 2)
	if len(authHeader) != 2 {
		return errors.New("malformed authorization header. Use `Bearer <token>` or `Basic <token>`")
	}

	if basicAuth != nil && strings.EqualFold(authHeader[0], "basic") {
		return authenticateWithBasicAuth(r.ctx, r.client, authHeader[1], basicAuth, r.logger)
	}

	return authenticateWithBearer(r.ctx, r.client, authHeader[1], bearer, r.logger)
}

func (r *route) getIdentifierPath() string {
	if r.config != nil && r.config.SecretIdentifierOnPayload != "" {
		return r.config.SecretIdentifierOnPayload
	}
	return defaultIdentifierPath
}

func (r *route) processSecret(secretIdentifier string) (events.SecretRotationEvent, error) {
	event := events.SecretRotationEvent{
		SecretIdentifier:  secretIdentifier,
		RotationTimestamp: time.Now().Format("2006-01-02-15-04-05.000"),
		TriggerSource:     schema.WEBHOOK,
	}
	return event, r.processEvent(event)
}

func (r *route) processEvent(event events.SecretRotationEvent) error {
	select {
	case r.eventChan <- event:
		r.logger.Info("Published event to eventChan", "Event", event)
		return nil
	case <-r.ctx.Done():
		return r.ctx.Err()
	}
}

func (r *route) handleRetries() {
	maxRetries := min(r.config.RetryPolicy.MaxRetries, defaultMaxRetries)
	for {
		select {
		case <-r.ctx.Done():
			return
		case message, ok := <-r.retryQueue:
			if !ok {
				return
			}
			if r.ctx.Err() != nil {
				return
			}

			beforeOrNow := message.retryAt.Compare(time.Now()) <= 0
			if beforeOrNow {
				err := r.processEvent(message.event)
				if err == nil {
					r.logger.Info(fmt.Sprintf(
						"Message for '%s' successfully processed after %d retries",
						message.event.SecretIdentifier,
						message.currentRun,
					))
					continue
				}

				if message.currentRun >= maxRetries {
					r.logger.Error(err, fmt.Sprintf(
						"Message for '%s' was not processed after %d retries",
						message.event.SecretIdentifier,
						message.currentRun,
					))
					continue
				}

				message.currentRun++
				message.retryAt = getNextRetryAt(r.config.RetryPolicy.Algorithm, message.currentRun)
			}
			select {
			case r.retryQueue <- message:
			case <-r.ctx.Done():
				return
			}
		}
	}
}

func (r *route) drainRetryQueue() {
	r.logger.Info("Draining retry queue")
	for message := range r.retryQueue {
		err := r.processEvent(message.event)
		if err == nil {
			r.logger.Info(fmt.Sprintf(
				"Message for '%s' successfully processed after %d retries",
				message.event.SecretIdentifier,
				message.currentRun,
			))
		} else {
			r.logger.Info(fmt.Sprintf("Message for '%s' was not processed", message.event.SecretIdentifier))
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
		return "", errors.New("secret not found on event")
	}
	return secretIdentifier.String(), nil
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
