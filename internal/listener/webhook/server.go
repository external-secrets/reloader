package webhook

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const webhookRoutePrefix = "/webhook/"

// WebhookServer manages a single shared HTTP server for all webhook Config CRs.
// Each Config gets a route at POST /webhook/{config-name}, or POST /webhook/{config-name}/{pathSuffix}
// when pathSuffix is set on the webhook notification source.
type WebhookServer struct {
	addr           string
	server         *http.Server
	routes         map[string]*route
	mu             sync.RWMutex
	logger         logr.Logger
	httpShutdownMu sync.Once
}

// NewWebhookServer constructs a server; call Start to listen.
func NewWebhookServer(addr string, logger logr.Logger) *WebhookServer {
	s := &WebhookServer{
		addr:   addr,
		routes: make(map[string]*route),
		logger: logger.WithName("webhook-server"),
	}
	s.server = &http.Server{
		Addr:    addr,
		Handler: s,
	}
	return s
}

// ServeHTTP dispatches incoming requests to the registered route under a read lock,
// avoiding reassignment of server.Handler while ListenAndServe is running.
func (s *WebhookServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	routeKey, ok := routeKeyFromPath(req.URL.Path)
	if !ok {
		http.NotFound(w, req)
		return
	}

	s.mu.RLock()
	route, found := s.routes[routeKey]
	s.mu.RUnlock()
	if !found {
		http.NotFound(w, req)
		return
	}

	recoverMiddleware(route.handle, s.logger)(w, req)
}

func routeKeyFromPath(path string) (string, bool) {
	if !strings.HasPrefix(path, webhookRoutePrefix) {
		return "", false
	}
	rest := strings.TrimPrefix(path, webhookRoutePrefix)
	if rest == "" {
		return "", false
	}
	parts := strings.Split(rest, "/")
	switch len(parts) {
	case 1:
		return parts[0], true
	case 2:
		if parts[0] == "" || parts[1] == "" {
			return "", false
		}
		return parts[0] + "/" + parts[1], true
	default:
		return "", false
	}
}

// RouteKey returns the HTTP route key for a webhook notification source.
// An explicit pathSuffix yields {configName}/{pathSuffix}. Without a suffix, a single
// webhook source keeps the Config name; additional sources on the same Config fall
// back to {configName}/{listenerHash} so routes are not overwritten.
func RouteKey(configName, pathSuffix, listenerKey string, webhookSourceCount int) string {
	if pathSuffix != "" {
		return configName + "/" + pathSuffix
	}
	if webhookSourceCount <= 1 {
		return configName
	}
	const prefix = "Webhook-"
	suffix := listenerKey
	if strings.HasPrefix(listenerKey, prefix) {
		suffix = listenerKey[len(prefix):]
	}
	return configName + "/" + suffix
}

// NeedLeaderElection implements manager.LeaderElectionRunnable so the shared
// webhook server only listens on the elected leader when leader election is enabled.
func (s *WebhookServer) NeedLeaderElection() bool {
	return true
}

// Start implements manager.Runnable: listens until ctx is cancelled, then shuts down gracefully.
func (s *WebhookServer) Start(ctx context.Context) error {
	s.logger.Info("Starting shared webhook server", "addr", s.addr)

	ln, err := net.Listen("tcp", s.addr)
	if err != nil {
		return err
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- s.server.Serve(ln)
	}()

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.closeHTTPServer(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

func (s *WebhookServer) closeHTTPServer(ctx context.Context) error {
	var shutdownErr error
	s.httpShutdownMu.Do(func() {
		shutdownErr = s.server.Shutdown(ctx)
	})
	return shutdownErr
}

// Register adds or replaces the route for the given route key.
func (s *WebhookServer) Register(routeKey string, routeCtx context.Context, cfg *v1alpha1.WebhookConfig, k8sClient client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.routes[routeKey]; ok {
		existing.shutdown()
	}

	r := newRoute(routeCtx, routeKey, cfg, k8sClient, eventChan, logger)
	s.routes[routeKey] = r

	s.logger.Info("Registered webhook route", "routeKey", routeKey, "path", webhookRoutePrefix+routeKey)
}

// Unregister removes the route for the given route key.
func (s *WebhookServer) Unregister(routeKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.routes[routeKey]; ok {
		r.shutdown()
		delete(s.routes, routeKey)
		s.logger.Info("Unregistered webhook route", "routeKey", routeKey)
	}
}

// HasRoute reports whether a route exists for routeKey.
func (s *WebhookServer) HasRoute(routeKey string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.routes[routeKey]
	return ok
}

func recoverMiddleware(next http.HandlerFunc, logger logr.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				logger.Error(fmt.Errorf("panic: %v", rec), "Recovered from panic in webhook handler")
				w.WriteHeader(http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}
