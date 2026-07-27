package pubsub

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/pubsub" //nolint:staticcheck
	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/pkg/auth/gcp"
	"github.com/go-logr/logr"
	"google.golang.org/api/option"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

// NewGooglePubSubListener creates a new GooglePubSubListener.
func (p *Provider) CreateListener(ctx context.Context, config *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if config == nil || config.GooglePubSub == nil {
		return nil, errors.New("GooglePubSub config is nil")
	}
	ctx, cancel := context.WithCancel(ctx)

	ts, err := gcp.NewTokenSource(ctx, config.GooglePubSub.Auth, config.GooglePubSub.ProjectID, client)
	if err != nil {
		defer cancel()
		return nil, fmt.Errorf("could not create token source: %w", err)
	}

	pubsubClient, err := pubsub.NewClient(ctx, config.GooglePubSub.ProjectID, option.WithTokenSource(ts))
	if err != nil {
		defer cancel()
		return nil, fmt.Errorf("could not create pubsub client: %w", err)
	}
	return &GooglePubSub{
		config:       config.GooglePubSub,
		context:      ctx,
		cancel:       cancel,
		client:       client,
		eventChan:    eventChan,
		logger:       logger,
		pubsubClient: pubsubClient,
	}, nil
}

func init() {
	schema.RegisterProvider(schema.GOOGLE_PUB_SUB, &Provider{})
}
