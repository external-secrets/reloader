package mock

import (
	"context"
	"errors"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

// CreateListener creates a mock listener for simulated secret rotation events.
func (p *Provider) CreateListener(ctx context.Context, source *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if source == nil || source.Mock == nil {
		return nil, errors.New("mock listener requires a valid mock configuration")
	}
	mockEvents := []events.SecretRotationEvent{
		{
			SecretIdentifier:  "aws://secret/arn:aws:secretsmanager:us-east-1:123456789012:secret:mysecret",
			RotationTimestamp: "2024-09-19T12:00:00Z",
			TriggerSource:     "aws-secretsmanager",
		},
	}
	return NewMockListener(mockEvents, time.Duration(source.Mock.EmitInterval)*time.Millisecond, eventChan), nil
}

func init() {
	schema.RegisterProvider(schema.MOCK, &Provider{})
}
