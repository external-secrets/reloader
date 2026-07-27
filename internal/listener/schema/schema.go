package schema

import (
	"context"
	"sync"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AWS_SQS               = "AwsSqs"
	AZURE_EVENT_GRID      = "AzureEventGrid"
	GOOGLE_PUB_SUB        = "GooglePubSub"
	WEBHOOK               = "Webhook"
	TCP_SOCKET            = "TCPSocket"
	HASHICORP_VAULT       = "HashicorpVault"
	MOCK                  = "Mock"
	KUBERNETES_SECRET     = "KubernetesSecret"
	KUBERNETES_CONFIG_MAP = "KubernetesConfigMap"
)

var (
	providers sync.Map // map[string]Listener
)

func init() {
	providers = sync.Map{}
}

// Listener defines the interface for starting and stopping a listener.
type Listener interface {
	Start() error
	Stop() error
}

// Provider is an interface for creating event listeners for secret rotation events.
type Provider interface {
	CreateListener(ctx context.Context, source *v1alpha1.NotificationSource, client client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) (Listener, error)
}

func ForceRegister(name string, prov Provider) {
	providers.Store(name, prov)
}

func RegisterProvider(name string, prov Provider) bool {
	if _, loaded := providers.LoadOrStore(name, prov); loaded {
		return false
	}
	return true
}

func GetProvider(name string) Provider {
	if prov, loaded := providers.Load(name); loaded {
		if p, ok := prov.(Provider); ok {
			return p
		}
	}
	return nil
}
