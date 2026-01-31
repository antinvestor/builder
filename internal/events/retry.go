package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"
)

// Common retry errors.
var (
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
	ErrNonRetryable       = errors.New("error is not retryable")
)

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	// MaxRetries is the maximum number of retry attempts.
	MaxRetries int `json:"max_retries"`

	// InitialDelayMS is the initial delay in milliseconds.
	InitialDelayMS int `json:"initial_delay_ms"`

	// MaxDelayMS is the maximum delay in milliseconds.
	MaxDelayMS int `json:"max_delay_ms"`

	// BackoffMultiplier is the exponential backoff multiplier.
	BackoffMultiplier float64 `json:"backoff_multiplier"`

	// Jitter adds randomness to prevent thundering herd.
	Jitter float64 `json:"jitter"` // 0.0 to 1.0

	// RetryableErrors are error codes that should be retried.
	RetryableErrors []string `json:"retryable_errors,omitempty"`
}

// DefaultRetryPolicy returns the default retry policy.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:        5,
		InitialDelayMS:    1000,    // 1 second
		MaxDelayMS:        300000,  // 5 minutes
		BackoffMultiplier: 2.0,
		Jitter:            0.1,
		RetryableErrors: []string{
			"network",
			"timeout",
			"rate_limit",
			"overloaded",
			"server_error",
		},
	}
}

// CalculateDelay calculates the delay for a retry attempt.
func (p RetryPolicy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}

	delay := float64(p.InitialDelayMS) * math.Pow(p.BackoffMultiplier, float64(attempt-1))
	if delay > float64(p.MaxDelayMS) {
		delay = float64(p.MaxDelayMS)
	}

	// Add jitter
	if p.Jitter > 0 {
		jitterAmount := delay * p.Jitter
		// Simple pseudo-random based on attempt number
		jitterOffset := float64(attempt%7) / 7.0 * jitterAmount
		delay = delay - jitterAmount/2 + jitterOffset
	}

	return time.Duration(delay) * time.Millisecond
}

// ShouldRetry determines if an error should be retried.
func (p RetryPolicy) ShouldRetry(errorCode string, attempt int) bool {
	if attempt >= p.MaxRetries {
		return false
	}

	if len(p.RetryableErrors) == 0 {
		return true
	}

	for _, code := range p.RetryableErrors {
		if code == errorCode {
			return true
		}
	}
	return false
}

// RetryMetadata tracks retry state.
type RetryMetadata struct {
	// OriginalEventID is the original event ID (first attempt).
	OriginalEventID EventID `json:"original_event_id"`

	// CurrentAttempt is the current attempt number (1-based).
	CurrentAttempt int `json:"current_attempt"`

	// MaxAttempts is the maximum attempts allowed.
	MaxAttempts int `json:"max_attempts"`

	// FirstAttemptAt is when the first attempt was made.
	FirstAttemptAt time.Time `json:"first_attempt_at"`

	// LastAttemptAt is when the last attempt was made.
	LastAttemptAt time.Time `json:"last_attempt_at"`

	// NextAttemptAt is when the next attempt should be made.
	NextAttemptAt time.Time `json:"next_attempt_at,omitempty"`

	// LastErrorCode is the error code from the last attempt.
	LastErrorCode string `json:"last_error_code,omitempty"`

	// LastErrorMessage is the error message from the last attempt.
	LastErrorMessage string `json:"last_error_message,omitempty"`

	// RetryHistory contains the history of retry attempts.
	RetryHistory []RetryAttempt `json:"retry_history,omitempty"`
}

// RetryAttempt records a single retry attempt.
type RetryAttempt struct {
	Attempt      int       `json:"attempt"`
	AttemptedAt  time.Time `json:"attempted_at"`
	ErrorCode    string    `json:"error_code"`
	ErrorMessage string    `json:"error_message"`
	DurationMS   int64     `json:"duration_ms"`
}

// RetryTopicSelector selects the appropriate retry topic based on attempt.
type RetryTopicSelector struct {
	// RetryTopics maps attempt ranges to topics.
	// Key is max attempt for this topic.
	RetryTopics map[int]string
	DLQTopic    string
}

// DefaultRetryTopicSelector returns the default retry topic selector.
func DefaultRetryTopicSelector() *RetryTopicSelector {
	return &RetryTopicSelector{
		RetryTopics: map[int]string{
			2: "feature.events.retry.1", // Attempts 1-2: ~1 minute delay
			4: "feature.events.retry.2", // Attempts 3-4: ~5 minute delay
			6: "feature.events.retry.3", // Attempts 5-6: ~30 minute delay
		},
		DLQTopic: "feature.events.dlq",
	}
}

// SelectTopic selects the topic for a retry attempt.
func (s *RetryTopicSelector) SelectTopic(attempt int) string {
	for maxAttempt, topic := range s.RetryTopics {
		if attempt <= maxAttempt {
			return topic
		}
	}
	return s.DLQTopic
}

// IsDLQ returns true if the attempt should go to DLQ.
func (s *RetryTopicSelector) IsDLQ(attempt, maxRetries int) bool {
	return attempt > maxRetries
}

// DLQEntry represents an entry in the dead-letter queue.
type DLQEntry struct {
	// Event is the original event.
	Event *Event `json:"event"`

	// RetryMetadata contains retry information.
	RetryMetadata RetryMetadata `json:"retry_metadata"`

	// FailureReason describes why processing failed.
	FailureReason string `json:"failure_reason"`

	// FailureClassification categorizes the failure.
	FailureClassification DLQFailureClass `json:"failure_classification"`

	// EnteredDLQAt is when the entry was added to DLQ.
	EnteredDLQAt time.Time `json:"entered_dlq_at"`

	// ExpiresAt is when the entry should be auto-deleted.
	ExpiresAt time.Time `json:"expires_at"`

	// ManualReviewRequired indicates if manual intervention is needed.
	ManualReviewRequired bool `json:"manual_review_required"`

	// Resolution tracks resolution status.
	Resolution *DLQResolution `json:"resolution,omitempty"`
}

// DLQFailureClass categorizes DLQ failures.
type DLQFailureClass string

const (
	DLQFailureTransient  DLQFailureClass = "transient"   // Network, timeout - may succeed later
	DLQFailurePermanent  DLQFailureClass = "permanent"   // Bad data, invalid state
	DLQFailureUnknown    DLQFailureClass = "unknown"     // Unexpected error
	DLQFailureValidation DLQFailureClass = "validation"  // Schema/validation failure
	DLQFailureResource   DLQFailureClass = "resource"    // Resource exhaustion
)

// DLQResolution tracks how a DLQ entry was resolved.
type DLQResolution struct {
	Status       DLQResolutionStatus `json:"status"`
	ResolvedBy   string              `json:"resolved_by"`
	ResolvedAt   time.Time           `json:"resolved_at"`
	Notes        string              `json:"notes,omitempty"`
	RetryEventID EventID             `json:"retry_event_id,omitempty"` // If requeued
}

// DLQResolutionStatus indicates how a DLQ entry was resolved.
type DLQResolutionStatus string

const (
	DLQResolutionRequeued   DLQResolutionStatus = "requeued"   // Sent back for processing
	DLQResolutionDiscarded  DLQResolutionStatus = "discarded"  // Intentionally dropped
	DLQResolutionManualFix  DLQResolutionStatus = "manual_fix" // Fixed manually
	DLQResolutionExpired    DLQResolutionStatus = "expired"    // Auto-expired
)

// RetryHandler handles retry logic for event processing using Frame primitives.
type RetryHandler struct {
	policy        RetryPolicy
	topicSelector *RetryTopicSelector
	eventsEmitter *EventEmitter
	queuePublisher *QueuePublisher
	dlqQueueName  string
}

// RetryHandlerConfig configures the retry handler.
type RetryHandlerConfig struct {
	Policy       RetryPolicy
	DLQQueueName string
}

// DefaultRetryHandlerConfig returns the default retry handler config.
func DefaultRetryHandlerConfig() RetryHandlerConfig {
	return RetryHandlerConfig{
		Policy:       DefaultRetryPolicy(),
		DLQQueueName: "feature.events.dlq",
	}
}

// NewRetryHandler creates a new retry handler using Frame primitives.
// Usage:
//
//	emitter := events.NewEventEmitter(svc.EventsManager().Emit)
//	publisher := events.NewQueuePublisher(svc.QueueManager().Publish)
//	handler := events.NewRetryHandler(cfg, emitter, publisher)
func NewRetryHandler(cfg RetryHandlerConfig, emitter *EventEmitter, publisher *QueuePublisher) *RetryHandler {
	return &RetryHandler{
		policy:         cfg.Policy,
		topicSelector:  DefaultRetryTopicSelector(),
		eventsEmitter:  emitter,
		queuePublisher: publisher,
		dlqQueueName:   cfg.DLQQueueName,
	}
}

// HandleFailure handles a processing failure.
func (h *RetryHandler) HandleFailure(ctx context.Context, event *Event, err error, errorCode string) error {
	attempt := event.RetryAttempt() + 1

	if !h.policy.ShouldRetry(errorCode, attempt) {
		return h.sendToDLQ(ctx, event, err, errorCode, DLQFailurePermanent)
	}

	if attempt > h.policy.MaxRetries {
		return h.sendToDLQ(ctx, event, err, errorCode, DLQFailureTransient)
	}

	// Create retry event
	retryEvent, buildErr := h.createRetryEvent(event, attempt, errorCode, err.Error())
	if buildErr != nil {
		return fmt.Errorf("create retry event: %w", buildErr)
	}

	// Get the retry queue name based on attempt
	retryQueueName := h.topicSelector.SelectTopic(attempt)

	// Publish to retry queue using Frame's QueueManager
	return h.queuePublisher.Publish(ctx, retryQueueName, retryEvent)
}

func (h *RetryHandler) createRetryEvent(original *Event, attempt int, errorCode, errorMsg string) (*Event, error) {
	// Copy the original event
	var eventCopy Event
	data, err := json.Marshal(original)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &eventCopy); err != nil {
		return nil, err
	}

	// Update for retry
	eventCopy.EventID = NewEventID()
	if eventCopy.OriginalEventID.IsZero() {
		eventCopy.OriginalEventID = original.EventID
	}

	// Add retry metadata to tags
	if eventCopy.Metadata.Tags == nil {
		eventCopy.Metadata.Tags = make(map[string]string)
	}
	eventCopy.Metadata.Tags["retry_attempt"] = fmt.Sprintf("%d", attempt)
	eventCopy.Metadata.Tags["last_error_code"] = errorCode
	eventCopy.Metadata.Tags["original_event_id"] = eventCopy.OriginalEventID.String()

	// Update timestamps
	eventCopy.CreatedAt = time.Now().UTC()
	eventCopy.HLCTimestamp = NewHybridTimestamp()

	return &eventCopy, nil
}

func (h *RetryHandler) sendToDLQ(ctx context.Context, event *Event, err error, errorCode string, class DLQFailureClass) error {
	entry := &DLQEntry{
		Event: event,
		RetryMetadata: RetryMetadata{
			OriginalEventID:  event.OriginalEventID,
			CurrentAttempt:   event.RetryAttempt(),
			MaxAttempts:      h.policy.MaxRetries,
			LastAttemptAt:    time.Now(),
			LastErrorCode:    errorCode,
			LastErrorMessage: err.Error(),
		},
		FailureReason:         err.Error(),
		FailureClassification: class,
		EnteredDLQAt:          time.Now(),
		ExpiresAt:             time.Now().AddDate(0, 0, 28), // 28 day retention
		ManualReviewRequired:  class == DLQFailurePermanent || class == DLQFailureUnknown,
	}

	// Publish to DLQ using Frame's QueueManager
	return h.publishDLQEntry(ctx, entry)
}

// publishDLQEntry publishes a DLQ entry to the dead-letter queue.
func (h *RetryHandler) publishDLQEntry(ctx context.Context, entry *DLQEntry) error {
	// Serialize the entry to JSON for the payload
	entryPayload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal DLQ entry: %w", err)
	}

	// Create an event wrapper for the DLQ entry
	dlqEvent := &Event{
		EventID:            NewEventID(),
		EventType:          FeatureExecutionFailed,
		FeatureExecutionID: entry.Event.FeatureExecutionID,
		OriginalEventID:    entry.Event.EventID,
		SchemaVersion:      "1.0.0",
		CreatedAt:          time.Now().UTC(),
		HLCTimestamp:       NewHybridTimestamp(),
		Payload:            entryPayload,
		Metadata: EventMetadata{
			Tags: map[string]string{
				"dlq_reason":    entry.FailureReason,
				"dlq_class":     string(entry.FailureClassification),
				"original_type": entry.Event.EventType.String(),
			},
		},
	}

	return h.queuePublisher.Publish(ctx, h.dlqQueueName, dlqEvent)
}

// RetryScheduler schedules delayed retries.
type RetryScheduler interface {
	// ScheduleRetry schedules an event for retry after a delay.
	ScheduleRetry(ctx context.Context, event *Event, delay time.Duration) error

	// CancelRetry cancels a scheduled retry.
	CancelRetry(ctx context.Context, eventID EventID) error
}

// CircuitBreaker implements circuit breaker pattern for retries.
type CircuitBreaker struct {
	name           string
	maxFailures    int
	resetTimeout   time.Duration
	halfOpenMax    int

	failures       int
	lastFailure    time.Time
	state          CircuitState
	halfOpenCount  int
}

// CircuitState represents circuit breaker state.
type CircuitState int

const (
	CircuitClosed CircuitState = iota
	CircuitOpen
	CircuitHalfOpen
)

// NewCircuitBreaker creates a new circuit breaker.
func NewCircuitBreaker(name string, maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		name:         name,
		maxFailures:  maxFailures,
		resetTimeout: resetTimeout,
		halfOpenMax:  3,
		state:        CircuitClosed,
	}
}

// AllowRequest returns true if the request should be allowed.
func (cb *CircuitBreaker) AllowRequest() bool {
	now := time.Now()

	switch cb.state {
	case CircuitClosed:
		return true
	case CircuitOpen:
		if now.Sub(cb.lastFailure) > cb.resetTimeout {
			cb.state = CircuitHalfOpen
			cb.halfOpenCount = 0
			return true
		}
		return false
	case CircuitHalfOpen:
		cb.halfOpenCount++
		return cb.halfOpenCount <= cb.halfOpenMax
	}
	return false
}

// RecordSuccess records a successful request.
func (cb *CircuitBreaker) RecordSuccess() {
	if cb.state == CircuitHalfOpen {
		cb.state = CircuitClosed
		cb.failures = 0
	}
}

// RecordFailure records a failed request.
func (cb *CircuitBreaker) RecordFailure() {
	cb.failures++
	cb.lastFailure = time.Now()

	if cb.state == CircuitHalfOpen {
		cb.state = CircuitOpen
	} else if cb.failures >= cb.maxFailures {
		cb.state = CircuitOpen
	}
}

// State returns the current state.
func (cb *CircuitBreaker) State() CircuitState {
	return cb.state
}

// String returns state as string.
func (s CircuitState) String() string {
	switch s {
	case CircuitClosed:
		return "closed"
	case CircuitOpen:
		return "open"
	case CircuitHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}
