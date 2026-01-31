package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// JSONMap is a type alias for map[string]any used for JSON payloads.
type JSONMap = map[string]any

// =============================================================================
// Frame-Compatible Queue Configuration
// =============================================================================

// QueueConfig defines configuration for a queue using Frame primitives.
// Queue URIs support multiple backends: mem://, nats://, kafka://
type QueueConfig struct {
	// Name is the queue/topic name used for registration.
	Name string `json:"name"`

	// URI is the queue connection URI.
	// Examples:
	//   - mem://feature.events (in-memory for testing)
	//   - nats://feature.events (NATS JetStream)
	//   - kafka://feature.events (Kafka/Redpanda)
	URI string `json:"uri"`

	// RetentionDuration is how long to retain messages.
	RetentionDuration time.Duration `json:"retention_duration"`

	// Description describes the queue purpose.
	Description string `json:"description,omitempty"`
}

// DefaultQueueConfigs returns the default queue configurations for the feature service.
func DefaultQueueConfigs() []QueueConfig {
	return []QueueConfig{
		// Main event queue - partitioned by execution ID
		{
			Name:              "feature.events",
			URI:               "mem://feature.events",
			RetentionDuration: 7 * 24 * time.Hour,
			Description:       "Main event queue for feature execution events",
		},
		// DLQ for failed events
		{
			Name:              "feature.events.dlq",
			URI:               "mem://feature.events.dlq",
			RetentionDuration: 28 * 24 * time.Hour,
			Description:       "Dead-letter queue for failed events",
		},
		// Retry queues with exponential backoff
		{
			Name:              "feature.events.retry.1",
			URI:               "mem://feature.events.retry.1",
			RetentionDuration: 1 * time.Minute,
			Description:       "Retry queue level 1 (~1 minute delay)",
		},
		{
			Name:              "feature.events.retry.2",
			URI:               "mem://feature.events.retry.2",
			RetentionDuration: 5 * time.Minute,
			Description:       "Retry queue level 2 (~5 minute delay)",
		},
		{
			Name:              "feature.events.retry.3",
			URI:               "mem://feature.events.retry.3",
			RetentionDuration: 30 * time.Minute,
			Description:       "Retry queue level 3 (~30 minute delay)",
		},
		// Feature request queue (external input)
		{
			Name:              "feature.requests",
			URI:               "mem://feature.requests",
			RetentionDuration: 24 * time.Hour,
			Description:       "Incoming feature requests",
		},
		// Feature result queue (external output)
		{
			Name:              "feature.results",
			URI:               "mem://feature.results",
			RetentionDuration: 7 * 24 * time.Hour,
			Description:       "Completed feature results",
		},
	}
}

// =============================================================================
// Frame Event Handler Interface
// =============================================================================

// FrameEventHandler defines the interface for Frame-compatible event handlers.
// This matches the pattern used in service-profile.
type FrameEventHandler interface {
	// Name returns the unique event identifier.
	Name() string

	// PayloadType returns a pointer to the expected payload type.
	PayloadType() any

	// Validate validates the payload before execution (optional, can return nil).
	Validate(ctx context.Context, payload any) error

	// Execute processes the event.
	Execute(ctx context.Context, payload any) error
}

// BaseEventHandler provides common functionality for event handlers.
type BaseEventHandler struct {
	name        string
	payloadType any
}

// NewBaseEventHandler creates a new base event handler.
func NewBaseEventHandler(name string, payloadType any) *BaseEventHandler {
	return &BaseEventHandler{
		name:        name,
		payloadType: payloadType,
	}
}

// Name returns the event handler name.
func (h *BaseEventHandler) Name() string {
	return h.name
}

// PayloadType returns the payload type.
func (h *BaseEventHandler) PayloadType() any {
	return h.payloadType
}

// =============================================================================
// Frame Queue Subscriber Interface
// =============================================================================

// FrameQueueHandler defines the interface for Frame queue subscribers.
// Implements queue.SubscribeWorker from Frame.
type FrameQueueHandler interface {
	// Handle processes an incoming queue message.
	Handle(ctx context.Context, headers map[string]string, payload []byte) error
}

// =============================================================================
// Event Message Conversion
// =============================================================================

// EventMessage represents a message for queue publishing.
type EventMessage struct {
	// Key is the partition key (execution ID).
	Key string `json:"key"`

	// Payload is the event payload.
	Payload any `json:"payload"`

	// Headers are message headers.
	Headers map[string]string `json:"headers,omitempty"`

	// Timestamp is the message timestamp.
	Timestamp time.Time `json:"timestamp,omitempty"`
}

// EventToQueuePayload converts an event to a Frame queue payload.
func EventToQueuePayload(event *Event) (JSONMap, map[string]string, error) {
	payload := JSONMap{
		"event_id":        event.EventID.String(),
		"event_type":      event.EventType.String(),
		"execution_id":    event.FeatureExecutionID.String(),
		"sequence_number": event.SequenceNumber,
		"created_at":      event.CreatedAt.Format(time.RFC3339Nano),
		"payload":         event.Payload,
		"metadata":        event.Metadata,
	}

	headers := map[string]string{
		"event_type":     event.EventType.String(),
		"event_id":       event.EventID.String(),
		"execution_id":   event.FeatureExecutionID.String(),
		"sequence":       fmt.Sprintf("%d", event.SequenceNumber),
		"schema_version": event.SchemaVersion,
	}

	if event.Metadata.TraceContext != nil {
		headers["traceparent"] = fmt.Sprintf("00-%s-%s-%s",
			event.Metadata.TraceContext.TraceID,
			event.Metadata.TraceContext.SpanID,
			event.Metadata.TraceContext.TraceFlags)
	}

	return payload, headers, nil
}

// QueuePayloadToEvent converts a Frame queue payload back to an event.
func QueuePayloadToEvent(payload []byte) (*Event, error) {
	var event Event
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("unmarshal event: %w", err)
	}
	return &event, nil
}

// =============================================================================
// Event Emitter for Internal Events
// =============================================================================

// EventEmitter wraps Frame's EventsManager for type-safe event emission.
type EventEmitter struct {
	// eventsManager is injected from Frame's svc.EventsManager()
	emitFunc func(ctx context.Context, name string, payload any) error
}

// NewEventEmitter creates a new event emitter.
// Usage: emitter := NewEventEmitter(svc.EventsManager().Emit)
func NewEventEmitter(emitFunc func(ctx context.Context, name string, payload any) error) *EventEmitter {
	return &EventEmitter{emitFunc: emitFunc}
}

// Emit emits an internal event.
func (e *EventEmitter) Emit(ctx context.Context, eventName string, payload any) error {
	return e.emitFunc(ctx, eventName, payload)
}

// EmitWithType emits a typed event.
func (e *EventEmitter) EmitWithType(ctx context.Context, eventType EventType, payload any) error {
	return e.Emit(ctx, eventType.String(), payload)
}

// =============================================================================
// Queue Publisher for External Events
// =============================================================================

// QueuePublisher wraps Frame's QueueManager for type-safe queue publishing.
type QueuePublisher struct {
	// publishFunc is injected from Frame's svc.QueueManager().Publish
	publishFunc func(ctx context.Context, queueName string, payload any, headers map[string]string) error
}

// NewQueuePublisher creates a new queue publisher.
// Usage: publisher := NewQueuePublisher(svc.QueueManager().Publish)
func NewQueuePublisher(publishFunc func(ctx context.Context, queueName string, payload any, headers map[string]string) error) *QueuePublisher {
	return &QueuePublisher{publishFunc: publishFunc}
}

// Publish publishes a message to a queue.
func (p *QueuePublisher) Publish(ctx context.Context, queueName string, event *Event) error {
	payload, headers, err := EventToQueuePayload(event)
	if err != nil {
		return fmt.Errorf("convert event to payload: %w", err)
	}
	return p.publishFunc(ctx, queueName, payload, headers)
}

// PublishResult publishes a feature result to the results queue.
func (p *QueuePublisher) PublishResult(ctx context.Context, queueName string, result *FeatureResult) error {
	payload := JSONMap{
		"execution_id": result.ExecutionID.String(),
		"status":       result.Status,
		"result":       result.Result,
		"completed_at": result.CompletedAt.Format(time.RFC3339Nano),
	}

	headers := map[string]string{
		"execution_id": result.ExecutionID.String(),
		"status":       string(result.Status),
	}

	return p.publishFunc(ctx, queueName, payload, headers)
}

// FeatureResult represents the final result of a feature execution.
type FeatureResult struct {
	ExecutionID ExecutionID       `json:"execution_id"`
	Status      FeatureStatus     `json:"status"`
	Result      map[string]any    `json:"result,omitempty"`
	CompletedAt time.Time         `json:"completed_at"`
	Error       *FeatureErrorInfo `json:"error,omitempty"`
}

// FeatureStatus represents the status of a feature execution.
type FeatureStatus string

const (
	FeatureStatusSuccess FeatureStatus = "success"
	FeatureStatusFailed  FeatureStatus = "failed"
	FeatureStatusPartial FeatureStatus = "partial"
)

// FeatureErrorInfo contains error information.
type FeatureErrorInfo struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// =============================================================================
// Partition Key Helpers
// =============================================================================

// PartitionKey generates a partition key for an execution.
// Used to ensure all events for an execution go to the same partition.
func PartitionKey(executionID ExecutionID) string {
	return executionID.String()
}

// murmur2Hash implements murmur2 hash for partition assignment.
// This is used for consistent partition routing when needed.
func murmur2Hash(data []byte) uint32 {
	const (
		seed = 0x9747b28c
		m    = 0x5bd1e995
		r    = 24
	)

	length := len(data)
	h := seed ^ uint32(length)

	for len(data) >= 4 {
		k := uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24
		k *= m
		k ^= k >> r
		k *= m
		h *= m
		h ^= k
		data = data[4:]
	}

	switch len(data) {
	case 3:
		h ^= uint32(data[2]) << 16
		fallthrough
	case 2:
		h ^= uint32(data[1]) << 8
		fallthrough
	case 1:
		h ^= uint32(data[0])
		h *= m
	}

	h ^= h >> 13
	h *= m
	h ^= h >> 15

	return h
}

// TopicPartitionKey creates a partition key for routing.
func TopicPartitionKey(executionID ExecutionID, numPartitions int) int32 {
	key := executionID.String()
	hash := murmur2Hash([]byte(key))
	return int32(hash % uint32(numPartitions))
}
