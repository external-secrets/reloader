package externalsecret

import (
	"context"
	"testing"

	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/external-secrets-inc/reloader/api/v1alpha1"
)

func externalSecretWithTemplateFromConfigMap(name string, configMapName string) *esov1.ExternalSecret {
	return &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			Target: esov1.ExternalSecretTarget{
				Template: &esov1.ExternalSecretTemplate{
					TemplateFrom: []esov1.TemplateFrom{
						{
							ConfigMap: &esov1.TemplateRef{
								Name: configMapName,
								Items: []esov1.TemplateRefItem{
									{Key: "config", TemplateAs: esov1.TemplateScopeValues},
								},
							},
						},
					},
				},
			},
		},
	}
}

func externalSecretWithTemplateFromSecret(name string, secretName string) *esov1.ExternalSecret {
	return &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			Target: esov1.ExternalSecretTarget{
				Template: &esov1.ExternalSecretTemplate{
					TemplateFrom: []esov1.TemplateFrom{
						{
							Secret: &esov1.TemplateRef{
								Name: secretName,
								Items: []esov1.TemplateRefItem{
									{Key: "tpl", TemplateAs: esov1.TemplateScopeValues},
								},
							},
						},
					},
				},
			},
		},
	}
}

func externalSecretWithTemplateFromMultiple(configMapName, secretName string) *esov1.ExternalSecret {
	templateFrom := []esov1.TemplateFrom{}
	if configMapName != "" {
		templateFrom = append(templateFrom, esov1.TemplateFrom{
			ConfigMap: &esov1.TemplateRef{Name: configMapName, Items: []esov1.TemplateRefItem{{Key: "config"}}},
		})
	}
	if secretName != "" {
		templateFrom = append(templateFrom, esov1.TemplateFrom{
			Secret: &esov1.TemplateRef{Name: secretName, Items: []esov1.TemplateRefItem{{Key: "tpl"}}},
		})
	}
	return &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "multi", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			Target: esov1.ExternalSecretTarget{
				Template: &esov1.ExternalSecretTemplate{TemplateFrom: templateFrom},
			},
		},
	}
}

func externalSecretWithRemoteRefKey(name string, remoteKey string) *esov1.ExternalSecret {
	return &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			Data: []esov1.ExternalSecretData{
				{SecretKey: "key", RemoteRef: esov1.ExternalSecretDataRemoteRef{Key: remoteKey}},
			},
		},
	}
}

func newHandlerWithDefaults() *Handler {
	ctx := context.Background()
	scheme := newScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	dest := v1alpha1.DestinationToWatch{
		Type:           "ExternalSecret",
		ExternalSecret: &v1alpha1.ExternalSecretDestination{},
	}
	h := &Handler{
		ctx:              ctx,
		client:           c,
		destinationCache: dest,
	}
	h.referenceFn = h._references
	return h
}

func newScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = esov1.AddToScheme(scheme)
	return scheme
}

// TestHandler_References_TemplateFromConfigMap verifies that an ExternalSecret
// that uses target.template.templateFrom[].configMap.name is considered to
// reference that ConfigMap (e.g. when the event is from a KubernetesConfigMap source).
func TestHandler_References_TemplateFromConfigMap(t *testing.T) {
	h := newHandlerWithDefaults()
	es := externalSecretWithTemplateFromConfigMap("operator", "operator-config")
	ref, err := h.References(es, "operator-config")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if !ref {
		t.Error("expected References to return true when templateFrom.configMap.name matches secretIdentifier")
	}
}

// TestHandler_References_TemplateFromConfigMapNoMatch verifies that when the
// ConfigMap name in templateFrom does not match the secret identifier,
// References returns false.
func TestHandler_References_TemplateFromConfigMapNoMatch(t *testing.T) {
	h := newHandlerWithDefaults()
	es := externalSecretWithTemplateFromConfigMap("operator", "other-config")
	ref, err := h.References(es, "operator-config")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if ref {
		t.Error("expected References to return false when templateFrom.configMap.name does not match")
	}
}

// TestHandler_References_TemplateFromSecret verifies that an ExternalSecret
// that uses target.template.templateFrom[].secret.name is considered to
// reference that Secret.
func TestHandler_References_TemplateFromSecret(t *testing.T) {
	h := newHandlerWithDefaults()
	es := externalSecretWithTemplateFromSecret("app", "my-secret")
	ref, err := h.References(es, "my-secret")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if !ref {
		t.Error("expected References to return true when templateFrom.secret.name matches secretIdentifier")
	}
}

// TestHandler_References_TemplateFromMultiple verifies that when templateFrom
// has both ConfigMap and Secret refs, matching either name returns true.
func TestHandler_References_TemplateFromMultiple(t *testing.T) {
	h := newHandlerWithDefaults()
	es := externalSecretWithTemplateFromMultiple("my-configmap", "my-secret")

	ref, err := h.References(es, "my-configmap")
	if err != nil {
		t.Fatalf("References(configmap): %v", err)
	}
	if !ref {
		t.Error("expected true for configmap name")
	}

	ref, err = h.References(es, "my-secret")
	if err != nil {
		t.Fatalf("References(secret): %v", err)
	}
	if !ref {
		t.Error("expected true for secret name")
	}

	ref, err = h.References(es, "other")
	if err != nil {
		t.Fatalf("References(other): %v", err)
	}
	if ref {
		t.Error("expected false for non-matching name")
	}
}

// TestHandler_References_RemoteRefKey verifies existing behavior: matching
// spec.data[].remoteRef.key returns true.
func TestHandler_References_RemoteRefKey(t *testing.T) {
	h := newHandlerWithDefaults()
	es := externalSecretWithRemoteRefKey("es", "my-remote-key")
	ref, err := h.References(es, "my-remote-key")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if !ref {
		t.Error("expected References to return true when remoteRef.key matches")
	}
}

// TestHandler_References_NotExternalSecret verifies that passing a non-ExternalSecret
// object returns an error.
func TestHandler_References_NotExternalSecret(t *testing.T) {
	h := newHandlerWithDefaults()
	obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "default"}}
	ref, err := h.References(obj, "x")
	if err == nil {
		t.Fatal("expected error when obj is not ExternalSecret")
	}
	if ref {
		t.Error("expected false when error is returned")
	}
}
