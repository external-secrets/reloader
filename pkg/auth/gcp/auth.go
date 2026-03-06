package gcp

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	kclient "sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/util/resolvers"
)

const (
	CloudPlatformRole = "https://www.googleapis.com/auth/cloud-platform"
)

func NewTokenSource(ctx context.Context, auth *v1alpha1.GooglePubSubAuth, projectID string, kube kclient.Client) (oauth2.TokenSource, error) {
	if auth == nil {
		return google.DefaultTokenSource(ctx, CloudPlatformRole)
	}
	ts, err := tokenSourceFromSecret(ctx, auth, kube)
	if ts != nil || err != nil {
		return ts, err
	}
	if auth.WorkloadIdentity != nil && auth.WorkloadIdentity.ClusterProjectID != "" {
		projectID = auth.WorkloadIdentity.ClusterProjectID
	}
	wi, err := newWorkloadIdentity(ctx, projectID)
	if err != nil {
		return nil, errors.New("unable to initialize workload identity")
	}
	defer wi.Close() //nolint
	ts, err = wi.TokenSource(ctx, auth, kube)
	if ts != nil || err != nil {
		return ts, err
	}
	return google.DefaultTokenSource(ctx, CloudPlatformRole)
}

func tokenSourceFromSecret(ctx context.Context, auth *v1alpha1.GooglePubSubAuth, kube kclient.Client) (oauth2.TokenSource, error) {
	sr := auth.SecretRef
	if sr == nil {
		return nil, nil
	}
	credentials, err := resolvers.SecretKeyRef(
		ctx,
		kube,
		&auth.SecretRef.SecretAccessKey)
	if err != nil {
		return nil, err
	}
	config, err := google.JWTConfigFromJSON([]byte(credentials), CloudPlatformRole)
	if err != nil {
		return nil, fmt.Errorf("unable to process jwt credentials:%w", err)
	}
	return config.TokenSource(ctx), nil
}
