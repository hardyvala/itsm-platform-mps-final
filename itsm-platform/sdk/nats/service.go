package nats

import (
	"context"
	"fmt"
	"log"
	"strings"

	"itsm-platform/sdk/dsl"

	"github.com/nats-io/nats.go"
)

// ServiceManager handles all NATS operations for a service
type ServiceManager struct {
	nc           *nats.Conn
	eventManager *EventManager
	graph        *dsl.ServiceGraph
	handlers     map[string]func(*nats.Msg)
}

// NewServiceManager creates a complete NATS setup for a service
func NewServiceManager(natsURL string, graph *dsl.ServiceGraph) (*ServiceManager, error) {
	// Connect to NATS
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS: %w", err)
	}

	// Create event manager for JetStream
	em, err := NewEventManager(nc, graph)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create event manager: %w", err)
	}

	sm := &ServiceManager{
		nc:           nc,
		eventManager: em,
		graph:        graph,
		handlers:     make(map[string]func(*nats.Msg)),
	}

	return sm, nil
}

// RegisterEntityHandlers registers CRUD handlers for a specific entity
func (sm *ServiceManager) RegisterEntityHandlers(entityName string, createHandler, updateHandler, deleteHandler, getHandler, queryHandler func(*nats.Msg)) error {
	serviceName := sm.graph.Metadata.Service

	// Register CRUD operations
	sm.registerHandler(fmt.Sprintf("%s.*.%s.create", serviceName, entityName), createHandler)
	sm.registerHandler(fmt.Sprintf("%s.*.%s.update", serviceName, entityName), updateHandler)
	sm.registerHandler(fmt.Sprintf("%s.*.%s.delete", serviceName, entityName), deleteHandler)
	sm.registerHandler(fmt.Sprintf("%s.*.%s.get", serviceName, entityName), getHandler)
	sm.registerHandler(fmt.Sprintf("%s.*.%s.query", serviceName, entityName), queryHandler)

	return nil
}

// RegisterEventHandlers registers handlers for subscribed events
func (sm *ServiceManager) RegisterEventHandlers(eventHandlers map[string]func(*nats.Msg)) error {
	for pattern, handler := range eventHandlers {
		sm.registerHandler(pattern, handler)
	}
	return nil
}

// registerHandler subscribes to a subject with a handler
func (sm *ServiceManager) registerHandler(subject string, handler func(*nats.Msg)) {
	sm.handlers[subject] = handler
}

// StartService initializes all subscriptions
func (sm *ServiceManager) StartService(ctx context.Context) error {
	// Subscribe to all registered handlers
	for subject, handler := range sm.handlers {
		if _, err := sm.nc.Subscribe(subject, handler); err != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
		}
		log.Printf("Subscribed to: %s", subject)
	}

	// Start event subscribers for cross-service events
	if err := sm.eventManager.StartSubscribers(ctx); err != nil {
		return fmt.Errorf("failed to start event subscribers: %w", err)
	}

	return nil
}

// PublishEvent publishes an event using the configured patterns
func (sm *ServiceManager) PublishEvent(ctx context.Context, eventName, tenantID string, data map[string]interface{}) error {
	// Find the publish config for this event
	for _, pub := range sm.graph.Events.Publish {
		if pub.Event == eventName {
			// Replace template variables in subject
			subject := pub.Subject
			subject = replacePlaceholder(subject, "{tenant_id}", tenantID)

			// Publish through event manager
			return sm.eventManager.Publish(ctx, subject, data)
		}
	}

	return fmt.Errorf("no publish configuration found for event: %s", eventName)
}

// GetConnection returns the NATS connection for custom operations
func (sm *ServiceManager) GetConnection() *nats.Conn {
	return sm.nc
}

// Close cleanly shuts down the service manager
func (sm *ServiceManager) Close() {
	if sm.eventManager != nil {
		sm.eventManager.Close()
	}
	if sm.nc != nil {
		sm.nc.Close()
	}
}

// replacePlaceholder replaces template variables in subject patterns
func replacePlaceholder(template, placeholder, value string) string {
	return strings.ReplaceAll(template, placeholder, value)
}
