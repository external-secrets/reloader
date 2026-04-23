package listener

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	esov1alpha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/listener/schema"
	"github.com/external-secrets/reloader/internal/listener/webhook"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Manager manages event listeners for secret rotation events. It coordinates the creation, starting, and stopping of listeners.
type Manager struct {
	context       context.Context
	client        client.Client
	eventChan     chan events.SecretRotationEvent
	listeners     map[types.NamespacedName]map[string]schema.Listener
	mu            sync.Mutex
	logger        logr.Logger
	webhookServer *webhook.WebhookServer
}

func NewListenerManager(ctx context.Context, eventChan chan events.SecretRotationEvent, client client.Client, logger logr.Logger, hook *webhook.WebhookServer) *Manager {
	return &Manager{
		context:       ctx,
		eventChan:     eventChan,
		client:        client,
		listeners:     make(map[types.NamespacedName]map[string]schema.Listener),
		logger:        logger,
		webhookServer: hook,
	}
}

// ManageListeners manages the active listeners based on the provided notification sources. It starts new listeners and stops unwanted ones.
func (lm *Manager) ManageListeners(manifestName types.NamespacedName, sources []esov1alpha1.NotificationSource) error {
	lm.mu.Lock()
	// Register listener for that manifest if we haven't
	if _, ok := lm.listeners[manifestName]; !ok {
		lm.listeners[manifestName] = make(map[string]schema.Listener)
	}
	// Clean up desired listeners for manifest
	desiredListeners := map[string]esov1alpha1.NotificationSource{}
	defer lm.mu.Unlock()
	for _, source := range sources {
		key, err := generateListenerKey(source)
		if err != nil {
			lm.logger.Error(err, "failed to generate listener key", "source", source)
			continue
		}
		desiredListeners[key] = source
	}

	// Remove unwanted listeners
	for key, l := range lm.listeners[manifestName] {
		if _, exists := desiredListeners[key]; !exists {
			lm.logger.Info("Stopping listener", "key", key)
			if err := l.Stop(); err != nil {
				lm.logger.Error(err, "failed to stop listener", "key", key)
			}
			delete(lm.listeners[manifestName], key)
			lm.logger.V(1).Info("removing listener entry", "manifest", manifestName, "key", key)
		}
	}

	// Add new listeners
	for key, source := range desiredListeners {
		if _, exists := lm.listeners[manifestName][key]; exists {
			if source.Type == schema.WEBHOOK && lm.webhookServer != nil && source.Webhook != nil {
				lm.webhookServer.Register(manifestName.Name, lm.context, source.Webhook, lm.client, lm.eventChan, lm.logger)
				lm.logger.V(1).Info("updated webhook registration", "manifest", manifestName, "key", key)
			} else {
				lm.logger.V(1).Info("listener already exists", "key", key)
			}
			continue
		}
		lm.logger.Info("Creating new eventListener", "key", key, "type", source.Type)
		if source.Type == schema.WEBHOOK {
			if lm.webhookServer == nil {
				lm.logger.Error(nil, "webhook server is not configured; cannot start Webhook listener")
				continue
			}
			if source.Webhook == nil {
				lm.logger.Error(nil, "webhook config is nil")
				continue
			}
			eventListener := webhook.NewWebhookListener(
				lm.webhookServer,
				manifestName.Name,
				lm.context,
				source.Webhook,
				lm.client,
				lm.eventChan,
				lm.logger,
			)
			if err := eventListener.Start(); err != nil {
				lm.logger.Error(err, "failed to start listener", "key", key)
				continue
			}
			lm.listeners[manifestName][key] = eventListener
			continue
		}

		prov := schema.GetProvider(source.Type)
		if prov == nil {
			lm.logger.Error(nil, "failed to get provider", "type", source.Type)
			continue
		}
		eventListener, err := prov.CreateListener(lm.context, &source, lm.client, lm.eventChan, lm.logger)
		if err != nil {
			lm.logger.Error(err, "failed to create listener", "key", key)
			continue
		}
		if err := eventListener.Start(); err != nil {
			lm.logger.Error(err, "failed to start listener", "key", key)
			continue
		}
		lm.listeners[manifestName][key] = eventListener
	}
	// cleanup if empty
	if len(lm.listeners[manifestName]) == 0 {
		lm.logger.V(1).Info("removing listener map for manifest", "manifest", manifestName)
		delete(lm.listeners, manifestName)
	}
	return nil
}

// StopAll stops all active listeners managed by the Manager and removes them from the listeners map.
func (lm *Manager) StopAll() error {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	var errs []error
	for mk, mv := range lm.listeners {
		for key, l := range mv {

			lm.logger.Info("Stopping listener", "key", key)
			if err := l.Stop(); err != nil {
				lm.logger.Error(err, "failed to stop listener", "key", key)
				errs = append(errs, err)
			}
			delete(lm.listeners[mk], key)
		}
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// generateListenerKey creates a unique key for a NotificationSource based on its Type and configuration.
func generateListenerKey(source esov1alpha1.NotificationSource) (string, error) {
	if source.Type == schema.WEBHOOK {
		return schema.WEBHOOK, nil
	}

	// Marshal the specific configuration based on the Type
	var config any
	switch source.Type {
	case schema.AWS_SQS:
		config = source.AwsSqs
	case schema.AZURE_EVENT_GRID:
		config = source.AzureEventGrid
	case schema.GOOGLE_PUB_SUB:
		config = source.GooglePubSub
	case schema.HASHICORP_VAULT:
		config = source.HashicorpVault
	case schema.TCP_SOCKET:
		config = source.TCPSocket
	case schema.MOCK:
		config = source.Mock
	case schema.KUBERNETES_SECRET:
		config = source.KubernetesSecret
	case schema.KUBERNETES_CONFIG_MAP:
		config = source.KubernetesConfigMap
	default:
		return "", fmt.Errorf("unsupported notification source type: %s", source.Type)
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	// Compute SHA1 hash of the configuration for uniqueness
	hash := sha1.Sum(data)

	// Combine Type and hash to form the key
	key := fmt.Sprintf("%s-%x", source.Type, hash)

	return key, nil
}
