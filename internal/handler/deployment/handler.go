package deployment

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/handler/schema"
	"github.com/external-secrets/reloader/internal/util"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
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
	if destination.Deployment == nil {
		return nil, errors.New("destination isn't type Deployment")
	}
	logger := log.FromContext(h.ctx)
	var deployments appsv1.DeploymentList
	var opt client.ListOption
	if event.Namespace != "" {
		opt = client.InNamespace(event.Namespace)
	}
	if err := h.client.List(h.ctx, &deployments, opt); err != nil {
		return nil, fmt.Errorf("failed to list Deployments:%w", err)
	}
	for key, deployment := range deployments.Items {
		isWatched, err := h.isResourceWatched(deployment, h.destinationCache)
		if err != nil {
			logger.Error(err, "failed to check if Deployment is watched", "name", deployment.Name, "namespace", deployment.Namespace)
			continue
		}
		if isWatched {
			objs = append(objs, &deployments.Items[key])
		}
	}
	return objs, nil
}

func (h *Handler) Apply(obj client.Object, event events.SecretRotationEvent) error {
	return h.applyFn(obj, event)
}

func (h *Handler) _apply(obj client.Object, event events.SecretRotationEvent) error {
	logger := log.FromContext(h.ctx)
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return errors.New("obj isn't type Deployment")
	}
	tpl := deployment.Spec.Template
	annotations := tpl.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations["reloader.external-secrets.io/last-reloaded"] = event.RotationTimestamp
	annotations["reloader.external-secrets.io/trigger-source"] = event.TriggerSource

	tpl.SetAnnotations(annotations)
	deployment.Spec.Template = tpl
	if err := h.client.Update(h.ctx, deployment); err != nil {
		return fmt.Errorf("failed to update Deployment:%w", err)
	}
	logger.V(1).Info("Annotated Deployment", "name", deployment.GetName(), "namespace", deployment.GetNamespace())
	return nil
}

// isResourceWatched determines if a single Deployment matches any of the SecretsToWatch criteria.
func (h *Handler) isResourceWatched(deployment appsv1.Deployment, w v1alpha1.DestinationToWatch) (bool, error) {
	watchCriteria := w.Deployment
	if watchCriteria == nil {
		return false, errors.New("watch type is not deployment")
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
	namespaceMatch, err := util.MatchesAnyNamespaceSelector(h.ctx, &deployment, namespaceSelectors, h.client)
	if err != nil {
		return false, err
	}
	labelMatch, err := util.MatchesLabelSelectors(h.ctx, &deployment, labelSelector, h.client)
	if err != nil {
		return false, err
	}
	nameMatch := util.IsNameInList(&deployment, nameSet)
	if namespaceMatch && labelMatch && nameMatch {
		return true, nil
	}

	return false, nil
}

func (h *Handler) WaitFor(obj client.Object) error {
	return h.waitForFn(obj)
}

// _waitFor waits for the rollout status to be completed
func (h *Handler) _waitFor(obj client.Object) error {
	logger := log.FromContext(h.ctx)
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return errors.New("object is not a Deployment")
	}

	logger.V(1).Info("Waiting for Deployment rollout to complete", "name", deployment.GetName(), "namespace", deployment.GetNamespace())

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	// Set a timeout of 10 minutes
	timeout := time.After(10 * time.Minute)

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for deployment %s/%s rollout to complete", deployment.Namespace, deployment.Name)
		case <-ticker.C:
			// Get the latest deployment status
			currentDeployment := &appsv1.Deployment{}
			if err := h.client.Get(h.ctx, client.ObjectKey{Namespace: deployment.Namespace, Name: deployment.Name}, currentDeployment); err != nil {
				return fmt.Errorf("failed to get deployment: %w", err)
			}

			// Check if rollout is complete
			if isDeploymentRolloutComplete(currentDeployment) {
				logger.V(1).Info("Deployment rollout completed successfully", "name", deployment.GetName(), "namespace", deployment.GetNamespace())
				return nil
			}
		}
	}
}

// isDeploymentRolloutComplete checks if a deployment rollout is complete
func isDeploymentRolloutComplete(deployment *appsv1.Deployment) bool {
	// Ensure the deployment has the expected generation
	if deployment.Generation != deployment.Status.ObservedGeneration {
		return false
	}

	// Check if the deployment is paused
	if deployment.Spec.Paused {
		return false
	}

	// Check if the deployment is complete
	for _, condition := range deployment.Status.Conditions {
		if condition.Type == appsv1.DeploymentProgressing &&
			condition.Status == "True" &&
			condition.Reason == "NewReplicaSetAvailable" {
			// Check if all replicas are updated, available and ready
			if deployment.Spec.Replicas != nil {
				if deployment.Status.UpdatedReplicas == *deployment.Spec.Replicas &&
					deployment.Status.AvailableReplicas == *deployment.Spec.Replicas &&
					deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
					return true
				}
			}
		}
	}

	return false
}
func (h *Handler) References(obj client.Object, identifier string) (bool, error) {
	return h.referenceFn(obj, identifier)
}

// _references checks if the Deployment references the given secret identifier.
// It is the default References implementation
func (h *Handler) _references(obj client.Object, identifier string) (bool, error) {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return false, errors.New("obj isn't type Deployment")
	}
	// Check Data field
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if containsKey(container, identifier) {
			return true, nil
		}
	}
	return false, nil
}

func containsKey(container v1.Container, identifier string) bool {
	for _, env := range container.Env {
		// Referenced on a Secret
		if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil && env.ValueFrom.SecretKeyRef.Name == identifier {
			return true
		}
		// Referenced on a ConfigMap
		if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil && env.ValueFrom.ConfigMapKeyRef.Name == identifier {
			return true
		}
	}
	for _, envFrom := range container.EnvFrom {
		// Referenced on a Secret
		if envFrom.SecretRef != nil && envFrom.SecretRef.Name == identifier {
			return true
		}
		// Referenced on a ConfigMap
		if envFrom.ConfigMapRef != nil && envFrom.ConfigMapRef.Name == identifier {
			return true
		}
	}
	return false
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
