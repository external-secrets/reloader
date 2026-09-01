package handler

import (
	"context"
	"testing"

	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	esreloaderv1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
)

// These tests exercise HandleEvent's wiring of MatchStrategy: when a
// destination configures one, it must replace the destination type's
// built-in References() check, and any error evaluating it must propagate
// out of HandleEvent rather than being swallowed.

func newFakeClientWithExternalSecret(t *testing.T, es *esov1.ExternalSecret) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := esov1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme: %v", err)
	}
	builder := fake.NewClientBuilder().WithScheme(scheme)
	if es != nil {
		builder = builder.WithObjects(es)
	}
	return builder.Build()
}

func getAnnotations(t *testing.T, c client.Client, name, namespace string) map[string]string {
	t.Helper()
	es := &esov1.ExternalSecret{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: name, Namespace: namespace}, es); err != nil {
		t.Fatalf("Get: %v", err)
	}
	return es.GetAnnotations()
}

func TestHandleEvent_MatchStrategyReplacesDefaultReferences(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ai-gateway", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "platform/ai-gateway/service-secrets"}},
			},
		},
	}
	c := newFakeClientWithExternalSecret(t, es)

	eh := NewEventHandler(c)
	eh.UpdateDestinationsToWatch([]esreloaderv1alpha1.DestinationToWatch{
		{
			Type:           "ExternalSecret",
			ExternalSecret: &esreloaderv1alpha1.ExternalSecretDestination{Names: []string{"ai-gateway"}},
			MatchStrategy: &esreloaderv1alpha1.MatchStrategy{
				Path: "spec.dataFrom[*].extract.key",
				Conditions: []esreloaderv1alpha1.Condition{
					{Value: "{{ .SecretIdentifier }}", Operation: esreloaderv1alpha1.ConditionOperationContainedBy},
				},
			},
		},
	})

	// The default References() check would strictly compare this ARN
	// against dataFrom.extract.key and never match. The MatchStrategy
	// above must be the thing that decides this, and it should match.
	event := events.SecretRotationEvent{
		SecretIdentifier:  "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/ai-gateway/service-secrets-78YXTj",
		RotationTimestamp: "2026-07-23T17:25:37Z",
		TriggerSource:     "aws-secretsmanager",
	}
	if err := eh.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	annotations := getAnnotations(t, c, "ai-gateway", "default")
	if annotations["reloader/last-rotated"] != "2026-07-23T17:25:37Z" {
		t.Errorf("expected the ExternalSecret to be annotated via the MatchStrategy match, got annotations: %v", annotations)
	}
}

func TestHandleEvent_MatchStrategyNoMatchLeavesObjectUntouched(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ai-gateway", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "platform/ai-gateway/service-secrets"}},
			},
		},
	}
	c := newFakeClientWithExternalSecret(t, es)

	eh := NewEventHandler(c)
	eh.UpdateDestinationsToWatch([]esreloaderv1alpha1.DestinationToWatch{
		{
			Type:           "ExternalSecret",
			ExternalSecret: &esreloaderv1alpha1.ExternalSecretDestination{Names: []string{"ai-gateway"}},
			MatchStrategy: &esreloaderv1alpha1.MatchStrategy{
				Path: "spec.dataFrom[*].extract.key",
				Conditions: []esreloaderv1alpha1.Condition{
					{Value: "{{ .SecretIdentifier }}", Operation: esreloaderv1alpha1.ConditionOperationContainedBy},
				},
			},
		},
	})

	event := events.SecretRotationEvent{
		SecretIdentifier:  "arn:aws:secretsmanager:us-east-1:051826739313:secret:platform/unrelated-service/service-secrets-78YXTj",
		RotationTimestamp: "2026-07-23T17:25:37Z",
	}
	if err := eh.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	annotations := getAnnotations(t, c, "ai-gateway", "default")
	if annotations != nil {
		t.Errorf("expected no annotations for an unrelated secret, got: %v", annotations)
	}
}

func TestHandleEvent_MatchStrategyErrorPropagates(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ai-gateway", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "platform/ai-gateway/service-secrets"}},
			},
		},
	}
	c := newFakeClientWithExternalSecret(t, es)

	eh := NewEventHandler(c)
	eh.UpdateDestinationsToWatch([]esreloaderv1alpha1.DestinationToWatch{
		{
			Type:           "ExternalSecret",
			ExternalSecret: &esreloaderv1alpha1.ExternalSecretDestination{Names: []string{"ai-gateway"}},
			MatchStrategy: &esreloaderv1alpha1.MatchStrategy{
				Path: "spec.dataFrom[*].extract.key",
				Conditions: []esreloaderv1alpha1.Condition{
					// Invalid regular expression: this is a configuration
					// error and must be surfaced, not silently swallowed
					// into a "no match".
					{Value: "(", Operation: esreloaderv1alpha1.ConditionOperationIn},
				},
			},
		},
	})

	event := events.SecretRotationEvent{SecretIdentifier: "anything"}
	if err := eh.HandleEvent(context.Background(), event); err == nil {
		t.Fatal("expected HandleEvent to return an error when MatchStrategy evaluation fails")
	}

	// And, since evaluation errored rather than resolving to "no match", the
	// object must not have been touched either.
	annotations := getAnnotations(t, c, "ai-gateway", "default")
	if annotations != nil {
		t.Errorf("expected no annotations when MatchStrategy evaluation errors, got: %v", annotations)
	}
}

func TestHandleEvent_NoMatchStrategyFallsBackToDefaultReferences(t *testing.T) {
	es := &esov1.ExternalSecret{
		ObjectMeta: metav1.ObjectMeta{Name: "ai-gateway", Namespace: "default"},
		Spec: esov1.ExternalSecretSpec{
			DataFrom: []esov1.ExternalSecretDataFromRemoteRef{
				{Extract: &esov1.ExternalSecretDataRemoteRef{Key: "platform/ai-gateway/service-secrets"}},
			},
		},
	}
	c := newFakeClientWithExternalSecret(t, es)

	eh := NewEventHandler(c)
	eh.UpdateDestinationsToWatch([]esreloaderv1alpha1.DestinationToWatch{
		{
			Type:           "ExternalSecret",
			ExternalSecret: &esreloaderv1alpha1.ExternalSecretDestination{Names: []string{"ai-gateway"}},
			// No MatchStrategy: falls back to the default References()
			// check, which does a strict equality match.
		},
	})

	// Exact match against dataFrom.extract.key should still work via the
	// default (unchanged) behavior.
	event := events.SecretRotationEvent{
		SecretIdentifier:  "platform/ai-gateway/service-secrets",
		RotationTimestamp: "2026-07-23T17:25:37Z",
	}
	if err := eh.HandleEvent(context.Background(), event); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}

	annotations := getAnnotations(t, c, "ai-gateway", "default")
	if annotations["reloader/last-rotated"] != "2026-07-23T17:25:37Z" {
		t.Errorf("expected default References() matching to still work, got annotations: %v", annotations)
	}
}

// TestHandleEvent_UnknownProviderIsSkipped verifies HandleEvent doesn't fail
// the whole event when a destination type has no registered provider - it
// should just skip that destination.
func TestHandleEvent_UnknownProviderIsSkipped(t *testing.T) {
	c := newFakeClientWithExternalSecret(t, nil)
	eh := NewEventHandler(c)
	eh.UpdateDestinationsToWatch([]esreloaderv1alpha1.DestinationToWatch{
		{Type: "NotARealDestinationType"},
	})

	if err := eh.HandleEvent(context.Background(), events.SecretRotationEvent{SecretIdentifier: "x"}); err != nil {
		t.Fatalf("HandleEvent: %v", err)
	}
}
