package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// KubernetesConfigMapConfig contains configuration for Kubernetes notifications.
type KubernetesConfigMapConfig struct {
	// Server URL
	// +required
	ServerURL string `json:"serverURL"`

	// How to authenticate with Kubernetes cluster. If not specified, the default config is used.
	// +optional
	Auth *KubernetesAuth `json:"auth,omitempty"`

	// LabelSelector can be used to identify and narrow down secrets for watching.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
}
