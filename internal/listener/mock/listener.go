package mock

import (
	"fmt"
	"sync"
	"time"

	"github.com/external-secrets/reloader/internal/events"
)

// MockNotificationListener is a mock implementation of a notification listener for secret rotation events.
type MockNotificationListener struct {
	events       []events.SecretRotationEvent
	emitInterval time.Duration
	mu           sync.Mutex
	stopped      bool
	eventChan    chan events.SecretRotationEvent
}

// Start initiates the emission of events from the MockNotificationListener. Returns an error if the listener has been stopped.
func (m *MockNotificationListener) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return fmt.Errorf("listener has been stopped")
	}

	go func() {
		for _, event := range m.events {
			time.Sleep(m.emitInterval)
			m.eventChan <- event
		}
	}()

	return nil
}

// Stop signals the MockNotificationListener to stop emitting events.
func (m *MockNotificationListener) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
	return nil
}

// NewMockListener creates a new MockNotificationListener with specified events, emit interval, and event channel.
func NewMockListener(events []events.SecretRotationEvent, emitInterval time.Duration, eventChan chan events.SecretRotationEvent) *MockNotificationListener {
	return &MockNotificationListener{
		events:       events,
		emitInterval: emitInterval,
		eventChan:    eventChan,
	}
}
