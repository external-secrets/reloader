package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/pubsub" //nolint:staticcheck
	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	gcpModel "github.com/external-secrets/reloader/pkg/models/gcp"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GooglePubSub handles Google Pub/Sub notifications.
type GooglePubSub struct {
	config       *v1alpha1.GooglePubSubConfig
	context      context.Context
	cancel       context.CancelFunc
	client       client.Client
	eventChan    chan events.SecretRotationEvent
	pubsubClient *pubsub.Client
	logger       logr.Logger
}

// Stop stops polling the Google Pub/Sub.
func (h *GooglePubSub) Stop() error {
	h.cancel()
	return nil
}

// Start begins polling the Google Pub/Sub for messages.
func (h *GooglePubSub) Start() error {
	h.logger.Info(fmt.Sprintf("Started subscribing to %s subscription %s\n", h.config.ProjectID, h.config.SubscriptionID))
	sb := h.pubsubClient.SubscriptionInProject(h.config.SubscriptionID, h.config.ProjectID)
	go func() {
		err := sb.Receive(h.context, processMessage(h.eventChan, h.logger, h.config.SubscriptionID))
		if err != nil {
			h.cancel()
		}
	}()
	return nil
}

func processMessage(eventChannel chan events.SecretRotationEvent, logger logr.Logger, subscription string) func(context.Context, *pubsub.Message) {
	return func(ctx context.Context, m *pubsub.Message) {
		if ctx.Err() != nil {
			m.Nack()
			logger.Info("closing channel due to context error", "error", ctx.Err())
			return
		}
		audit := gcpModel.AuditLogMessage{}
		err := json.Unmarshal(m.Data, &audit)
		if err != nil {
			m.Nack()
			logger.Error(err, "could not unmarshal message")
			return
		}
		msgTime := time.Now().Format(time.RFC3339)
		logger.Info("new message received", "subscription", subscription,
			"msgTime", msgTime,
			"methodName", audit.ProtoPayload.MethodName)
		switch audit.ProtoPayload.MethodName {
		case "google.cloud.secretmanager.v1.SecretManagerService.AddSecretVersion":
			name := strings.Split(audit.ProtoPayload.ResourceName, "/")[3]
			event := events.SecretRotationEvent{}
			event.SecretIdentifier = name
			event.RotationTimestamp = msgTime
			event.TriggerSource = schema.GOOGLE_PUB_SUB
			eventChannel <- event
			logger.Info("Published event to eventChan", "Event", event)
		default:
		}
		m.Ack()
	}
}
