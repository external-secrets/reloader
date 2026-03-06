package pushsecret

import (
	"context"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/handler/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Provider struct{}

func (p *Provider) NewHandler(ctx context.Context, client client.Client, cache v1alpha1.DestinationToWatch) schema.Handler {
	h := &Handler{
		ctx:              ctx,
		client:           client,
		destinationCache: cache,
	}
	h.applyFn = h._apply
	h.referenceFn = h._references
	h.waitForFn = h._waitFor
	return h
}

func init() {
	schema.RegisterProvider(schema.PUSH_SECRET, &Provider{})
}
