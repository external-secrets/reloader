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

	configName, ok := configNameFromPath(req.URL.Path)
	if !ok {
		http.NotFound(w, req)
		return
	}

	s.mu.RLock()
	route, found := s.routes[configName]
	s.mu.RUnlock()
	if !found {
		http.NotFound(w, req)
		return
	}

	recoverMiddleware(route.handle, s.logger)(w, req)
}

func configNameFromPath(path string) (string, bool) {
	if !strings.HasPrefix(path, webhookRoutePrefix) {
		return "", false
	}
	configName := strings.TrimPrefix(path, webhookRoutePrefix)
	if configName == "" || strings.Contains(configName, "/") {
		return "", false
	}
	return configName, true
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

// Register adds or replaces the route for the given Config name.
func (s *WebhookServer) Register(configName string, routeCtx context.Context, cfg *v1alpha1.WebhookConfig, k8sClient client.Client, eventChan chan events.SecretRotationEvent, logger logr.Logger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.routes[configName]; ok {
		existing.shutdown()
	}

	r := newRoute(routeCtx, configName, cfg, k8sClient, eventChan, logger)
	s.routes[configName] = r

	s.logger.Info("Registered webhook route", "config", configName, "path", webhookRoutePrefix+configName)
}

// Unregister removes the route for the given Config name.
func (s *WebhookServer) Unregister(configName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r, ok := s.routes[configName]; ok {
		r.shutdown()
		delete(s.routes, configName)
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
