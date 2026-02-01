package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DeduplicationStore tracks processed events for deduplication.
type DeduplicationStore interface {
	// MarkProcessed marks an event as processed.
	MarkProcessed(ctx context.Context, eventID EventID, executionID ExecutionID) error

	// IsProcessed checks if an event has been processed.
	IsProcessed(ctx context.Context, eventID EventID) (bool, error)

	// MarkProcessedWithResult marks an event as processed with its result.
	MarkProcessedWithResult(ctx context.Context, eventID EventID, executionID ExecutionID, result *ProcessingResult) error

	// GetProcessingResult returns the result of a processed event.
	GetProcessingResult(ctx context.Context, eventID EventID) (*ProcessingResult, error)

	// Cleanup removes old deduplication entries.
	Cleanup(ctx context.Context, olderThan time.Duration) (int, error)
}

// ProcessingResult stores the result of event processing for idempotency.
type ProcessingResult struct {
	// EventID is the processed event ID.
	EventID EventID `json:"event_id"`

	// ExecutionID is the feature execution ID.
	ExecutionID ExecutionID `json:"execution_id"`

	// ProcessedAt is when the event was processed.
	ProcessedAt time.Time `json:"processed_at"`

	// Success indicates if processing succeeded.
	Success bool `json:"success"`

	// ErrorCode is set if processing failed.
	ErrorCode string `json:"error_code,omitempty"`

	// ErrorMessage is set if processing failed.
	ErrorMessage string `json:"error_message,omitempty"`

	// ResultData stores arbitrary result data for replay.
	ResultData map[string]any `json:"result_data,omitempty"`

	// ProducedEventIDs are events produced as a result.
	ProducedEventIDs []EventID `json:"produced_event_ids,omitempty"`

	// Duration is how long processing took.
	DurationMS int64 `json:"duration_ms"`
}

// InMemoryDeduplicationStore is an in-memory implementation for testing.
type InMemoryDeduplicationStore struct {
	mu        sync.RWMutex
	entries   map[string]*deduplicationEntry
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

type deduplicationEntry struct {
	eventID     EventID
	executionID ExecutionID
	processedAt time.Time
	result      *ProcessingResult
}

// NewInMemoryDeduplicationStore creates a new in-memory deduplication store.
func NewInMemoryDeduplicationStore() *InMemoryDeduplicationStore {
	store := &InMemoryDeduplicationStore{
		entries:   make(map[string]*deduplicationEntry),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
	// Start cleanup goroutine
	go store.periodicCleanup()
	return store
}

// Close stops the deduplication store's cleanup goroutine gracefully.
func (s *InMemoryDeduplicationStore) Close() error {
	close(s.stopCh)
	<-s.stoppedCh
	return nil
}

func (s *InMemoryDeduplicationStore) periodicCleanup() {
	defer close(s.stoppedCh)

	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			_, _ = s.Cleanup(context.Background(), 24*time.Hour)
		}
	}
}

// MarkProcessed marks an event as processed.
func (s *InMemoryDeduplicationStore) MarkProcessed(ctx context.Context, eventID EventID, executionID ExecutionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := eventID.String()
	s.entries[key] = &deduplicationEntry{
		eventID:     eventID,
		executionID: executionID,
		processedAt: time.Now(),
	}
	return nil
}

// IsProcessed checks if an event has been processed.
func (s *InMemoryDeduplicationStore) IsProcessed(ctx context.Context, eventID EventID) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.entries[eventID.String()]
	return exists, nil
}

// MarkProcessedWithResult marks an event as processed with its result.
func (s *InMemoryDeduplicationStore) MarkProcessedWithResult(ctx context.Context, eventID EventID, executionID ExecutionID, result *ProcessingResult) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := eventID.String()
	s.entries[key] = &deduplicationEntry{
		eventID:     eventID,
		executionID: executionID,
		processedAt: time.Now(),
		result:      result,
	}
	return nil
}

// GetProcessingResult returns the result of a processed event.
func (s *InMemoryDeduplicationStore) GetProcessingResult(ctx context.Context, eventID EventID) (*ProcessingResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[eventID.String()]
	if !exists {
		return nil, nil
	}
	return entry.result, nil
}

// Cleanup removes old deduplication entries.
func (s *InMemoryDeduplicationStore) Cleanup(ctx context.Context, olderThan time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	removed := 0

	for key, entry := range s.entries {
		if entry.processedAt.Before(cutoff) {
			delete(s.entries, key)
			removed++
		}
	}

	return removed, nil
}

// IdempotentHandler wraps an event handler with deduplication.
type IdempotentHandler struct {
	store   DeduplicationStore
	handler EventHandler
}

// NewIdempotentHandler creates a new idempotent handler.
func NewIdempotentHandler(store DeduplicationStore, handler EventHandler) *IdempotentHandler {
	return &IdempotentHandler{
		store:   store,
		handler: handler,
	}
}

// Handle handles an event idempotently.
func (h *IdempotentHandler) Handle(ctx context.Context, event *Event) error {
	// Check if already processed
	processed, err := h.store.IsProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check processed: %w", err)
	}

	if processed {
		// Already processed, return cached result if available
		result, err := h.store.GetProcessingResult(ctx, event.EventID)
		if err != nil {
			return fmt.Errorf("get processing result: %w", err)
		}

		if result != nil && !result.Success {
			// Replay the error
			return fmt.Errorf("previous processing failed: %s", result.ErrorMessage)
		}

		// Success - skip processing
		return nil
	}

	// Process the event
	start := time.Now()
	handleErr := h.handler.Handle(ctx, event)
	duration := time.Since(start)

	// Record the result
	result := &ProcessingResult{
		EventID:     event.EventID,
		ExecutionID: event.FeatureExecutionID,
		ProcessedAt: time.Now(),
		Success:     handleErr == nil,
		DurationMS:  duration.Milliseconds(),
	}

	if handleErr != nil {
		result.ErrorMessage = handleErr.Error()
	}

	if err := h.store.MarkProcessedWithResult(ctx, event.EventID, event.FeatureExecutionID, result); err != nil {
		// Log but don't fail - the event was processed
		// Next delivery will hit the dedup check
	}

	return handleErr
}

// DeduplicationKey generates a deduplication key for an event.
type DeduplicationKey struct {
	// EventID is the unique event identifier.
	EventID EventID

	// ExecutionID provides execution-level scoping.
	ExecutionID ExecutionID

	// SequenceNumber provides ordering within execution.
	SequenceNumber uint64
}

// String returns the string representation of the key.
func (k DeduplicationKey) String() string {
	return fmt.Sprintf("%s:%s:%d", k.ExecutionID.String(), k.EventID.String(), k.SequenceNumber)
}

// NewDeduplicationKey creates a deduplication key from an event.
func NewDeduplicationKey(event *Event) DeduplicationKey {
	return DeduplicationKey{
		EventID:        event.EventID,
		ExecutionID:    event.FeatureExecutionID,
		SequenceNumber: event.SequenceNumber,
	}
}

// SequenceTracker tracks processed sequence numbers for gap detection.
type SequenceTracker struct {
	mu        sync.RWMutex
	sequences map[string]*sequenceState
}

type sequenceState struct {
	lastProcessed uint64
	processed     map[uint64]bool
	gaps          []uint64
}

// NewSequenceTracker creates a new sequence tracker.
func NewSequenceTracker() *SequenceTracker {
	return &SequenceTracker{
		sequences: make(map[string]*sequenceState),
	}
}

// RecordProcessed records that a sequence number was processed.
func (t *SequenceTracker) RecordProcessed(executionID ExecutionID, seq uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := executionID.String()
	state, ok := t.sequences[key]
	if !ok {
		state = &sequenceState{
			processed: make(map[uint64]bool),
		}
		t.sequences[key] = state
	}

	state.processed[seq] = true

	// Update gaps
	if seq > state.lastProcessed {
		// Check for new gaps
		for i := state.lastProcessed + 1; i < seq; i++ {
			if !state.processed[i] {
				state.gaps = append(state.gaps, i)
			}
		}
		state.lastProcessed = seq
	} else {
		// Fill in a gap
		newGaps := make([]uint64, 0, len(state.gaps))
		for _, g := range state.gaps {
			if g != seq {
				newGaps = append(newGaps, g)
			}
		}
		state.gaps = newGaps
	}
}

// GetGaps returns any gaps in the sequence.
func (t *SequenceTracker) GetGaps(executionID ExecutionID) []uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := executionID.String()
	state, ok := t.sequences[key]
	if !ok {
		return nil
	}

	result := make([]uint64, len(state.gaps))
	copy(result, state.gaps)
	return result
}

// GetLastProcessed returns the last processed sequence number.
func (t *SequenceTracker) GetLastProcessed(executionID ExecutionID) uint64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := executionID.String()
	state, ok := t.sequences[key]
	if !ok {
		return 0
	}
	return state.lastProcessed
}

// IsProcessed checks if a specific sequence was processed.
func (t *SequenceTracker) IsProcessed(executionID ExecutionID, seq uint64) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := executionID.String()
	state, ok := t.sequences[key]
	if !ok {
		return false
	}
	return state.processed[seq]
}

// ExactlyOnceProcessor combines all guarantees for exactly-once processing.
type ExactlyOnceProcessor struct {
	dedup    DeduplicationStore
	sequence *SequenceTracker
	handler  EventHandler
}

// NewExactlyOnceProcessor creates a processor with exactly-once guarantees.
func NewExactlyOnceProcessor(dedup DeduplicationStore, handler EventHandler) *ExactlyOnceProcessor {
	return &ExactlyOnceProcessor{
		dedup:    dedup,
		sequence: NewSequenceTracker(),
		handler:  handler,
	}
}

// Process processes an event with exactly-once guarantees.
func (p *ExactlyOnceProcessor) Process(ctx context.Context, event *Event) error {
	// Step 1: Check deduplication
	processed, err := p.dedup.IsProcessed(ctx, event.EventID)
	if err != nil {
		return fmt.Errorf("check dedup: %w", err)
	}

	if processed {
		// Record in sequence tracker for gap detection
		p.sequence.RecordProcessed(event.FeatureExecutionID, event.SequenceNumber)
		return nil
	}

	// Step 2: Process the event
	start := time.Now()
	handleErr := p.handler.Handle(ctx, event)
	duration := time.Since(start)

	// Step 3: Record result (even failures, for idempotency)
	result := &ProcessingResult{
		EventID:     event.EventID,
		ExecutionID: event.FeatureExecutionID,
		ProcessedAt: time.Now(),
		Success:     handleErr == nil,
		DurationMS:  duration.Milliseconds(),
	}

	if handleErr != nil {
		result.ErrorMessage = handleErr.Error()
	}

	if err := p.dedup.MarkProcessedWithResult(ctx, event.EventID, event.FeatureExecutionID, result); err != nil {
		// Best effort - don't fail the processing
	}

	// Step 4: Track sequence
	p.sequence.RecordProcessed(event.FeatureExecutionID, event.SequenceNumber)

	return handleErr
}

// GetGaps returns sequence gaps for an execution.
func (p *ExactlyOnceProcessor) GetGaps(executionID ExecutionID) []uint64 {
	return p.sequence.GetGaps(executionID)
}
