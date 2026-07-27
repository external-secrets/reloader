package util

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// matchesAnyNamespaceSelector checks if the namespace labels match any of the provided namespace selectors.
func MatchesAnyNamespaceSelector(ctx context.Context, obj client.Object, namespaceSelectors []labels.Selector, client client.Client) (bool, error) {
	if len(namespaceSelectors) == 0 {
		// If no namespace selectors are provided, it's not a match
		return true, nil
	}

	// Get the namespace object
	var namespace corev1.Namespace
	if err := client.Get(ctx, types.NamespacedName{Name: obj.GetNamespace()}, &namespace); err != nil {
		return false, fmt.Errorf("failed to get namespace: %w", err)
	}

	// Check if the namespace labels match any of the selectors
	for _, nsSelector := range namespaceSelectors {
		if nsSelector.Matches(labels.Set(namespace.Labels)) {
			return true, nil
		}
	}
	return false, nil
}

// matchesLabelSelectors checks if the secret's labels match the provided label selector.
func MatchesLabelSelectors(ctx context.Context, obj client.Object, labelSelector labels.Selector, client client.Client) (bool, error) {
	if labelSelector == nil {
		// If no label selector is provided, consider it a match.
		return true, nil
	}

	return labelSelector.Matches(labels.Set(obj.GetLabels())), nil
}

// isNameInList checks if the secret's name is in the provided names set.
func IsNameInList(obj client.Object, nameSet map[string]struct{}) bool {
	if len(nameSet) == 0 {
		// If no names are specified, consider it a match.
		return true
	}

	_, exists := nameSet[obj.GetName()]
	return exists
}
