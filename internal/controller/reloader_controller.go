// controller/reloader_controller.go

package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/handler"
	"github.com/external-secrets/reloader/internal/listener"
	"github.com/external-secrets/reloader/internal/listener/webhook"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

type EventAction string

const (
	EventActionCreated  EventAction = "Created"
	EventActionUpdated  EventAction = "Updated"
	EventActionDeleted  EventAction = "Deleted"
	ProcessedAnnotation string      = "reloader/processed"
	reloaderFinalizer               = "reloader.external-secrets.io/finalizer"
)

// ReloaderReconciler reconciles an Reloader object
type ReloaderReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Internal fields
	listenerManager *listener.Manager
	webhookServer   *webhook.WebhookServer

	// eventChan is a channel that transports SecretRotationEvent instances between various parts of the system, such as event handlers and listeners.
	eventChan    chan events.SecretRotationEvent
	eventHandler *handler.EventHandler
}

// NewReloaderReconciler creates a new ReloaderReconciler with the default factory.
func NewReloaderReconciler(client client.Client, scheme *runtime.Scheme, hook *webhook.WebhookServer) *ReloaderReconciler {
	return &ReloaderReconciler{
		Client:        client,
		Scheme:        scheme,
		webhookServer: hook,
		eventChan:     make(chan events.SecretRotationEvent),
		eventHandler:  handler.NewEventHandler(client),
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ReloaderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx, cancel := context.WithCancel(context.Background())
	r.listenerManager = listener.NewListenerManager(ctx, r.eventChan, r.Client, log.FromContext(ctx), r.webhookServer)

	// Start a goroutine to process events
	go r.processEvents(ctx)

	// Graceful shutdown
	err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		<-ctx.Done()
		cancel()
		err := r.listenerManager.StopAll()
		if err != nil {
			return err
		}
		return nil
	}))
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Config{}).
		Complete(r)
}

// Auto Generated RBAC to ease a little bit the process
// For real installations, probably users will want to overwrite these.
// +kubebuilder:rbac:groups=reloader.external-secrets.io,resources=configs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=reloader.external-secrets.io,resources=configs/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=reloader.external-secrets.io,resources=configs/finalizers,verbs=update
// For k8s ExternalSecrets and PushSecrets destination
// +kubebuilder:rbac:groups=external-secrets.io,resources=externalsecrets;pushsecrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=workflows.external-secrets.io,resources=workflowruntemplates,verbs=get;list;watch;update;patch
// For k8s Deployments destination
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update;patch
// For k8s Secret notification source
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// For k8s ConfigMap notification source
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch

// Reconcile reconciles a Config object, ensuring that the internal state aligns with the desired state.
// It fetches the Reloader instance, updates the internal cache, and manages notification listeners.
// Returns ctrl.Result and an error on failure.
func (r *ReloaderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var cfg v1alpha1.Config

	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		if apierrors.IsNotFound(err) {
			// Object is gone (e.g. after finalizer). Only tear down listeners for this Config — not all Configs.
			manifestName := types.NamespacedName{
				Namespace: req.Namespace,
				Name:      req.Name,
			}
			if err := r.listenerManager.ManageListeners(manifestName, nil); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		}
		logger.Error(err, "unable to fetch Config")
		return ctrl.Result{}, err
	}
	if cfg.DeletionTimestamp != nil && controllerutil.ContainsFinalizer(&cfg, reloaderFinalizer) {
		// Handle any cleanup logic here, as this is a DELETE request
		manifestName := types.NamespacedName{
			Namespace: req.Namespace,
			Name:      req.Name,
		}
		if err := r.listenerManager.ManageListeners(manifestName, []v1alpha1.NotificationSource{}); err != nil {
			logger.Error(err, "failed to manage notification listeners")
			return ctrl.Result{}, err
		}
		controllerutil.RemoveFinalizer(&cfg, reloaderFinalizer)
		if err := r.Update(ctx, &cfg, &client.UpdateOptions{}); err != nil {
			return ctrl.Result{}, fmt.Errorf("could not update finalizers: %w", err)
		}
		logger.Info("Reloader deletion complete", "namespace", req.Namespace, "name", req.Name)
		return ctrl.Result{}, nil
	}
	// make sure we have finalizers
	if !controllerutil.ContainsFinalizer(&cfg, reloaderFinalizer) {
		controllerutil.AddFinalizer(&cfg, reloaderFinalizer)
		if err := r.Update(ctx, &cfg, &client.UpdateOptions{}); err != nil {
			return ctrl.Result{}, fmt.Errorf("could not update finalizers: %w", err)
		}
		// The Update already re-added to the reconcile queue - safe to just return here
		return ctrl.Result{}, nil
	}

	// Handle new resource
	if isResourceNew(&cfg) {
		logger.Info("New cfg detected. Performing initial setup.", "namespace", req.Namespace, "name", req.Name)

		// Add the processed annotation to mark this as not new anymore
		if cfg.Annotations == nil {
			cfg.Annotations = make(map[string]string)
		}

		processedAnnotation := cfg.Annotations[ProcessedAnnotation]

		// Ensure the annotation is added only if it doesn't exist
		if processedAnnotation == "" {
			cfg.Annotations[ProcessedAnnotation] = time.Now().Format(time.RFC3339)
			if err := r.Update(ctx, &cfg); err != nil {
				logger.Error(err, "Failed to update Reloader with processed annotation")
				return ctrl.Result{Requeue: true}, err
			}
		} else {
			logger.Info("Reloader has already been marked as processed.")
		}
	}

	// Reloader Update Detected
	r.eventHandler.UpdateDestinationsToWatch(cfg.Spec.DestinationsToWatch)
	manifestName := types.NamespacedName{
		Namespace: req.Namespace,
		Name:      req.Name,
	}
	if err := r.listenerManager.ManageListeners(manifestName, cfg.Spec.NotificationSources); err != nil {
		logger.Error(err, "failed to manage notification listeners")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// processEvents listens for SecretRotationEvents and handles them.
func (r *ReloaderReconciler) processEvents(ctx context.Context) {
	logger := log.FromContext(ctx)
	for {
		select {
		case event := <-r.eventChan:
			// Since events can take time to be processed due to waitFor conditions,
			// we should dispatch events on their own goroutine.
			// TODO[gusfcarvalho]: are there any possible conflicts with this?
			go func() {
				err := r.eventHandler.HandleEvent(ctx, event)
				if err != nil {
					logger.Error(err, "Failed to handle SecretRotationEvent", "SecretIdentifier", event.SecretIdentifier, "Source", event.TriggerSource)
				}
			}()
		case <-ctx.Done():
			return
		}
	}
}

// isResourceNew checks if the given Reloader resource is new by checking the presence of the processed annotation.
func isResourceNew(cfg *v1alpha1.Config) bool {
	if _, exists := cfg.Annotations[ProcessedAnnotation]; exists {
		return false
	}
	return true
}
