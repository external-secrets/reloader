package webhook

import (
	"context"
	"errors"
	"fmt"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct {
}

// NewWebhookListener creates a new Listener that listens for webhook notifications based on the provided configuration and event channel.
func (p *Provider) CreateListener(ctx context.Context, config *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if config == nil || config.Webhook == nil {
		return nil, errors.New("webhook config is nil")
	}
	server, err := createServer(config.Webhook)
	if err != nil {
		logger.Error(err, "failed to create webhook server")
		return nil, fmt.Errorf("failed to create webhook server: %w", err)
	}

	childCtx, cancel := context.WithCancel(ctx)

	listener := &WebhookListener{
		config:     config.Webhook,
		eventChan:  eventChan,
		ctx:        childCtx,
		cancel:     cancel,
		logger:     logger,
		server:     server,
		client:     client,
		retryQueue: make(chan *RetryMessage),
	}

	listener.createHandler()

	return listener, nil
}

func init() {
	schema.RegisterProvider(schema.WEBHOOK, &Provider{})
}
