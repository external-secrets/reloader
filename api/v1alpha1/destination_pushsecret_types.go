package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Defines an PushSecretDestination. Behavior is an annotations patch.
// Default UpdateStrategy is annotations patch to trigger pushsecret reconcile.
// Default MatchStrategy is matching secret-key with:
// * Equality against `spec.selector.secret.name`
// * Equality against `spec.data[*].match.remoteRef.remoteKey`

type PushSecretDestination struct {
	// NamespaceSelectors selects namespaces based on labels.
	// The manifest must reside in a namespace that matches at least one of these selectors.
	// +optional
	NamespaceSelectors []metav1.LabelSelector `json:"namespaceSelectors,omitempty"`

	// LabelSelectors selects resources based on their labels.
	// The resource must satisfy all conditions defined in this selector.
	// Supports both matchLabels and matchExpressions for advanced filtering.
	// +optional
	LabelSelectors *metav1.LabelSelector `json:"labelSelectors,omitempty"`

	// Names specifies a list of resource names to watch.
	// The resource must have a name that matches one of these entries.
	// +optional
	Names []string `json:"names,omitempty"`
}
