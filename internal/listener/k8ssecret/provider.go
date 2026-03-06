package k8ssecret

import (
	"context"
	"errors"
	"sync"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/kubernetes"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

// CreateListener creates a Kubernetes Secret Listener.
func (p *Provider) CreateListener(ctx context.Context, config *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (schema.Listener, error) {
	if config == nil || config.KubernetesSecret == nil {
		return nil, errors.New("KubernetesSecret config is nil")
	}
	ctx, cancel := context.WithCancel(ctx)
	h := &kubernetes.Handler[*corev1.Secret]{
		Config: &v1alpha1.KubernetesObjectConfig{
			ServerURL:     config.KubernetesSecret.ServerURL,
			Auth:          config.KubernetesSecret.Auth,
			LabelSelector: config.KubernetesSecret.LabelSelector,
		},
		Ctx:        ctx,
		Cancel:     cancel,
		Client:     client,
		EventChan:  eventChan,
		Logger:     logger,
		VersionMap: sync.Map{},
		Obj:        &corev1.Secret{},
		Name:       "secret",
	}

	return h, nil
}

func init() {
	schema.RegisterProvider(schema.KUBERNETES_SECRET, &Provider{})
}
