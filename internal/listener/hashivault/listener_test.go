package hashivault

import (
	"context"
	"testing"

	"github.com/external-secrets/reloader/internal/events"
	"github.com/go-logr/logr"
)

func TestProcessKVCreateEvent(t *testing.T) {
	eventChan := make(chan events.SecretRotationEvent, 1)
	listener := &HashicorpVault{
		context:   context.Background(),
		eventChan: eventChan,
		logger:    logr.Discard(),
	}
	listener.processFn([]byte(`{
		"type": "response",
		"request": {"operation": "create", "path": "secret/data/apps/example/api-key"},
		"response": {"mount_type": "kv", "mount_point": "secret/"}
	}`))

	select {
	case event := <-eventChan:
		if event.SecretIdentifier != "apps/example/api-key" {
			t.Errorf("unexpected secret identifier: %q", event.SecretIdentifier)
		}
	default:
		t.Fatal("create operation did not emit a rotation event")
	}
}
