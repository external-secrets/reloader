package listener

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	esov1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/listener/schema"
)

func TestGenerateListenerKey_KubernetesConfigMap(t *testing.T) {
	source := esov1alpha1.NotificationSource{
		Type: schema.KUBERNETES_CONFIG_MAP,
		KubernetesConfigMap: &esov1alpha1.KubernetesConfigMapConfig{
			ServerURL: "https://kubernetes.default.svc",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"platform.enreach.tech/reloader": "true"},
			},
		},
	}
	key, err := generateListenerKey(source)
	if err != nil {
		t.Fatalf("generateListenerKey: %v", err)
	}
	if key == "" {
		t.Error("expected non-empty key")
	}
	if len(key) < len(schema.KUBERNETES_CONFIG_MAP)+2 {
		t.Errorf("key should start with type and hash, got %q", key)
	}
	// Same config produces same key
	key2, err := generateListenerKey(source)
	if err != nil {
		t.Fatalf("generateListenerKey (second call): %v", err)
	}
	if key != key2 {
		t.Errorf("same config should produce same key: %q vs %q", key, key2)
	}
}

func TestGenerateListenerKey_KubernetesConfigMap_DifferentConfigsDifferentKeys(t *testing.T) {
	s1 := esov1alpha1.NotificationSource{
		Type: schema.KUBERNETES_CONFIG_MAP,
		KubernetesConfigMap: &esov1alpha1.KubernetesConfigMapConfig{
			ServerURL: "https://kubernetes.default.svc",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"a": "1"},
			},
		},
	}
	s2 := esov1alpha1.NotificationSource{
		Type: schema.KUBERNETES_CONFIG_MAP,
		KubernetesConfigMap: &esov1alpha1.KubernetesConfigMapConfig{
			ServerURL: "https://kubernetes.default.svc",
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"b": "2"},
			},
		},
	}
	key1, err := generateListenerKey(s1)
	if err != nil {
		t.Fatalf("generateListenerKey s1: %v", err)
	}
	key2, err := generateListenerKey(s2)
	if err != nil {
		t.Fatalf("generateListenerKey s2: %v", err)
	}
	if key1 == key2 {
		t.Errorf("different configs should produce different keys: %q", key1)
	}
}

func TestGenerateListenerKey_KubernetesConfigMap_UnsupportedType(t *testing.T) {
	source := esov1alpha1.NotificationSource{
		Type: "InvalidType",
	}
	_, err := generateListenerKey(source)
	if err == nil {
		t.Fatal("expected error for unsupported type")
	}
}
