package kubernetes

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/pkg/util"
	"github.com/go-logr/logr"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Handler creates a kubernetes Secret watch on a given cluster and sends event
// on any operation propagated to the watch (create, update, delete).
type Handler[T client.Object] struct {
	Config        *v1alpha1.KubernetesObjectConfig
	Ctx           context.Context
	Cancel        context.CancelFunc
	Client        client.Client
	Mgr           ctrl.Manager
	CtrlClientSet typedcorev1.CoreV1Interface
	EventChan     chan events.SecretRotationEvent
	Logger        logr.Logger
	VersionMap    sync.Map // map[types.NamespacedName]string
	Obj           T
	Name          string
}

// Start initiates the Kubernetes Secret listener.
func (h *Handler[T]) Start() error {
	log := ctrl.Log.WithName(h.Name + "-watcher")
	cfg, err := h.getKubeConfig()
	if err != nil {
		return fmt.Errorf("could not parse config: %w", err)
	}
	h.CtrlClientSet, err = typedcorev1.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("could not create client set: %w", err)
	}
	metricsServerOptions := metricsserver.Options{
		BindAddress: "0",
	}
	manager, err := ctrl.NewManager(cfg, ctrl.Options{
		Metrics: metricsServerOptions,
	})
	if err != nil {
		return fmt.Errorf("could not create manager: %w", err)
	}
	var opts []builder.WatchesOption
	if h.Config.LabelSelector != nil {
		selectorPredicate, err := predicate.LabelSelectorPredicate(*h.Config.LabelSelector)
		if err != nil {
			return fmt.Errorf("could not create selector predicate: %w", err)
		}
		opts = append(opts, builder.WithPredicates(selectorPredicate))
	}
	err = ctrl.
		NewControllerManagedBy(manager).
		Named("k8s"+h.Name).
		Watches(h.Obj, handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, s client.Object) []ctrl.Request {
			secret, ok := s.(T)
			if !ok {
				log.Error(err, "while processing secret")
				return nil
			}
			if secret.GetDeletionTimestamp() != nil {
				log.V(2).Info("skipping deleted secret", "namespace", secret.GetNamespace(), "name", secret.GetName())
				return nil
			}
			version := secret.GetResourceVersion()
			storedVersion, loaded := h.VersionMap.LoadOrStore(types.NamespacedName{Namespace: secret.GetNamespace(), Name: secret.GetName()}, version)
			if !loaded {
				log.V(2).Info(h.Name+" not added to cache, skipping", "namespace", secret.GetNamespace(), "name", secret.GetName())
				return nil
			}
			if version == storedVersion {
				// Happens for some weird reason
				log.V(2).Info("skipping object with same version", "namespace", secret.GetNamespace(), "name", secret.GetName())
				return nil
			}
			// Safe to send event
			h.VersionMap.Store(types.NamespacedName{Namespace: secret.GetNamespace(), Name: secret.GetName()}, version)
			h.EventChan <- events.SecretRotationEvent{
				SecretIdentifier:  secret.GetName(),
				Namespace:         secret.GetNamespace(),
				RotationTimestamp: time.Now().Format(time.RFC3339),
				TriggerSource:     fmt.Sprintf("%s/%s", schema.KUBERNETES_SECRET, secret.GetName()),
			}
			return nil
		}), opts...).
		Complete(reconcile.Func(func(ctx context.Context, r reconcile.Request) (reconcile.Result, error) {
			// We dont need to reconcile anything, as we are sending this over another controller
			return reconcile.Result{}, nil
		}))
	if err != nil {
		return fmt.Errorf("could not create controller: %w", err)
	}
	h.Mgr = manager
	go func() {
		if err := manager.Start(h.Ctx); err != nil {
			h.Logger.Error(err, "failed to start watching "+h.Name)
		}
	}()
	return nil
}

// Stop stops the Watch by closing the stop channel.
func (h *Handler[T]) Stop() error {
	h.Cancel()
	return nil
}

func (h *Handler[T]) getKubeConfig() (*rest.Config, error) {
	if h.Config.Auth == nil {
		h.Logger.V(1).Info("no auth specified - using default config")
		cfg, err := ctrl.GetConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get default config: %w", err)
		}
		return cfg, nil
	}
	if h.Config.Auth.KubeConfigRef != nil {
		cfg, err := h.fetchSecretKey(h.Ctx, &h.Config.Auth.KubeConfigRef.SecretRef)
		if err != nil {
			return nil, err
		}

		return clientcmd.RESTConfigFromKubeConfig(cfg)
	}

	if h.Config.ServerURL == "" {
		return nil, errors.New("no server URL provided")
	}

	cfg := &rest.Config{
		Host: h.Config.ServerURL,
	}

	ca, err := h.fetchCACertFromSource(certOpts{
		CABundle: []byte(h.Config.Auth.CABundle),
	})
	if err != nil {
		return nil, err
	}

	cfg.TLSClientConfig = rest.TLSClientConfig{
		Insecure: false,
		CAData:   ca,
	}

	switch {
	case h.Config.Auth.TokenRef != nil:
		token, err := h.fetchSecretKey(h.Ctx, &h.Config.Auth.TokenRef.SecretRef)
		if err != nil {
			return nil, fmt.Errorf("could not fetch Auth.Token.BearerToken: %w", err)
		}
		cfg.BearerToken = string(token)
	case h.Config.Auth.ServiceAccountRef != nil:
		token, err := h.serviceAccountToken(h.Ctx, h.Config.Auth.ServiceAccountRef)
		if err != nil {
			return nil, fmt.Errorf("could not fetch Auth.ServiceAccount: %w", err)
		}
		cfg.BearerToken = string(token)
	default:
		return nil, errors.New("no auth provider given")
	}

	return cfg, nil

}

func (h *Handler[T]) fetchSecretKey(ctx context.Context, secretKeySelector *v1alpha1.SecretKeySelector) ([]byte, error) {
	if secretKeySelector == nil {
		return nil, errors.New("secret key selector is nil")
	}
	secret, err := util.GetSecret(ctx, h.Client, secretKeySelector.Name, secretKeySelector.Namespace, h.Logger)
	if err != nil {
		return nil, fmt.Errorf("could not fetch secret key: %w", err)
	}
	data, ok := secret.Data[secretKeySelector.Key]
	if !ok {
		return nil, fmt.Errorf("key %s not found in secret %s", secretKeySelector.Key, secretKeySelector.Name)
	}
	return data, nil
}

func (h *Handler[T]) serviceAccountToken(ctx context.Context, serviceAccountSelector *v1alpha1.ServiceAccountSelector) ([]byte, error) {
	if h.CtrlClientSet == nil {
		return nil, errors.New("controller client not initialized; creating account token is unavailable")
	}

	if serviceAccountSelector == nil {
		return nil, errors.New("service account selector is nil")
	}
	namespace := serviceAccountSelector.Namespace
	expirationSeconds := int64(3600)
	tr, err := h.CtrlClientSet.ServiceAccounts(namespace).CreateToken(ctx, serviceAccountSelector.Name, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         serviceAccountSelector.Audiences,
			ExpirationSeconds: &expirationSeconds,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("could not create token: %w", err)
	}
	return []byte(tr.Status.Token), nil
}

func (h *Handler[T]) fetchCACertFromSource(certOpts certOpts) ([]byte, error) {
	if len(certOpts.CABundle) > 0 {
		pem, err := base64decode(certOpts.CABundle)
		if err != nil {
			return nil, fmt.Errorf("failed to decode ca bundle: %w", err)
		}

		return pem, nil
	}
	return nil, nil
}

type certOpts struct {
	CABundle []byte
}

func base64decode(cert []byte) ([]byte, error) {
	if c, err := parseCertificateBytes(cert); err == nil {
		return c, nil
	}

	// try b64 decoding and test for validity again...
	certificate, err := base64.StdEncoding.DecodeString(string(cert))
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	return parseCertificateBytes(certificate)
}

func parseCertificateBytes(certBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(certBytes)
	if block == nil {
		return nil, errors.New("failed to parse the new certificate, not valid pem data")
	}

	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, fmt.Errorf("failed to validate certificate: %w", err)
	}

	return certBytes, nil
}
