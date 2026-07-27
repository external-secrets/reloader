package util

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// getSecret retrieves a Kubernetes Secret.
func GetSecret(ctx context.Context, k8sClient client.Client, name, namespace string, logger logr.Logger) (*corev1.Secret, error) {
	logger.Info("Retrieving Kubernetes Secret", "SecretName", name, "Namespace", namespace)
	secret := &corev1.Secret{}
	key := types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}
	if err := k8sClient.Get(ctx, key, secret); err != nil {
		logger.Error(err, "Failed to get secret", "SecretName", name, "Namespace", namespace)
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}
	logger.Info("Successfully retrieved secret", "SecretName", name)
	return secret, nil
}
