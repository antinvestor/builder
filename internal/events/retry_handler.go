package events

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/pitabwire/util"
)

// Retry configuration constants.
const (
	// defaultMaxAttemptsPerLevel is the default max attempts at each retry level.
	defaultMaxAttemptsPerLevel = 2

	// defaultDelayLevel1 is the delay for retry level 1.
	defaultDelayLevel1 = 1 * time.Minute

	// defaultDelayLevel2 is the delay for retry level 2.
	defaultDelayLevel2 = 5 * time.Minute

	// defaultDelayLevel3 is the delay for retry level 3.
	defaultDelayLevel3 = 30 * time.Minute

	// totalMaxAttempts is the total max attempts across all levels.
	totalMaxAttempts = 6 // 2 per level * 3 levels

	// dlqRetentionDays is the number of days to retain DLQ entries.
	dlqRetentionDays = 28

	// circuitBreakerMaxFailures is the max failures before opening circuit.
	circuitBreakerMaxFailures = 5

	// l1AttemptsMax is the max attempts at level 1.
	l1AttemptsMax = 2

	// l2AttemptsMax is the max attempts at levels 1+2.
	l2AttemptsMax = 4
)

// RetryLevel represents the retry queue level.
type RetryLevel int

const (
	RetryLevel1 RetryLevel = 1 // Fast retries (~1 minute delay)
	RetryLevel2 RetryLevel = 2 // Medium retries (~5 minute delay)
	RetryLevel3 RetryLevel = 3 // Slow retries (~30 minute delay)
)

// RetryQueueConfig configures retry queue behavior.
type RetryQueueConfig struct {
	// Level is the retry level (1, 2, or 3).
	Level RetryLevel `json:"level"`

	// QueueName is the queue to subscribe to.
	QueueName string `json:"queue_name"`

	// MaxAttemptsAtLevel is max attempts before escalating to next level.
	MaxAttemptsAtLevel int `json:"max_attempts_at_level"`

	// NextLevelQueueName is the queue to escalate to (empty for DLQ).
	NextLevelQueueName string `json:"next_level_queue_name"`

	// DLQQueueName is the dead-letter queue for final failures.
	DLQQueueName string `json:"dlq_queue_name"`

	// DelayBetweenRetries is the delay between retries at this level.
	DelayBetweenRetries time.Duration `json:"delay_between_retries"`
}

// DefaultRetryQueueConfigs returns the default retry queue configurations.
func DefaultRetryQueueConfigs() []RetryQueueConfig {
	return []RetryQueueConfig{
		{
			Level:               RetryLevel1,
			QueueName:           "feature.events.retry.1",
			MaxAttemptsAtLevel:  defaultMaxAttemptsPerLevel,
			NextLevelQueueName:  "feature.events.retry.2",
			DLQQueueName:        "feature.events.dlq",
			DelayBetweenRetries: defaultDelayLevel1,
		},
		{
			Level:               RetryLevel2,
			QueueName:           "feature.events.retry.2",
			MaxAttemptsAtLevel:  defaultMaxAttemptsPerLevel,
			NextLevelQueueName:  "feature.events.retry.3",
			DLQQueueName:        "feature.events.dlq",
			DelayBetweenRetries: defaultDelayLevel2,
		},
		{
			Level:               RetryLevel3,
			QueueName:           "feature.events.retry.3",
			MaxAttemptsAtLevel:  defaultMaxAttemptsPerLevel,
			NextLevelQueueName:  "", // Escalates to DLQ
			DLQQueueName:        "feature.events.dlq",
			DelayBetweenRetries: defaultDelayLevel3,
		},
	}
}

// RetryQueueHandler handles events from retry queues.
// It implements FrameQueueHandler for Frame integration.
type RetryQueueHandler struct {
	config          RetryQueueConfig
	eventHandler    EventHandler
	queuePublisher  *QueuePublisher
	deduplication   DeduplicationStore
	circuitBreakers map[string]*CircuitBreaker
	cbMutex         sync.RWMutex
}

// NewRetryQueueHandler creates a new retry queue handler.
func NewRetryQueueHandler(
	config RetryQueueConfig,
	eventHandler EventHandler,
	publisher *QueuePublisher,
	dedup DeduplicationStore,
) *RetryQueueHandler {
	return &RetryQueueHandler{
		config:          config,
		eventHandler:    eventHandler,
		queuePublisher:  publisher,
		deduplication:   dedup,
		circuitBreakers: make(map[string]*CircuitBreaker),
	}
}

// Handle processes an incoming retry queue message.
// Implements FrameQueueHandler interface.
func (h *RetryQueueHandler) Handle(
	ctx context.Context,
	_ map[string]string,
	payload []byte,
) error {
	log := util.Log(ctx)

	// Deserialize the event
	event, err := QueuePayloadToEvent(payload)
	if err != nil {
		log.WithError(err).Error("failed to deserialize retry event")
		// Can't process - send to DLQ with parse error
		return h.sendParseErrorToDLQ(ctx, payload, err)
	}

	log = log.WithField("event_id", event.EventID.String()).
		WithField("retry_attempt", event.RetryAttempt()).
		WithField("retry_level", h.config.Level)

	// Check if already processed (deduplication)
	if h.deduplication != nil {
		processed, checkErr := h.deduplication.IsProcessed(ctx, event.EventID)
		if checkErr != nil {
			log.WithError(checkErr).Warn("failed to check deduplication")
		} else if processed {
			log.Debug("event already processed, skipping")
			return nil
		}
	}

	// Get or create circuit breaker for the event type
	cb := h.getCircuitBreaker(event.EventType.String())
	if !cb.AllowRequest() {
		log.Warn("circuit breaker open, escalating to next level")
		return h.escalateOrDLQ(
			ctx,
			event,
			fmt.Errorf("circuit breaker open for %s", event.EventType),
		)
	}

	// Attempt to process the event
	start := time.Now()
	handleErr := h.eventHandler.Handle(ctx, event)
	duration := time.Since(start)

	if handleErr == nil {
		// Success!
		cb.RecordSuccess()
		log.Info("retry succeeded", "duration_ms", duration.Milliseconds())

		// Mark as processed
		if h.deduplication != nil {
			result := &ProcessingResult{
				EventID:     event.EventID,
				ExecutionID: event.FeatureExecutionID,
				ProcessedAt: time.Now(),
				Success:     true,
				DurationMS:  duration.Milliseconds(),
			}
			markErr := h.deduplication.MarkProcessedWithResult(
				ctx,
				event.EventID,
				event.FeatureExecutionID,
				result,
			)
			if markErr != nil {
				log.WithError(markErr).Warn("failed to mark event as processed")
			}
		}

		return nil
	}

	// Failed - record failure and decide next action
	cb.RecordFailure()
	log.WithError(handleErr).Warn("retry attempt failed")

	return h.escalateOrDLQ(ctx, event, handleErr)
}

// escalateOrDLQ either escalates to the next retry level or sends to DLQ.
func (h *RetryQueueHandler) escalateOrDLQ(ctx context.Context, event *Event, err error) error {
	log := util.Log(ctx)
	attempt := event.RetryAttempt()
	attemptAtLevel := h.attemptAtCurrentLevel(attempt)

	log = log.WithField("attempt", attempt).
		WithField("attempt_at_level", attemptAtLevel).
		WithField("max_at_level", h.config.MaxAttemptsAtLevel)

	// Check if we should escalate to next level
	if attemptAtLevel >= h.config.MaxAttemptsAtLevel {
		if h.config.NextLevelQueueName != "" {
			log.Info("escalating to next retry level")
			return h.escalateToNextLevel(ctx, event, err)
		}

		// No next level - send to DLQ
		log.Info("max retries reached, sending to DLQ")
		return h.sendToDLQ(ctx, event, err, DLQFailureTransient)
	}

	// Retry at current level
	log.Info("retrying at current level")
	return h.retryAtCurrentLevel(ctx, event, err)
}

// attemptAtCurrentLevel calculates which attempt this is at the current level.
func (h *RetryQueueHandler) attemptAtCurrentLevel(totalAttempt int) int {
	switch h.config.Level {
	case RetryLevel1:
		return totalAttempt
	case RetryLevel2:
		return totalAttempt - l1AttemptsMax // After 2 attempts at L1
	case RetryLevel3:
		return totalAttempt - l2AttemptsMax // After 2 attempts at L1 + 2 at L2
	default:
		return totalAttempt
	}
}

// retryAtCurrentLevel publishes the event back to the current retry queue.
func (h *RetryQueueHandler) retryAtCurrentLevel(
	ctx context.Context,
	event *Event,
	err error,
) error {
	retryEvent := h.createRetryEvent(event, event.RetryAttempt()+1, err)
	return h.queuePublisher.Publish(ctx, h.config.QueueName, retryEvent)
}

// escalateToNextLevel publishes the event to the next retry level queue.
func (h *RetryQueueHandler) escalateToNextLevel(
	ctx context.Context,
	event *Event,
	err error,
) error {
	retryEvent := h.createRetryEvent(event, event.RetryAttempt()+1, err)
	return h.queuePublisher.Publish(ctx, h.config.NextLevelQueueName, retryEvent)
}

// sendToDLQ sends the event to the dead-letter queue.
func (h *RetryQueueHandler) sendToDLQ(
	ctx context.Context,
	event *Event,
	err error,
	class DLQFailureClass,
) error {
	entry := &DLQEntry{
		Event: event,
		RetryMetadata: RetryMetadata{
			OriginalEventID:  event.OriginalEventID,
			CurrentAttempt:   event.RetryAttempt(),
			MaxAttempts:      totalMaxAttempts,
			LastAttemptAt:    time.Now(),
			LastErrorMessage: err.Error(),
		},
		FailureReason:         err.Error(),
		FailureClassification: class,
		EnteredDLQAt:          time.Now(),
		ExpiresAt:             time.Now().AddDate(0, 0, dlqRetentionDays),
		ManualReviewRequired:  class == DLQFailurePermanent || class == DLQFailureUnknown,
	}

	return h.publishDLQEntry(ctx, entry)
}

// sendParseErrorToDLQ sends a parse error to DLQ.
func (h *RetryQueueHandler) sendParseErrorToDLQ(
	ctx context.Context,
	payload []byte,
	err error,
) error {
	entry := &DLQEntry{
		Event: &Event{
			EventID:       NewEventID(),
			EventType:     FeatureExecutionFailed,
			SchemaVersion: "1.0.0",
			CreatedAt:     time.Now().UTC(),
			HLCTimestamp:  NewHybridTimestamp(),
			Payload:       payload,
		},
		RetryMetadata: RetryMetadata{
			CurrentAttempt:   0,
			LastAttemptAt:    time.Now(),
			LastErrorMessage: err.Error(),
			LastErrorCode:    "parse_error",
		},
		FailureReason:         fmt.Sprintf("failed to parse event: %v", err),
		FailureClassification: DLQFailureValidation,
		EnteredDLQAt:          time.Now(),
		ExpiresAt:             time.Now().AddDate(0, 0, dlqRetentionDays),
		ManualReviewRequired:  true,
	}

	return h.publishDLQEntry(ctx, entry)
}

// publishDLQEntry publishes a DLQ entry to the dead-letter queue.
func (h *RetryQueueHandler) publishDLQEntry(ctx context.Context, entry *DLQEntry) error {
	entryPayload, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal DLQ entry: %w", err)
	}

	dlqEvent := &Event{
		EventID:       NewEventID(),
		EventType:     FeatureExecutionFailed,
		SchemaVersion: "1.0.0",
		CreatedAt:     time.Now().UTC(),
		HLCTimestamp:  NewHybridTimestamp(),
		Payload:       entryPayload,
		Metadata: EventMetadata{
			Tags: map[string]string{
				"dlq_reason":    entry.FailureReason,
				"dlq_class":     string(entry.FailureClassification),
				"retry_level":   fmt.Sprintf("%d", h.config.Level),
				"original_type": entry.Event.EventType.String(),
			},
		},
	}

	if entry.Event != nil && !entry.Event.FeatureExecutionID.IsZero() {
		dlqEvent.FeatureExecutionID = entry.Event.FeatureExecutionID
		dlqEvent.OriginalEventID = entry.Event.EventID
		dlqEvent.CorrelationID = entry.Event.CorrelationID
	}

	return h.queuePublisher.Publish(ctx, h.config.DLQQueueName, dlqEvent)
}

// createRetryEvent creates a new event for retry.
func (h *RetryQueueHandler) createRetryEvent(original *Event, attempt int, lastErr error) *Event {
	// Deep copy the original event
	var eventCopy Event
	data, _ := json.Marshal(original)
	_ = json.Unmarshal(data, &eventCopy)

	// Update for retry
	eventCopy.EventID = NewEventID()
	if eventCopy.OriginalEventID.IsZero() {
		eventCopy.OriginalEventID = original.EventID
	}

	// Add retry metadata to tags
	if eventCopy.Metadata.Tags == nil {
		eventCopy.Metadata.Tags = make(map[string]string)
	}
	eventCopy.Metadata.Tags["retry_attempt"] = strconv.Itoa(attempt)
	eventCopy.Metadata.Tags["retry_level"] = strconv.Itoa(int(h.config.Level))
	eventCopy.Metadata.Tags["last_error"] = lastErr.Error()
	eventCopy.Metadata.Tags["original_event_id"] = eventCopy.OriginalEventID.String()

	// Update timestamps
	eventCopy.CreatedAt = time.Now().UTC()
	eventCopy.HLCTimestamp = NewHybridTimestamp()

	return &eventCopy
}

// getCircuitBreaker gets or creates a circuit breaker for an event type.
// Uses double-checked locking to ensure thread-safe lazy initialization.
func (h *RetryQueueHandler) getCircuitBreaker(eventType string) *CircuitBreaker {
	// First check with read lock
	h.cbMutex.RLock()
	cb, ok := h.circuitBreakers[eventType]
	h.cbMutex.RUnlock()
	if ok {
		return cb
	}

	// Acquire write lock for creation
	h.cbMutex.Lock()
	defer h.cbMutex.Unlock()

	// Double-check in case another goroutine created it while we were waiting
	if cb, ok := h.circuitBreakers[eventType]; ok {
		return cb
	}

	cb = NewCircuitBreaker(eventType, circuitBreakerMaxFailures, defaultDelayLevel3)
	h.circuitBreakers[eventType] = cb
	return cb
}

// RetryQueueManager manages multiple retry queue handlers.
type RetryQueueManager struct {
	handlers       map[RetryLevel]*RetryQueueHandler
	queuePublisher *QueuePublisher
	deduplication  DeduplicationStore
}

// NewRetryQueueManager creates a new retry queue manager.
func NewRetryQueueManager(
	eventHandler EventHandler,
	publisher *QueuePublisher,
	dedup DeduplicationStore,
) *RetryQueueManager {
	manager := &RetryQueueManager{
		handlers:       make(map[RetryLevel]*RetryQueueHandler),
		queuePublisher: publisher,
		deduplication:  dedup,
	}

	// Create handlers for each level
	for _, config := range DefaultRetryQueueConfigs() {
		manager.handlers[config.Level] = NewRetryQueueHandler(
			config,
			eventHandler,
			publisher,
			dedup,
		)
	}

	return manager
}

// GetHandler returns the handler for a specific retry level.
func (m *RetryQueueManager) GetHandler(level RetryLevel) *RetryQueueHandler {
	return m.handlers[level]
}

// GetHandlers returns all retry queue handlers.
func (m *RetryQueueManager) GetHandlers() map[RetryLevel]*RetryQueueHandler {
	return m.handlers
}

// QueueSubscriptions returns the queue name to handler mapping for Frame registration.
func (m *RetryQueueManager) QueueSubscriptions() map[string]FrameQueueHandler {
	subs := make(map[string]FrameQueueHandler)
	for _, config := range DefaultRetryQueueConfigs() {
		subs[config.QueueName] = m.handlers[config.Level]
	}
	return subs
}
