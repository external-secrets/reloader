package tcp

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
	if config == nil || config.TCPSocket == nil {
		return nil, errors.New("tcp socket config is nil")
	}
	ctx, cancel := context.WithCancel(ctx)
	h := &TCPSocket{
		config:    config.TCPSocket,
		context:   ctx,
		cancel:    cancel,
		client:    client,
		eventChan: eventChan,
		logger:    logger,
	}
	h.SetProcessFn(h.defaultProcess)
	return h, nil
}

// NewTCPSocketListener initializes a new TCP socket in a way other components can consume
func NewTCPSocketListener(ctx context.Context, config *v1alpha1.TCPSocketConfig, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (*TCPSocket, error) {
	ctx, cancel := context.WithCancel(ctx)
	sock := &TCPSocket{
		config:    config,
		context:   ctx,
		cancel:    cancel,
		client:    client,
		eventChan: eventChan,
		logger:    logger,
	}
	sock.SetProcessFn(sock.defaultProcess)
	return sock, nil
}

func init() {
	schema.RegisterProvider(schema.TCP_SOCKET, &Provider{})
}
