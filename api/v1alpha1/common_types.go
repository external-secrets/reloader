package v1alpha1

type ServiceAccountSelector struct {

	// Name specifies the name of the service account to be selected.
	// +required
	Name string `json:"name"`

	// ServiceAccountSelector represents a Kubernetes service account with a name and namespace for selection purposes.
	// +required
	Namespace string `json:"namespace"`
	// Audience specifies the `aud` claim for the service account token
	// If the service account uses a well-known annotation for e.g. IRSA or GCP Workload Identity
	// then this audiences will be appended to the list
	// +optional
	Audiences []string `json:"audiences,omitempty"`
}

// SecretKeySelector is used to reference a specific secret within a Kubernetes namespace.
// It contains the name of the secret and the namespace where it resides.
type SecretKeySelector struct {

	// Name specifies the name of the referenced Kubernetes secret.
	// +required
	Name string `json:"name"`

	// Key specifies the key within the referenced Kubernetes secret.
	// +required
	Key string `json:"key"`

	// Namespace specifies the Kubernetes namespace where the referenced secret resides.
	// +required
	Namespace string `json:"namespace"`
}
