package pushsecret

import (
	"context"
	"testing"

	esv1alpha1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/external-secrets/reloader/api/v1alpha1"
)

func newHandlerWithDefaults() *Handler {
	ctx := context.Background()
	scheme := newScheme()
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	dest := v1alpha1.DestinationToWatch{
		Type:       "PushSecret",
		PushSecret: &v1alpha1.PushSecretDestination{},
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
	_ = esv1alpha1.AddToScheme(scheme)
	return scheme
}

// TestHandler_References_RemoteKey verifies existing behavior: matching
// spec.data[].match.remoteRef.remoteKey returns true.
func TestHandler_References_RemoteKey(t *testing.T) {
	h := newHandlerWithDefaults()
	ps := &esv1alpha1.PushSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "default"},
		Spec: esv1alpha1.PushSecretSpec{
			Data: []esv1alpha1.PushSecretData{
				{Match: esv1alpha1.PushSecretMatch{RemoteRef: esv1alpha1.PushSecretRemoteRef{RemoteKey: "my-remote-key"}}},
			},
		},
	}
	ref, err := h.References(ps, "my-remote-key")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if !ref {
		t.Error("expected References to return true when remoteRef.remoteKey matches")
	}
}

// TestHandler_References_RemoteKeyAWSARN verifies that a PushSecret
// referencing a secret by its AWS Secrets Manager friendly name is still
// considered referenced when the event's secretIdentifier is the full ARN
// that AWS notification sources actually deliver.
func TestHandler_References_RemoteKeyAWSARN(t *testing.T) {
	h := newHandlerWithDefaults()
	ps := &esv1alpha1.PushSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "default"},
		Spec: esv1alpha1.PushSecretSpec{
			Data: []esv1alpha1.PushSecretData{
				{Match: esv1alpha1.PushSecretMatch{RemoteRef: esv1alpha1.PushSecretRemoteRef{RemoteKey: "platform/ai-gateway/service-secrets"}}},
			},
		},
	}
	ref, err := h.References(ps, "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if !ref {
		t.Error("expected References to return true when remoteKey matches the friendly name embedded in an AWS Secrets Manager ARN")
	}
}

// TestHandler_References_SelectorSecretName verifies existing behavior:
// matching spec.selector.secret.name returns true.
func TestHandler_References_SelectorSecretName(t *testing.T) {
	h := newHandlerWithDefaults()
	ps := &esv1alpha1.PushSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "default"},
		Spec: esv1alpha1.PushSecretSpec{
			Selector: esv1alpha1.PushSecretSelector{
				Secret: &esv1alpha1.PushSecretSecret{Name: "local-secret"},
			},
		},
	}
	ref, err := h.References(ps, "local-secret")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if !ref {
		t.Error("expected References to return true when selector.secret.name matches")
	}
}

// TestHandler_References_NoMatch verifies that an unrelated identifier does
// not match.
func TestHandler_References_NoMatch(t *testing.T) {
	h := newHandlerWithDefaults()
	ps := &esv1alpha1.PushSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ps", Namespace: "default"},
		Spec: esv1alpha1.PushSecretSpec{
			Data: []esv1alpha1.PushSecretData{
				{Match: esv1alpha1.PushSecretMatch{RemoteRef: esv1alpha1.PushSecretRemoteRef{RemoteKey: "my-remote-key"}}},
			},
		},
	}
	ref, err := h.References(ps, "unrelated")
	if err != nil {
		t.Fatalf("References: %v", err)
	}
	if ref {
		t.Error("expected References to return false for an unrelated identifier")
	}
}
