package webhook

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const webhookRoutePrefix = "/webhook/"

// WebhookServer manages a single shared HTTP server for all webhook Config CRs.
// Each Config gets a route at POST /webhook/{config-name}.
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
		Handler: s.buildMux(),
	}
	return s
}

// Start implements manager.Runnable: listens until ctx is cancelled, then shuts down gracefully.
func (s *WebhookServer) Start(ctx context.Context) error {
	s.logger.Info("Starting shared webhook server", "addr", s.addr)
	errCh := make(chan error, 1)
	go func() {
		err := s.server.ListenAndServe()
		errCh <- err
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
		_ = s.closeHTTPServer(shutdownCtx)
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

// Register adds or replaces the route for the given Config name.
func (s *WebhookServer) Register(configName string, routeCtx context.Context, cfg *v1alpha1.WebhookConfig, k8sClient client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.routes[configName]; ok {
		existing.shutdown()
	}

	r := newRoute(routeCtx, configName, cfg, k8sClient, eventChan, logger)
	s.routes[configName] = r
	s.server.Handler = s.buildMux()

	s.logger.Info("Registered webhook route", "config", configName, "path", webhookRoutePrefix+configName)
}

// Unregister removes the route for the given Config name.
func (s *WebhookServer) Unregister(configName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.routes[configName]; ok {
		r.shutdown()
		delete(s.routes, configName)
		s.server.Handler = s.buildMux()
		s.logger.Info("Unregistered webhook route", "config", configName)
	}
}

// HasRoute reports whether a route exists for configName.
func (s *WebhookServer) HasRoute(configName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.routes[configName]
	return ok
}

// buildMux rebuilds the mux from current routes. Caller must hold s.mu (write lock).
func (s *WebhookServer) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	for name, r := range s.routes {
		pattern := fmt.Sprintf("POST %s%s", webhookRoutePrefix, name)
		h := r
		mux.HandleFunc(pattern, recoverMiddleware(h.handle, s.logger))
	}
	return mux
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
