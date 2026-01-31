package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

// Event is the canonical envelope for all system events.
// Every event in the system conforms to this structure.
type Event struct {
	// === IDENTITY ===

	// EventID is a globally unique event identifier (XID - time-ordered).
	EventID EventID `json:"event_id"`

	// FeatureExecutionID is the feature execution this event belongs to (partition key).
	FeatureExecutionID ExecutionID `json:"feature_execution_id"`

	// EventType is the event type identifier (e.g., "feature.execution.initialized").
	EventType EventType `json:"event_type"`

	// SchemaVersion is the semantic version of the payload schema.
	SchemaVersion string `json:"schema_version"`

	// === ORDERING ===

	// SequenceNumber is a monotonically increasing number within the feature execution.
	// Starts at 1, increments by 1 for each event.
	SequenceNumber uint64 `json:"sequence_number"`

	// HLCTimestamp is a hybrid logical clock timestamp for cross-partition ordering.
	HLCTimestamp HybridTimestamp `json:"hlc_timestamp"`

	// CreatedAt is the wall clock timestamp when the event was created.
	CreatedAt time.Time `json:"created_at"`

	// === CAUSALITY ===

	// CausationID is the event ID that directly caused this event (parent in causal chain).
	CausationID EventID `json:"causation_id,omitempty"`

	// CorrelationID is the root event ID that started the feature execution.
	// This is constant for all events in an execution.
	CorrelationID EventID `json:"correlation_id"`

	// OriginalEventID is set for events triggered by retries.
	OriginalEventID EventID `json:"original_event_id,omitempty"`

	// === PAYLOAD ===

	// Payload is the JSON-encoded event payload. Type determined by EventType.
	Payload json.RawMessage `json:"payload"`

	// PayloadChecksum is the SHA-256 checksum of the serialized payload for integrity.
	PayloadChecksum string `json:"payload_checksum"`

	// === METADATA ===

	// Metadata contains producer and processing information.
	Metadata EventMetadata `json:"metadata"`

	// === PROCESSING HINTS ===

	// Hints guide consumer behavior for this event.
	Hints ProcessingHints `json:"hints"`
}

// HybridTimestamp combines physical and logical time for ordering.
type HybridTimestamp struct {
	// PhysicalMS is the physical time in milliseconds since Unix epoch.
	PhysicalMS int64 `json:"physical_ms"`

	// Logical is a counter for events at the same physical time.
	Logical uint32 `json:"logical"`
}

// NewHybridTimestamp creates a new hybrid timestamp.
func NewHybridTimestamp() HybridTimestamp {
	return HybridTimestamp{
		PhysicalMS: time.Now().UnixMilli(),
		Logical:    0,
	}
}

// After returns true if this timestamp is after other.
func (h HybridTimestamp) After(other HybridTimestamp) bool {
	if h.PhysicalMS != other.PhysicalMS {
		return h.PhysicalMS > other.PhysicalMS
	}
	return h.Logical > other.Logical
}

// Time converts to time.Time (losing logical component).
func (h HybridTimestamp) Time() time.Time {
	return time.UnixMilli(h.PhysicalMS)
}

// EventMetadata contains producer and processing information.
type EventMetadata struct {
	// ProducerID identifies the service/worker that produced this event.
	ProducerID string `json:"producer_id"`

	// ProducerVersion is the version of the producer service.
	ProducerVersion string `json:"producer_version"`

	// TenantID provides tenant isolation.
	TenantID string `json:"tenant_id"`

	// Environment identifies the deployment environment (production, staging, development).
	Environment string `json:"environment"`

	// Tags are arbitrary key-value pairs for filtering/routing.
	Tags map[string]string `json:"tags,omitempty"`

	// TraceContext provides distributed tracing context (W3C Trace Context format).
	TraceContext *TraceContext `json:"trace_context,omitempty"`
}

// TraceContext holds W3C Trace Context information.
type TraceContext struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	TraceFlags string `json:"trace_flags"`
	TraceState string `json:"trace_state,omitempty"`
}

// ProcessingHints guide consumer behavior for event processing.
type ProcessingHints struct {
	// Priority level (higher = process first).
	Priority Priority `json:"priority"`

	// MaxRetries is the maximum retry attempts before dead-letter.
	MaxRetries int `json:"max_retries"`

	// RetryBaseDelayMS is the base delay for exponential backoff (milliseconds).
	RetryBaseDelayMS int `json:"retry_base_delay_ms"`

	// RetryMaxDelayMS is the maximum delay cap (milliseconds).
	RetryMaxDelayMS int `json:"retry_max_delay_ms"`

	// ProcessingTimeoutMS is the timeout for processing this event (milliseconds).
	ProcessingTimeoutMS int `json:"processing_timeout_ms"`

	// AllowSkipOnFailure indicates if failure should not block subsequent events.
	AllowSkipOnFailure bool `json:"allow_skip_on_failure"`
}

// DefaultProcessingHints returns sensible defaults.
func DefaultProcessingHints() ProcessingHints {
	return ProcessingHints{
		Priority:            PriorityNormal,
		MaxRetries:          5,
		RetryBaseDelayMS:    1000,
		RetryMaxDelayMS:     300000, // 5 minutes
		ProcessingTimeoutMS: 60000,  // 1 minute
		AllowSkipOnFailure:  false,
	}
}

// Priority represents event processing priority.
type Priority int

const (
	PriorityUnspecified Priority = iota
	PriorityLow
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// String returns the string representation.
func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unspecified"
	}
}

// MarshalJSON implements json.Marshaler.
func (p Priority) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *Priority) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "low":
		*p = PriorityLow
	case "normal":
		*p = PriorityNormal
	case "high":
		*p = PriorityHigh
	case "critical":
		*p = PriorityCritical
	default:
		*p = PriorityUnspecified
	}
	return nil
}

// EventBuilder provides a fluent interface for constructing events.
type EventBuilder struct {
	event Event
	err   error
}

// NewEventBuilder creates a new event builder.
func NewEventBuilder() *EventBuilder {
	return &EventBuilder{
		event: Event{
			EventID:       NewEventID(),
			SchemaVersion: "1.0.0",
			HLCTimestamp:  NewHybridTimestamp(),
			CreatedAt:     time.Now().UTC(),
			Hints:         DefaultProcessingHints(),
		},
	}
}

// WithExecutionID sets the feature execution ID.
func (b *EventBuilder) WithExecutionID(id ExecutionID) *EventBuilder {
	b.event.FeatureExecutionID = id
	return b
}

// WithEventType sets the event type.
func (b *EventBuilder) WithEventType(t EventType) *EventBuilder {
	b.event.EventType = t
	return b
}

// WithSequence sets the sequence number.
func (b *EventBuilder) WithSequence(seq uint64) *EventBuilder {
	b.event.SequenceNumber = seq
	return b
}

// WithCausation sets the causation ID (the event that caused this one).
func (b *EventBuilder) WithCausation(id EventID) *EventBuilder {
	b.event.CausationID = id
	return b
}

// WithCorrelation sets the correlation ID (root event ID).
func (b *EventBuilder) WithCorrelation(id EventID) *EventBuilder {
	b.event.CorrelationID = id
	return b
}

// WithPayload sets the event payload.
func (b *EventBuilder) WithPayload(payload any) *EventBuilder {
	if b.err != nil {
		return b
	}

	data, err := json.Marshal(payload)
	if err != nil {
		b.err = fmt.Errorf("marshal payload: %w", err)
		return b
	}

	b.event.Payload = data
	b.event.PayloadChecksum = computeChecksum(data)
	return b
}

// WithMetadata sets the event metadata.
func (b *EventBuilder) WithMetadata(meta EventMetadata) *EventBuilder {
	b.event.Metadata = meta
	return b
}

// WithHints sets processing hints.
func (b *EventBuilder) WithHints(hints ProcessingHints) *EventBuilder {
	b.event.Hints = hints
	return b
}

// WithPriority sets the event priority.
func (b *EventBuilder) WithPriority(p Priority) *EventBuilder {
	b.event.Hints.Priority = p
	return b
}

// Build constructs the final event.
func (b *EventBuilder) Build() (*Event, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Validate required fields
	if b.event.FeatureExecutionID.IsZero() {
		return nil, fmt.Errorf("feature execution ID is required")
	}
	if b.event.EventType == "" {
		return nil, fmt.Errorf("event type is required")
	}
	if b.event.SequenceNumber == 0 {
		return nil, fmt.Errorf("sequence number is required")
	}
	if b.event.CorrelationID.IsZero() {
		return nil, fmt.Errorf("correlation ID is required")
	}
	if len(b.event.Payload) == 0 {
		return nil, fmt.Errorf("payload is required")
	}

	return &b.event, nil
}

// MustBuild constructs the event, panicking on error.
func (b *EventBuilder) MustBuild() *Event {
	e, err := b.Build()
	if err != nil {
		panic(err)
	}
	return e
}

// computeChecksum computes SHA-256 checksum of data.
func computeChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyChecksum verifies the payload checksum.
func (e *Event) VerifyChecksum() bool {
	return e.PayloadChecksum == computeChecksum(e.Payload)
}

// UnmarshalPayload unmarshals the payload into the given type.
func (e *Event) UnmarshalPayload(v any) error {
	return json.Unmarshal(e.Payload, v)
}

// Key returns the partition key for this event.
func (e *Event) Key() string {
	return e.FeatureExecutionID.String()
}

// IsRetry returns true if this event is a retry of another event.
func (e *Event) IsRetry() bool {
	return !e.OriginalEventID.IsZero()
}

// RetryAttempt returns the retry attempt number from tags.
func (e *Event) RetryAttempt() int {
	if e.Metadata.Tags == nil {
		return 0
	}
	if attempt, ok := e.Metadata.Tags["retry_attempt"]; ok {
		var n int
		fmt.Sscanf(attempt, "%d", &n)
		return n
	}
	return 0
}
