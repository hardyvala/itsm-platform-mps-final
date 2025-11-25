package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"itsm-platform/sdk/dsl"
)

// EventManager handles all NATS operations based on DSL
type EventManager struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	graph    *dsl.ServiceGraph
	handlers map[string]EventHandler
}

// EventHandler processes incoming events
type EventHandler func(ctx context.Context, event Event) error

// Event represents a NATS event
type Event struct {
	Subject   string                 `json:"subject"`
	TenantID  string                 `json:"tenant_id"`
	Entity    string                 `json:"entity"`
	Action    string                 `json:"action"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
	Service   string                 `json:"service"`
}

// NewEventManager creates and initializes the event system from DSL
func NewEventManager(nc *nats.Conn, graph *dsl.ServiceGraph) (*EventManager, error) {
	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	em := &EventManager{
		nc:       nc,
		js:       js,
		graph:    graph,
		handlers: make(map[string]EventHandler),
	}

	// Initialize stream from DSL
	if err := em.initStream(); err != nil {
		return nil, err
	}

	return em, nil
}

// initStream creates the JetStream stream defined in DSL
func (em *EventManager) initStream() error {
	ctx := context.Background()

	// Build subject patterns from DSL publish subjects
	var subjects []string
	for _, pub := range em.graph.Events.Publish {
		// Convert {tenant_id} to * for stream subscription
		subject := strings.ReplaceAll(pub, "{tenant_id}", "*")
		subjects = append(subjects, subject)
	}

	// Create or update stream
	_, err := em.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      em.graph.Events.Stream,
		Subjects:  subjects,
		Retention: jetstream.LimitsPolicy,
		MaxAge:    7 * 24 * time.Hour, // 7 days
		Storage:   jetstream.FileStorage,
		Replicas:  1, // Adjust for production
	})

	if err != nil {
		return fmt.Errorf("failed to create stream %s: %w", em.graph.Events.Stream, err)
	}

	return nil
}

// Publish sends an event to NATS
func (em *EventManager) Publish(ctx context.Context, subject string, data interface{}) error {
	event := Event{
		Subject:   subject,
		Data:      data.(map[string]interface{}),
		Timestamp: time.Now().UTC(),
		Service:   em.graph.Metadata.Service,
	}

	// Extract tenant_id from subject
	parts := strings.Split(subject, ".")
	if len(parts) >= 2 {
		event.TenantID = parts[1]
	}
	if len(parts) >= 3 {
		event.Entity = parts[2]
	}
	if len(parts) >= 4 {
		event.Action = parts[3]
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	_, err = em.js.Publish(ctx, subject, payload)
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}

// RegisterHandler registers a handler for a specific subject pattern
func (em *EventManager) RegisterHandler(subject string, handler EventHandler) {
	em.handlers[subject] = handler
}

// StartSubscribers starts consuming events from DSL-defined subscriptions
func (em *EventManager) StartSubscribers(ctx context.Context) error {
	for _, sub := range em.graph.Events.Subscribe {
		if err := em.subscribe(ctx, sub); err != nil {
			return err
		}
	}
	return nil
}

// subscribe creates a consumer for a subject pattern
func (em *EventManager) subscribe(ctx context.Context, subjectPattern string) error {
	// Determine which stream to consume from based on subject
	// For cross-service events, we need to consume from other service's streams
	streamName := em.inferStreamFromSubject(subjectPattern)

	consumerName := fmt.Sprintf("%s_%s_consumer",
		em.graph.Metadata.Service,
		strings.ReplaceAll(subjectPattern, ".", "_"))

	consumer, err := em.js.CreateOrUpdateConsumer(ctx, streamName, jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: subjectPattern,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       30 * time.Second,
		MaxDeliver:    5,
	})

	if err != nil {
		return fmt.Errorf("failed to create consumer for %s: %w", subjectPattern, err)
	}

	// Start consuming in goroutine
	go em.consumeMessages(ctx, consumer, subjectPattern)

	return nil
}

// inferStreamFromSubject determines stream name from subject pattern
func (em *EventManager) inferStreamFromSubject(subject string) string {
	// Extract service name from subject (first part)
	parts := strings.Split(subject, ".")
	if len(parts) > 0 {
		serviceName := parts[0]
		return strings.ToUpper(serviceName) + "_EVENTS"
	}
	return em.graph.Events.Stream
}

// consumeMessages processes messages from a consumer
func (em *EventManager) consumeMessages(ctx context.Context, consumer jetstream.Consumer, pattern string) {
	msgs, err := consumer.Messages()
	if err != nil {
		fmt.Printf("Failed to get message iterator: %v\n", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := msgs.Next()
			if err != nil {
				continue
			}

			var event Event
			if err := json.Unmarshal(msg.Data(), &event); err != nil {
				msg.Nak()
				continue
			}

			// Find matching handler
			handler := em.findHandler(event.Subject)
			if handler != nil {
				if err := handler(ctx, event); err != nil {
					msg.Nak()
					continue
				}
			}

			msg.Ack()
		}
	}
}

// findHandler finds the best matching handler for a subject
func (em *EventManager) findHandler(subject string) EventHandler {
	// Exact match
	if handler, ok := em.handlers[subject]; ok {
		return handler
	}

	// Pattern match (simple wildcard)
	for pattern, handler := range em.handlers {
		if em.matchPattern(pattern, subject) {
			return handler
		}
	}

	return nil
}

// matchPattern does simple wildcard matching
func (em *EventManager) matchPattern(pattern, subject string) bool {
	patternParts := strings.Split(pattern, ".")
	subjectParts := strings.Split(subject, ".")

	if len(patternParts) != len(subjectParts) {
		return false
	}

	for i, p := range patternParts {
		if p != "*" && p != subjectParts[i] {
			return false
		}
	}

	return true
}

// Close cleanly shuts down the event manager
func (em *EventManager) Close() {
	em.nc.Close()
}

// EventBuilder helps construct events
type EventBuilder struct {
	service  string
	tenantID string
	entity   string
	action   string
	data     map[string]interface{}
}

func NewEventBuilder(service string) *EventBuilder {
	return &EventBuilder{
		service: service,
		data:    make(map[string]interface{}),
	}
}

func (b *EventBuilder) Tenant(id string) *EventBuilder {
	b.tenantID = id
	return b
}

func (b *EventBuilder) Entity(name string) *EventBuilder {
	b.entity = strings.ToLower(name)
	return b
}

func (b *EventBuilder) Action(action string) *EventBuilder {
	b.action = action
	return b
}

func (b *EventBuilder) Data(data map[string]interface{}) *EventBuilder {
	b.data = data
	return b
}

func (b *EventBuilder) Subject() string {
	return fmt.Sprintf("%s.%s.%s.%s", b.service, b.tenantID, b.entity, b.action)
}

func (b *EventBuilder) Build() Event {
	return Event{
		Subject:   b.Subject(),
		TenantID:  b.tenantID,
		Entity:    b.entity,
		Action:    b.action,
		Data:      b.data,
		Timestamp: time.Now().UTC(),
		Service:   b.service,
	}
}
