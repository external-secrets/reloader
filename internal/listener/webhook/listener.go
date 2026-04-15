package webhook

import (
	"context"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// WebhookListener implements schema.Listener for one Config's webhook route on the shared WebhookServer.
type WebhookListener struct {
	server     *WebhookServer
	configName string
	routeCtx   context.Context
	cfg        *v1alpha1.WebhookConfig
	client     client.Client
	eventChan  chan events.SecretRotationEvent
	logger     logr.Logger
}

// NewWebhookListener returns a schema.Listener that registers/unregisters one route on the shared server.
func NewWebhookListener(
	server *WebhookServer,
	configName string,
	routeCtx context.Context,
	cfg *v1alpha1.WebhookConfig,
	k8sClient client.Client,
	eventChan chan events.SecretRotationEvent,
	logger logr.Logger,
) schema.Listener {
	return &WebhookListener{
		server:     server,
		configName: configName,
		routeCtx:   routeCtx,
		cfg:        cfg,
		client:     k8sClient,
		eventChan:  eventChan,
		logger:     logger,
	}
}

// Start registers the route on the shared server.
func (l *WebhookListener) Start() error {
	l.server.Register(l.configName, l.routeCtx, l.cfg, l.client, l.eventChan, l.logger)
	return nil
}

// Stop unregisters the route.
func (l *WebhookListener) Stop() error {
	l.server.Unregister(l.configName)
	return nil
}
