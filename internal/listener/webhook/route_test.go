package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/go-logr/logr"
)

func TestParsePayloadToString_RejectsOversizedBody(t *testing.T) {
	const limit = 64
	body := strings.NewReader(strings.Repeat("x", limit+1))
	rec := httptest.NewRecorder()

	_, err := parsePayloadToString(rec, io.NopCloser(body), limit)
	if err == nil {
		t.Fatal("expected error for oversized body")
	}
	if _, ok := err.(*http.MaxBytesError); !ok {
		t.Fatalf("expected MaxBytesError, got %T: %v", err, err)
	}
}

func TestRouteHandle_RejectsOversizedBody(t *testing.T) {
	r := newRoute(context.Background(), "cfg", &v1alpha1.WebhookConfig{}, nil, nil, logr.Discard())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/webhook/cfg", strings.NewReader(strings.Repeat("x", maxWebhookBodyBytes+1)))

	r.handle(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", rec.Code)
	}
}

func TestParsePayloadToString_AcceptsValidBody(t *testing.T) {
	payload := `{"0":{"data":{"ObjectName":"secret-one"}}}`
	rec := httptest.NewRecorder()

	got, err := parsePayloadToString(rec, io.NopCloser(strings.NewReader(payload)), maxWebhookBodyBytes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != payload {
		t.Fatalf("unexpected payload: %q", got)
	}
}

func TestRouteRetryMessage_RequeuesWithoutDeadlock(t *testing.T) {
	eventCh := make(chan events.SecretRotationEvent, 1)
	cfg := &v1alpha1.WebhookConfig{
		RetryPolicy: &v1alpha1.RetryPolicy{
			MaxRetries: 3,
			Algorithm:  "linear",
		},
	}
	r := newRoute(context.Background(), "cfg", cfg, nil, eventCh, logr.Discard())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.retryMessage(&retryMessage{
			event: events.SecretRotationEvent{
				SecretIdentifier: "secret-one",
			},
			currentRun: 1,
			retryAt:    time.Now(),
		}, 3)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("retry handling deadlocked")
	}
}

func TestRouteRetryMessage_ExhaustsRetriesWithoutDeadlock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	cfg := &v1alpha1.WebhookConfig{
		RetryPolicy: &v1alpha1.RetryPolicy{
			MaxRetries: 2,
			Algorithm:  "linear",
		},
	}
	r := newRoute(ctx, "cfg", cfg, nil, make(chan events.SecretRotationEvent), logr.Discard())

	done := make(chan struct{})
	go func() {
		defer close(done)
		r.retryMessage(&retryMessage{
			event: events.SecretRotationEvent{
				SecretIdentifier: "secret-one",
			},
			currentRun: 1,
			retryAt:    time.Now(),
		}, 2)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("retry handling deadlocked while exhausting retries")
	}
}
