package core

import (
	"log/slog"
	"sync"
	"time"
)

// EventType represents the type of system event.
type EventType string

const (
	// File events
	EventFileCreated  EventType = "file.created"
	EventFileModified EventType = "file.modified"
	EventFileDeleted  EventType = "file.deleted"
	EventFileRenamed  EventType = "file.renamed"
	EventFileMoved    EventType = "file.moved"

	// Tracking events
	EventFileTracked   EventType = "file.tracked"
	EventFileUntracked EventType = "file.untracked"

	// Version events
	EventSnapshotCreated  EventType = "snapshot.created"
	EventSnapshotRestored EventType = "snapshot.restored"

	// Sync events
	EventSyncStarted   EventType = "sync.started"
	EventSyncCompleted EventType = "sync.completed"
	EventSyncFailed    EventType = "sync.failed"
	EventSyncConflict  EventType = "sync.conflict"

	// System events
	EventEngineStarted  EventType = "engine.started"
	EventEngineStopped  EventType = "engine.stopped"
	EventDeviceDetected EventType = "device.detected"
	EventDeviceLost     EventType = "device.lost"

	// Encryption events
	EventFileEncrypted EventType = "file.encrypted"
	EventFileDecrypted EventType = "file.decrypted"
)

// Event represents a system event.
type Event struct {
	// Type is the event type.
	Type EventType `json:"type"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Source is the origin of the event (e.g., device ID, component name).
	Source string `json:"source"`
	// Data contains event-specific payload.
	Data map[string]interface{} `json:"data,omitempty"`
}

// NewEvent creates a new event with the current timestamp.
func NewEvent(eventType EventType, source string, data map[string]interface{}) Event {
	return Event{
		Type:      eventType,
		Timestamp: time.Now(),
		Source:    source,
		Data:      data,
	}
}

// EventHandler is a function that processes events.
type EventHandler func(Event)

// EventBus is a publish-subscribe event system for decoupled communication.
type EventBus struct {
	handlers map[EventType][]EventHandler
	mu       sync.RWMutex
	logger   *slog.Logger
	history  []Event
	histMu   sync.RWMutex
	maxHist  int
}

// NewEventBus creates a new event bus.
func NewEventBus(logger *slog.Logger) *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
		logger:   logger,
		history:  make([]Event, 0, 1000),
		maxHist:  1000,
	}
}

// Subscribe registers a handler for a specific event type.
func (eb *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.handlers[eventType] = append(eb.handlers[eventType], handler)
	eb.logger.Debug("event handler registered", "event_type", eventType)
}

// SubscribeAll registers a handler for all event types.
func (eb *EventBus) SubscribeAll(handler EventHandler) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	// Use empty string as wildcard key
	eb.handlers["*"] = append(eb.handlers["*"], handler)
	eb.logger.Debug("wildcard event handler registered")
}

// Publish sends an event to all registered handlers.
func (eb *EventBus) Publish(event Event) {
	eb.mu.RLock()
	handlers := make([]EventHandler, 0)

	// Get specific handlers
	if h, ok := eb.handlers[event.Type]; ok {
		handlers = append(handlers, h...)
	}

	// Get wildcard handlers
	if h, ok := eb.handlers["*"]; ok {
		handlers = append(handlers, h...)
	}
	eb.mu.RUnlock()

	// Store in history
	eb.histMu.Lock()
	if len(eb.history) >= eb.maxHist {
		eb.history = eb.history[1:]
	}
	eb.history = append(eb.history, event)
	eb.histMu.Unlock()

	eb.logger.Debug("publishing event",
		"type", event.Type,
		"source", event.Source,
		"handler_count", len(handlers),
	)

	// Execute handlers asynchronously
	for _, handler := range handlers {
		go func(h EventHandler) {
			defer func() {
				if r := recover(); r != nil {
					eb.logger.Error("event handler panicked",
						"type", event.Type,
						"error", r,
					)
				}
			}()
			h(event)
		}(handler)
	}
}

// History returns the event history.
func (eb *EventBus) History(limit int) []Event {
	eb.histMu.RLock()
	defer eb.histMu.RUnlock()

	if limit <= 0 || limit > len(eb.history) {
		limit = len(eb.history)
	}

	start := len(eb.history) - limit
	result := make([]Event, limit)
	copy(result, eb.history[start:])
	return result
}
