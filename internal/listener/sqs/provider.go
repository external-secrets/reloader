package sqs

import (
	"context"
	"errors"
	"fmt"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/internal/util/mapper"
	listenerAWS "github.com/external-secrets/reloader/pkg/listener/aws"
	modelAWS "github.com/external-secrets/reloader/pkg/models/aws"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

// NewAWSSQSListener creates a new AWSSQSListener.
func (p *Provider) CreateListener(ctx context.Context, config *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if config == nil || config.AwsSqs == nil {
		return nil, errors.New("aws sqs config is nil")
	}
	// Create authenticated SQS Listener
	parsedConfig, err := mapper.TransformConfig[modelAWS.AWSSQSConfig](config.AwsSqs)
	if err != nil {
		logger.Error(err, "Failed to parse config")
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	listener, err := listenerAWS.NewAWSSQSListener(ctx, &parsedConfig, client, logger)
	if err != nil {
		logger.Error(err, "Failed to create SQS Listener")
		return nil, fmt.Errorf("failed to create SQS Listener: %w", err)
	}

	return &AWSSQSListener{
		context:   ctx,
		listener:  listener,
		eventChan: eventChan,
		logger:    logger,
	}, nil
}

func init() {
	schema.RegisterProvider(schema.AWS_SQS, &Provider{})
}
