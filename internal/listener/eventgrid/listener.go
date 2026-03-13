package eventgrid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EventGridEvent struct {
	ID              string          `json:"id"`
	Topic           string          `json:"topic"`
	Subject         string          `json:"subject"`
	EventType       string          `json:"eventType"`
	EventTime       time.Time       `json:"eventTime"`
	MetadataVersion string          `json:"metadataVersion"`
	DataVersion     string          `json:"dataVersion"`
	Data            json.RawMessage `json:"data"`
}

type SecretNewVersionCreatedData struct {
	ObjectType string `json:"objectType"`
	ObjectName string `json:"objectName"`
}

type SubscriptionValidationData struct {
	ValidationCode string `json:"validationCode"`
	ValidationURL  string `json:"validationUrl"`
}

type AzureEventGridListener struct {
	cancel    context.CancelFunc
	client    client.Client
	context   context.Context
	config    *v1alpha1.AzureEventGridConfig
	eventChan chan events.SecretRotationEvent
	logger    logr.Logger
	server    *http.Server
}

func (a *AzureEventGridListener) Start() error {
	a.logger.Info("Starting Event Grid listener")

	mux := http.NewServeMux()
	for _, subscription := range a.config.Subscriptions {
		path := fmt.Sprintf("/%s", subscription)
		a.logger.Info("Registering handler for path", "path", path)
		mux.HandleFunc(path, eventHandler(a))
	}

	addr := fmt.Sprintf("%s:%d", a.config.Host, a.config.Port)
	a.server = &http.Server{Addr: addr, Handler: mux}
	a.logger.Info("Starting server", "addr", addr)
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error(err, "Failed to start server")
		}
	}()
	return nil
}

func (a *AzureEventGridListener) Stop() error {
	a.logger.Info("Stopping Azure Event Grid Listener...")
	a.cancel()
	return a.server.Close()
}

func eventHandler(a *AzureEventGridListener) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusBadRequest)
			return
		}

		var events []EventGridEvent
		if err := json.Unmarshal(body, &events); err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		// Process each event
		for _, event := range events {
			switch event.EventType {
			case "Microsoft.EventGrid.SubscriptionValidationEvent":
				handleEventGridHandshake(w, r, a.config, event, a.logger)
			case "Microsoft.KeyVault.SecretNewVersionCreated":
				a.handleSecretEvent(w, event)
			default:
				log.Printf("Unhandled event type: %s", event.EventType)
			}
		}
	}
}

func handleEventGridHandshake(w http.ResponseWriter, r *http.Request, config *v1alpha1.AzureEventGridConfig, event EventGridEvent, logger logr.Logger) {
	// Read the aeg-subscription-name header
	subscriptionName := strings.ToLower(r.Header.Get("aeg-subscription-name"))
	if subscriptionName == "" {
		logger.Error(nil, "Missing aeg-subscription-name header")
		http.Error(w, "Missing aeg-subscription-name header", http.StatusBadRequest)
		return
	}

	// Check if the subscription exists in the configuration
	exists := false
	for _, sub := range config.Subscriptions {
		if sub == subscriptionName {
			exists = true
			break
		}
	}

	if !exists {
		http.Error(w, "Subscription not found", http.StatusNotFound)
		logger.Error(nil, "Subscription not found")
		return
	}

	// Parse validation data
	var validationEventData struct {
		Data SubscriptionValidationData `json:"data"`
	}
	if err := json.Unmarshal(event.Data, &validationEventData.Data); err != nil {
		http.Error(w, "Failed to parse validation data", http.StatusBadRequest)
		logger.Error(err, "Failed to parse validation data")
		return
	}

	// Respond with the validation code
	responseData := map[string]string{
		"validationResponse": validationEventData.Data.ValidationCode,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err := json.NewEncoder(w).Encode(responseData)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		logger.Error(err, "Failed to encode response")
		return
	}

	// Call the validation URL to complete the handshake
	go func() {
		resp, err := http.Get(validationEventData.Data.ValidationURL)
		if err != nil {
			logger.Error(err, "Failed to call validation URL")
			return
		}
		defer resp.Body.Close() //nolint

		if resp.StatusCode != http.StatusOK {
			logger.Error(nil, fmt.Sprintf("Received non-200 response from validation callback url: %d %s", resp.StatusCode, resp.Status))
			return
		}

		logger.Info("Validation URL call successful", "status", resp.Status)
	}()
}

func (a *AzureEventGridListener) handleSecretEvent(w http.ResponseWriter, event EventGridEvent) {
	var data SecretNewVersionCreatedData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		a.logger.Error(err, "Error unmarshalling secret new version created event")
		http.Error(w, "Invalid JSON in request body", http.StatusBadRequest)
		return
	}

	a.logger.Info("Received secret new version created event", "secret", data.ObjectName)

	rotationEvent := events.SecretRotationEvent{
		SecretIdentifier:  data.ObjectName,
		RotationTimestamp: event.EventTime.String(),
		TriggerSource:     schema.AZURE_EVENT_GRID,
	}

	select {
	case a.eventChan <- rotationEvent:
		a.logger.Info("Published event to eventChan", "Event", event)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return
	case <-a.context.Done():
		a.logger.Info("Context cancelled, exiting")
		return
	}
}
