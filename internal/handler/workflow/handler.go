package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/handler/schema"
	"github.com/external-secrets/reloader/internal/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	kruntime "k8s.io/apimachinery/pkg/runtime/schema"
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
	if destination.WorkflowRunTemplate == nil {
		return nil, errors.New("destination isn't type Workflow")
	}
	logger := log.FromContext(h.ctx)
	workflows := &unstructured.UnstructuredList{}
	workflows.SetGroupVersionKind(kruntime.GroupVersionKind{
		Group:   "workflows.external-secrets.io",
		Version: "v1alpha1",
		Kind:    "WorkflowRunTemplateList",
	})
	if err := h.client.List(h.ctx, workflows); err != nil {
		return nil, fmt.Errorf("failed to list Workflows: %w", err)
	}
	for i := range workflows.Items {
		wf := &workflows.Items[i]
		isWatched, err := h.isResourceWatched(wf, h.destinationCache)
		if err != nil {
			logger.Error(err, "failed to check if Workflow is watched", "name", wf.GetName(), "namespace", wf.GetNamespace())
			continue
		}
		if isWatched {
			objs = append(objs, wf)
		}
	}
	return objs, nil
}

func (h *Handler) Apply(obj client.Object, event events.SecretRotationEvent) error {
	return h.applyFn(obj, event)
}

func (h *Handler) _apply(es client.Object, event events.SecretRotationEvent) error {
	logger := log.FromContext(h.ctx)

	annotations := es.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations["reloader/last-rotated"] = event.RotationTimestamp
	annotations["reloader/trigger-source"] = event.TriggerSource

	es.SetAnnotations(annotations)

	if err := h.client.Update(h.ctx, es); err != nil {
		return fmt.Errorf("failed to update WorkflowRunTemplate:%w", err)
	}
	logger.V(1).Info("Annotated WorkflowRunTemplate", "name", es.GetName(), "namespace", es.GetNamespace())
	return nil
}

// isResourceWatched determines if a single ExternalSecret matches any of the SecretsToWatch criteria.
func (h *Handler) isResourceWatched(obj client.Object, w v1alpha1.DestinationToWatch) (bool, error) {
	watchCriteria := w.WorkflowRunTemplate
	if watchCriteria == nil {
		return false, errors.New("watch type is not Workflow")
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
	namespaceMatch, err := util.MatchesAnyNamespaceSelector(h.ctx, obj, namespaceSelectors, h.client)
	if err != nil {
		return false, err
	}
	labelMatch, err := util.MatchesLabelSelectors(h.ctx, obj, labelSelector, h.client)
	if err != nil {
		return false, err
	}
	nameMatch := util.IsNameInList(obj, nameSet)
	if namespaceMatch && labelMatch && nameMatch {
		return true, nil
	}

	return false, nil
}

func (h *Handler) WaitFor(obj client.Object) error {
	return h.waitForFn(obj)
}

// _waitFor is a noop for ExternalSecrets
func (h *Handler) _waitFor(obj client.Object) error {
	// ExternalSecrets handler does not need to wait for anything
	return nil
}
func (h *Handler) References(obj client.Object, secretIdentifier string) (bool, error) {
	return h.referenceFn(obj, secretIdentifier)
}

// _references returns true always - as Workflows always need to be triggered on a new action.
func (h *Handler) _references(obj client.Object, secretIdentifier string) (bool, error) {
	return true, nil
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
