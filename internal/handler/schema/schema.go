package schema

import (
	"context"
	"sync"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	EXTERNAL_SECRET = "ExternalSecret"
	PUSH_SECRET     = "PushSecret"
	DEPLOYMENT      = "Deployment"
	WORKFLOW        = "WorkflowRunTemplate"
)

type ApplyFn func(obj client.Object, event events.SecretRotationEvent) error
type ReferenceFn func(obj client.Object, secretName string) (bool, error)

type WaitForFn func(obj client.Object) error

type Handler interface {
	// Method to implement References
	// In the future, `matchStrategy` will just replace the References Method
	References(obj client.Object, secretName string) (bool, error)

	// Method to implement Apply
	// In the future, `updateStrategy` will create a new Apply method
	Apply(obj client.Object, event events.SecretRotationEvent) error

	// Method to implement WaitFor
	// In the future, `waitStrategy` will create a new WaitFor method
	WaitFor(obj client.Object) error

	// Filter implements the filter logic given the selected destination
	// Returns all objects that match the specific destination configuraiton
	Filter(destination *v1alpha1.DestinationToWatch, event events.SecretRotationEvent) ([]client.Object, error)
	WithApply(fn ApplyFn) Handler
	WithReference(fn ReferenceFn) Handler
	WithWaitFor(fn WaitForFn) Handler
}

type Provider interface {
	NewHandler(ctx context.Context, client client.Client, destination v1alpha1.DestinationToWatch) Handler
}

var (
	providerMap sync.Map
)

func init() {
	providerMap = sync.Map{}
}

func ForceRegister(name string, prov Provider) {
	providerMap.Store(name, prov)
}

func RegisterProvider(name string, prov Provider) bool {
	actual, loaded := providerMap.LoadOrStore(name, prov)
	if actual != nil {
		return false
	}
	return loaded
}
func GetProvider(name string) Provider {
	content, exists := providerMap.Load(name)
	if !exists {
		return nil
	}
	h, ok := content.(Provider)
	if !ok {
		return nil
	}
	return h
}
