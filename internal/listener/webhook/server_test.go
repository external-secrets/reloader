package webhook

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRecoverMiddleware(t *testing.T) {
	h := recoverMiddleware(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}, logr.Discard())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	h(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestWebhookServer_RegisterHasRouteUnregister(t *testing.T) {
	s := NewWebhookServer("127.0.0.1:1", logr.Discard())
	ctx := context.Background()
	s.Register("cfg-a", ctx, &v1alpha1.WebhookConfig{}, nil, nil, logr.Discard())
	if !s.HasRoute("cfg-a") {
		t.Fatal("expected route cfg-a")
	}
	s.Unregister("cfg-a")
	if s.HasRoute("cfg-a") {
		t.Fatal("expected route removed")
	}
}

func TestWebhookServer_ConcurrentRegister(t *testing.T) {
	s := NewWebhookServer("127.0.0.1:1", logr.Discard())
	ctx := context.Background()
	const n = 32
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			s.Register("same", ctx, &v1alpha1.WebhookConfig{
				SecretIdentifierOnPayload: "id",
			}, nil, nil, logr.Discard())
		}()
	}
	for i := 0; i < n; i++ {
		<-done
	}
	if !s.HasRoute("same") {
		t.Fatal("expected one stable route")
	}
}

func TestWebhookServer_ConcurrentRegisterWithRunningServer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	s := NewWebhookServer(addr, logr.Discard())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = s.Start(ctx) }()
	waitTCP(t, addr)

	eventCh := make(chan events.SecretRotationEvent, 16)
	cl := fake.NewClientBuilder().Build()

	const n = 32
	done := make(chan struct{})
	for i := 0; i < n; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			name := fmt.Sprintf("cfg-%d", i%4)
			s.Register(name, ctx, &v1alpha1.WebhookConfig{}, cl, eventCh, logr.Discard())
		}(i)
	}
	for i := 0; i < n; i++ {
		<-done
	}

	body := `{"0":{"data":{"ObjectName":"secret-one"}}}`
	for _, name := range []string{"cfg-0", "cfg-1", "cfg-2", "cfg-3"} {
		req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/webhook/"+name, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("route %s: expected 204, got %d", name, resp.StatusCode)
		}
	}

	cancel()
}

func TestRouteKeyFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
		ok   bool
	}{
		{"/webhook/myconfig", "myconfig", true},
		{"/webhook/myconfig/abc123", "myconfig/abc123", true},
		{"/webhook/", "", false},
		{"/webhook/a/b/c", "", false},
		{"/other/myconfig", "", false},
	}
	for _, tt := range tests {
		got, ok := routeKeyFromPath(tt.path)
		if ok != tt.ok || got != tt.want {
			t.Errorf("routeKeyFromPath(%q) = (%q, %v), want (%q, %v)", tt.path, got, ok, tt.want, tt.ok)
		}
	}
}

func TestRouteKey(t *testing.T) {
	if got := RouteKey("keeper", "", "Webhook-deadbeef", 1); got != "keeper" {
		t.Fatalf("single webhook: got %q, want keeper", got)
	}
	if got := RouteKey("keeper", "vendor-a", "Webhook-deadbeef", 1); got != "keeper/vendor-a" {
		t.Fatalf("pathSuffix: got %q, want keeper/vendor-a", got)
	}
	if got := RouteKey("keeper", "", "Webhook-deadbeef", 2); got != "keeper/deadbeef" {
		t.Fatalf("multiple webhooks: got %q, want keeper/deadbeef", got)
	}
	if got := RouteKey("keeper", "vendor-a", "Webhook-deadbeef", 2); got != "keeper/vendor-a" {
		t.Fatalf("pathSuffix with multiple webhooks: got %q, want keeper/vendor-a", got)
	}
}

func waitTCP(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, 50*time.Millisecond)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("address %s did not become reachable", addr)
}

func TestWebhookServer_HTTPPostNoAuth(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	s := NewWebhookServer(addr, logr.Discard())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = s.Start(ctx) }()
	waitTCP(t, addr)

	eventCh := make(chan events.SecretRotationEvent, 2)
	cl := fake.NewClientBuilder().Build()

	s.Register("myconfig", ctx, &v1alpha1.WebhookConfig{}, cl, eventCh, logr.Discard())

	body := `{"0":{"data":{"ObjectName":"secret-one"}}}`
	req, err := http.NewRequest(http.MethodPost, "http://"+addr+"/webhook/myconfig", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	select {
	case ev := <-eventCh:
		if ev.SecretIdentifier != "secret-one" {
			t.Fatalf("unexpected event: %+v", ev)
		}
	default:
		t.Fatal("expected event on channel")
	}

	cancel()
}
