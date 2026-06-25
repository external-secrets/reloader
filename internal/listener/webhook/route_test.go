package webhook

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
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
