package eventgrid

import (
	"context"
	"errors"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

func (p *Provider) CreateListener(ctx context.Context, config *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if config == nil || config.AzureEventGrid == nil {
		return nil, errors.New("AzureEventGrid config is nil")
	}

	ctx, cancel := context.WithCancel(ctx)

	logger.Info("Creating new AzureEventGridListener")

	return &AzureEventGridListener{
		context:   ctx,
		cancel:    cancel,
		client:    client,
		config:    config.AzureEventGrid,
		eventChan: eventChan,
		logger:    logger,
	}, nil
}

func init() {
	schema.RegisterProvider(schema.AZURE_EVENT_GRID, &Provider{})
}
