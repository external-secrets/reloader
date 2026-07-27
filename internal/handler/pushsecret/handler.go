package pushsecret

import (
	"context"
	"errors"
	"fmt"

	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/handler/schema"
	"github.com/external-secrets/reloader/internal/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type Handler struct {
	ctx              context.Context
	client           client.Client
	destinationCache v1alpha1.DestinationToWatch
	applyFn          schema.ApplyFn
	referenceFn      schema.ReferenceFn
	waitForFn        schema.WaitForFn
}

func (h *Handler) Filter(destination *v1alpha1.DestinationToWatch, event events.SecretRotationEvent) ([]client.Object, error) {
	objs := []client.Object{}
	if destination.PushSecret == nil {
		return nil, errors.New("destination isn't type PushSecret")
	}
	logger := log.FromContext(h.ctx)
	var pushSecrets esv1alpha1.PushSecretList
	if err := h.client.List(h.ctx, &pushSecrets); err != nil {
		return nil, fmt.Errorf("failed to list PushSecrets:%w", err)
	}
	for key, ps := range pushSecrets.Items {
		isWatched, err := h.isResourceWatched(ps, h.destinationCache)
		if err != nil {
			logger.Error(err, "failed to check if PushSecret is watched", "name", ps.Name, "namespace", ps.Namespace)
			continue
		}
		if isWatched {
			objs = append(objs, &pushSecrets.Items[key])
		}
	}
	return objs, nil
}

func (h *Handler) Apply(obj client.Object, event events.SecretRotationEvent) error {
	return h.applyFn(obj, event)
}

func (h *Handler) _apply(ps client.Object, event events.SecretRotationEvent) error {
	logger := log.FromContext(h.ctx)

	annotations := ps.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations["reloader/last-rotated"] = event.RotationTimestamp
	annotations["reloader/trigger-source"] = event.TriggerSource

	ps.SetAnnotations(annotations)

	if err := h.client.Update(h.ctx, ps); err != nil {
		return fmt.Errorf("failed to update PushSecret:%w", err)
	}
	logger.V(1).Info("Annotated PushSecret", "name", ps.GetName(), "namespace", ps.GetNamespace())
	return nil
}

// isResourceWatched determines if a single PushSecret matches any of the SecretsToWatch criteria.
func (h *Handler) isResourceWatched(secret esv1alpha1.PushSecret, w v1alpha1.DestinationToWatch) (bool, error) {
	watchCriteria := w.PushSecret
	if watchCriteria == nil {
		return false, errors.New("watch type is not pushSecret")
	}
	// Preprocess NamespaceSelectors
	namespaceSelectors := make([]labels.Selector, 0, len(watchCriteria.NamespaceSelectors))
	for _, nsSelector := range watchCriteria.NamespaceSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&nsSelector)
		if err != nil {
			return false, fmt.Errorf("invalid namespace selector: %v", err)
		}
		namespaceSelectors = append(namespaceSelectors, selector)
	}

	// Preprocess LabelSelectors
	var labelSelector labels.Selector
	var err error
	if watchCriteria.LabelSelectors != nil {
		labelSelector, err = metav1.LabelSelectorAsSelector(watchCriteria.LabelSelectors)
		if err != nil {
			return false, fmt.Errorf("invalid label selector: %v", err)
		}
	}

	// Preprocess Names into a map
	nameSet := make(map[string]struct{})
	for _, name := range watchCriteria.Names {
		nameSet[name] = struct{}{}
	}

	// Perform matching
	namespaceMatch, err := util.MatchesAnyNamespaceSelector(h.ctx, &secret, namespaceSelectors, h.client)
	if err != nil {
		return false, err
	}
	labelMatch, err := util.MatchesLabelSelectors(h.ctx, &secret, labelSelector, h.client)
	if err != nil {
		return false, err
	}
	nameMatch := util.IsNameInList(&secret, nameSet)
	if namespaceMatch && labelMatch && nameMatch {
		return true, nil
	}

	return false, nil
}

func (h *Handler) WaitFor(obj client.Object) error {
	return h.waitForFn(obj)
}

// _waitFor is a noop for PushSecrets
func (h *Handler) _waitFor(obj client.Object) error {
	// PushSecrets handler does not need to wait for anything
	return nil
}
func (h *Handler) References(obj client.Object, secretIdentifier string) (bool, error) {
	return h.referenceFn(obj, secretIdentifier)
}

// _references checks if the PushSecret references the given secret identifier.
// It is the default References implementation
func (h *Handler) _references(obj client.Object, secretIdentifier string) (bool, error) {
	ps, ok := obj.(*esv1alpha1.PushSecret)
	if !ok {
		return false, errors.New("obj isn't type PushSecret")
	}
	// Check selector
	if ps.Spec.Selector.Secret != nil && ps.Spec.Selector.Secret.Name == secretIdentifier {
		return true, nil
	}
	for _, data := range ps.Spec.Data {
		if data.Match.RemoteRef.RemoteKey == secretIdentifier {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) WithApply(apply schema.ApplyFn) schema.Handler {
	h.applyFn = apply
	return h
}

func (h *Handler) WithReference(ref schema.ReferenceFn) schema.Handler {
	h.referenceFn = ref
	return h
}

func (h *Handler) WithWaitFor(waitFor schema.WaitForFn) schema.Handler {
	h.waitForFn = waitFor
	return h
}
