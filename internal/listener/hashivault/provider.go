package hashivault

import (
	"context"
	"errors"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/internal/listener/tcp"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

// NewTCPSocketListener initializes a new TCP socket listener using the provided configuration and event channel.
func (p *Provider) CreateListener(ctx context.Context, config *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if config == nil || config.HashicorpVault == nil {
		return nil, errors.New("HashicorpVault config is nil")
	}
	ctx, cancel := context.WithCancel(ctx)
	h := &HashicorpVault{
		config:    config.HashicorpVault,
		context:   ctx,
		cancel:    cancel,
		client:    client,
		eventChan: eventChan,
		logger:    logger,
	}

	sockConfig := &v1alpha1.TCPSocketConfig{
		Host: config.HashicorpVault.Host,
		Port: config.HashicorpVault.Port,
	}
	sock, err := tcp.NewTCPSocketListener(ctx, sockConfig, client, eventChan, logger)
	if err != nil {
		return nil, err
	}
	sock.SetProcessFn(h.processFn)
	h.tcpSocket = sock
	return h, nil
}

func init() {
	schema.RegisterProvider(schema.HASHICORP_VAULT, &Provider{})
}
