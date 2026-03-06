package handler

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	esov1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/handler/schema"
)

type EventHandler struct {
	ctx    context.Context
	client client.Client
	cache  []esov1alpha1.DestinationToWatch
}

func NewEventHandler(client client.Client) *EventHandler {
	ctx := context.Background()
	return &EventHandler{
		ctx:    ctx,
		client: client,
	}
}

func (h *EventHandler) UpdateDestinationsToWatch(watch []esov1alpha1.DestinationToWatch) {
	h.cache = watch
}

func (h *EventHandler) HandleEvent(ctx context.Context, event events.SecretRotationEvent) error {
	logger := log.FromContext(ctx)
	for _, watchCriteria := range h.cache {

		prov := schema.GetProvider(watchCriteria.Type)
		if prov == nil {
			logger.Info("Provider not found", "destination type", watchCriteria.Type)
			continue
		}
		h := prov.NewHandler(ctx, h.client, watchCriteria)
		// Mutate Handler for different Update and Match Strategies
		if watchCriteria.UpdateStrategy != nil {
			logger.Info("Optional Update strategies are not implemented", "UpdateStrategy", watchCriteria.UpdateStrategy)
		}
		if watchCriteria.MatchStrategy != nil {
			logger.Info("Optional Match strategies are not implemented", "MatchStrategy", watchCriteria.MatchStrategy)
		}
		objs, err := h.Filter(&watchCriteria, event)
		if err != nil {
			return fmt.Errorf("failed to filter objects:%w", err)
		}
		// Use Handler methods to figure out and apply objects
		for _, obj := range objs {
			isReferenced, err := h.References(obj, event.SecretIdentifier)
			if err != nil {
				// This error means something went wrong on a reference check - which is typically very bad
				logger.Error(err, "failed to check if object is referenced", "name", obj.GetName(), "namespace", obj.GetNamespace(), "type", watchCriteria.Type)
				return fmt.Errorf("failed to check if object is referenced:%w", err)
			}
			if !isReferenced {
				logger.V(1).Info("skipping object as its not referenced", "name", obj.GetName(), "namespace", obj.GetNamespace())
				continue
			}
			// TODO[gusfcarvalho] - After we know all objects we have to apply due to an event,
			// We should create a queuing mechanism to try to apply them even if errors happen
			// As the error might just have been a failing deployment that is going to be fixed eventually
			// object is referenced - apply
			err = h.Apply(obj, event)
			if err != nil {
				logger.Error(err, "failed to update object", "name", obj.GetName(), "namespace", obj.GetNamespace(), "type", watchCriteria.Type)
				// We need to apply all manifests that we can
				continue
			}
			err = h.WaitFor(obj)
			if err != nil {
				// If WaitFor fails, it means we need to stop the operation (and eventually requeue - see TODO)
				logger.Error(err, "stopped updating because of object update failure", "name", obj.GetName(), "namespace", obj.GetNamespace(), "type", watchCriteria.Type)
				return fmt.Errorf("failed to wait for object:%w", err)
			}
		}
	}
	return nil
}
