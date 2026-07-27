package util

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"

	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TokenRetriever struct {
	k8sClient               client.Client
	serviceAccountName      string
	serviceAccountNamespace string
	audiences               []string
	logger                  logr.Logger
}

func NewTokenRetriever(k8sClient client.Client, logger logr.Logger, serviceAccountName, serviceAccountNamespace string) *TokenRetriever {
	return &TokenRetriever{
		k8sClient:               k8sClient,
		serviceAccountName:      serviceAccountName,
		serviceAccountNamespace: serviceAccountNamespace,
		logger:                  logger,
		audiences:               []string{"sts.amazonaws.com"},
	}
}

func (tr *TokenRetriever) GetServiceAccountToken() ([]byte, error) {
	tr.logger.Info("Attempting to retrieve service account token",
		"ServiceAccount", tr.serviceAccountName,
		"Namespace", tr.serviceAccountNamespace,
		"Audiences", tr.audiences,
	)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tr.serviceAccountName,
			Namespace: tr.serviceAccountNamespace,
		},
	}

	tokenRequest := &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{
			Audiences: tr.audiences,
		},
	}

	err := tr.k8sClient.SubResource("token").Create(context.Background(), serviceAccount, tokenRequest)
	if err != nil {
		tr.logger.Error(err, "Error creating service account token",
			"ServiceAccount", tr.serviceAccountName,
			"Namespace", tr.serviceAccountNamespace,
		)
		return nil, fmt.Errorf("error creating service account token: %w", err)
	}

	tr.logger.Info("Successfully retrieved service account token",
		"ServiceAccount", tr.serviceAccountName,
		"Namespace", tr.serviceAccountNamespace,
	)

	return []byte(tokenRequest.Status.Token), nil
}

func (tr *TokenRetriever) GetIdentityToken() ([]byte, error) {
	tr.logger.Info("Attempting to retrieve identity token")

	token, err := tr.GetServiceAccountToken()
	if err != nil {
		tr.logger.Error(err, "Error retrieving identity token")
		return nil, err
	}

	tr.logger.Info("Successfully retrieved identity token")

	return token, nil
}
