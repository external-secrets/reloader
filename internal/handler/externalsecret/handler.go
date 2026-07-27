package externalsecret

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
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
	if destination.ExternalSecret == nil {
		return nil, errors.New("destination isn't type ExternalSecret")
	}
	logger := log.FromContext(h.ctx)
	var externalSecrets esov1.ExternalSecretList
	if err := h.client.List(h.ctx, &externalSecrets); err != nil {
		return nil, fmt.Errorf("failed to list ExternalSecrets:%w", err)
	}
	for key, es := range externalSecrets.Items {
		isWatched, err := h.isResourceWatched(es, h.destinationCache)
		if err != nil {
			logger.Error(err, "failed to check if ExternalSecret is watched", "name", es.Name, "namespace", es.Namespace)
			continue
		}
		if isWatched {
			objs = append(objs, &externalSecrets.Items[key])
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
		return fmt.Errorf("failed to update ExternalSecret:%w", err)
	}
	logger.V(1).Info("Annotated ExternalSecret", "name", es.GetName(), "namespace", es.GetNamespace())
	return nil
}

// isResourceWatched determines if a single ExternalSecret matches any of the SecretsToWatch criteria.
func (h *Handler) isResourceWatched(secret esov1.ExternalSecret, w v1alpha1.DestinationToWatch) (bool, error) {
	watchCriteria := w.ExternalSecret
	if watchCriteria == nil {
		return false, errors.New("watch type is not externalSecret")
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

// _waitFor is a noop for ExternalSecrets
func (h *Handler) _waitFor(obj client.Object) error {
	// ExternalSecrets handler does not need to wait for anything
	return nil
}
func (h *Handler) References(obj client.Object, secretIdentifier string) (bool, error) {
	return h.referenceFn(obj, secretIdentifier)
}

// _references checks if the ExternalSecret references the given secret identifier.
// It is the default References implementation
func (h *Handler) _references(obj client.Object, secretIdentifier string) (bool, error) {
	es, ok := obj.(*esov1.ExternalSecret)
	if !ok {
		return false, errors.New("obj isn't type ExternalSecret")
	}
	// Check Data field
	for _, data := range es.Spec.Data {
		if data.RemoteRef.Key == secretIdentifier {
			return true, nil
		}
	}

	// Check DataFrom field
	for _, dataFrom := range es.Spec.DataFrom {
		if dataFrom.Extract != nil && dataFrom.Extract.Key == secretIdentifier {
			return true, nil
		}
		// Handle RegExp matching if needed
		if dataFrom.Find != nil {
			if dataFrom.Find.Name != nil {
				re := regexp.MustCompile(dataFrom.Find.Name.RegExp)
				if re.MatchString(secretIdentifier) {
					return true, nil
				}
			}
		}
	}

	// Check target.template.templateFrom for ConfigMap/Secret references (e.g. when the
	// trigger is a ConfigMap or in-cluster Secret and the ExternalSecret uses it for templating).
	if es.Spec.Target.Template != nil {
		for _, tf := range es.Spec.Target.Template.TemplateFrom {
			if tf.ConfigMap != nil && tf.ConfigMap.Name == secretIdentifier {
				return true, nil
			}
			if tf.Secret != nil && tf.Secret.Name == secretIdentifier {
				return true, nil
			}
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
