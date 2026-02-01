package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pitabwire/util"
)

// DLQ recovery errors.
var (
	ErrDLQEntryNotFound  = errors.New("DLQ entry not found")
	ErrDLQEntryResolved  = errors.New("DLQ entry already resolved")
	ErrInvalidResolution = errors.New("invalid resolution status")
)

// DLQ configuration constants.
const (
	// defaultDLQListLimit is the default limit for listing DLQ entries.
	defaultDLQListLimit = 100

	// dlqRetentionDaysDLQ is the default retention period in days for DLQ entries.
	dlqRetentionDaysDLQ = 28

	// secondsPerMinute is used for age calculation.
	secondsPerMinute = 60.0
)

// DLQRecoveryService manages DLQ entries and recovery operations.
type DLQRecoveryService interface {
	// AddEntry adds an entry to the DLQ store.
	AddEntry(ctx context.Context, entry *DLQEntry) (string, error)

	// ListDLQEntries returns DLQ entries matching the filter.
	ListDLQEntries(ctx context.Context, filter DLQFilter) (*DLQListResult, error)

	// GetDLQEntry returns a single DLQ entry by ID.
	GetDLQEntry(ctx context.Context, entryID string) (*DLQEntry, error)

	// RequeueEntry requeues a DLQ entry for processing.
	RequeueEntry(ctx context.Context, entryID string, req RequeueRequest) error

	// DiscardEntry marks a DLQ entry as discarded.
	DiscardEntry(ctx context.Context, entryID string, req DiscardRequest) error

	// GetDLQStats returns DLQ statistics.
	GetDLQStats(ctx context.Context) (*DLQStats, error)

	// CleanupExpired removes expired DLQ entries.
	CleanupExpired(ctx context.Context) (int, error)
}

// DLQFilter defines filter criteria for listing DLQ entries.
type DLQFilter struct {
	// ExecutionID filters by feature execution ID.
	ExecutionID ExecutionID `json:"execution_id,omitzero"`

	// EventType filters by event type.
	EventType EventType `json:"event_type,omitempty"`

	// FailureClass filters by failure classification.
	FailureClass DLQFailureClass `json:"failure_class,omitempty"`

	// ManualReviewOnly returns only entries requiring manual review.
	ManualReviewOnly bool `json:"manual_review_only,omitempty"`

	// IncludeResolved includes resolved entries.
	IncludeResolved bool `json:"include_resolved,omitempty"`

	// EnteredAfter filters entries entered after this time.
	EnteredAfter time.Time `json:"entered_after,omitzero"`

	// EnteredBefore filters entries entered before this time.
	EnteredBefore time.Time `json:"entered_before,omitzero"`

	// Limit is the maximum number of entries to return.
	Limit int `json:"limit,omitempty"`

	// Offset is the pagination offset.
	Offset int `json:"offset,omitempty"`
}

// DLQListResult is the result of listing DLQ entries.
type DLQListResult struct {
	// Entries are the matching DLQ entries.
	Entries []*DLQEntryWithID `json:"entries"`

	// Total is the total number of matching entries.
	Total int `json:"total"`

	// HasMore indicates if there are more entries.
	HasMore bool `json:"has_more"`
}

// DLQEntryWithID is a DLQ entry with its storage ID.
type DLQEntryWithID struct {
	// ID is the storage identifier.
	ID string `json:"id"`

	// Entry is the DLQ entry.
	DLQEntry
}

// RequeueRequest is a request to requeue a DLQ entry.
type RequeueRequest struct {
	// ResolvedBy is who is requeuing the entry.
	ResolvedBy string `json:"resolved_by"`

	// Notes are optional notes about the requeue.
	Notes string `json:"notes,omitempty"`

	// TargetQueue is the queue to send to (defaults to main event queue).
	TargetQueue string `json:"target_queue,omitempty"`

	// ResetRetryCount resets the retry count to 0.
	ResetRetryCount bool `json:"reset_retry_count,omitempty"`
}

// DiscardRequest is a request to discard a DLQ entry.
type DiscardRequest struct {
	// ResolvedBy is who is discarding the entry.
	ResolvedBy string `json:"resolved_by"`

	// Notes are required notes explaining why it's being discarded.
	Notes string `json:"notes"`
}

// DLQStats contains DLQ statistics.
type DLQStats struct {
	// TotalEntries is the total number of entries.
	TotalEntries int `json:"total_entries"`

	// PendingEntries is the number of unresolved entries.
	PendingEntries int `json:"pending_entries"`

	// ResolvedEntries is the number of resolved entries.
	ResolvedEntries int `json:"resolved_entries"`

	// RequiresReview is the number requiring manual review.
	RequiresReview int `json:"requires_review"`

	// ByFailureClass breaks down entries by failure class.
	ByFailureClass map[DLQFailureClass]int `json:"by_failure_class"`

	// ByEventType breaks down entries by event type.
	ByEventType map[EventType]int `json:"by_event_type"`

	// ByResolutionStatus breaks down by resolution status.
	ByResolutionStatus map[DLQResolutionStatus]int `json:"by_resolution_status"`

	// OldestEntry is the timestamp of the oldest unresolved entry.
	OldestEntry time.Time `json:"oldest_entry,omitzero"`

	// NewestEntry is the timestamp of the newest entry.
	NewestEntry time.Time `json:"newest_entry,omitzero"`

	// AverageAgeMinutes is the average age of unresolved entries.
	AverageAgeMinutes float64 `json:"average_age_minutes"`

	// LastUpdated is when stats were last computed.
	LastUpdated time.Time `json:"last_updated"`
}

// InMemoryDLQStore is an in-memory implementation of DLQRecoveryService.
// Suitable for testing and single-instance deployments.
type InMemoryDLQStore struct {
	mu             sync.RWMutex
	entries        map[string]*DLQEntryWithID
	queuePublisher *QueuePublisher
	mainQueueName  string
	idCounter      int64
}

// NewInMemoryDLQStore creates a new in-memory DLQ store.
func NewInMemoryDLQStore(publisher *QueuePublisher, mainQueueName string) *InMemoryDLQStore {
	return &InMemoryDLQStore{
		entries:        make(map[string]*DLQEntryWithID),
		queuePublisher: publisher,
		mainQueueName:  mainQueueName,
	}
}

// AddEntry adds an entry to the DLQ store.
func (s *InMemoryDLQStore) AddEntry(_ context.Context, entry *DLQEntry) (string, error) {
	// Use atomic increment to generate unique IDs safely
	counter := atomic.AddInt64(&s.idCounter, 1)
	id := fmt.Sprintf("dlq-%d", counter)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries[id] = &DLQEntryWithID{
		ID:       id,
		DLQEntry: *entry,
	}

	return id, nil
}

// ListDLQEntries returns DLQ entries matching the filter.
func (s *InMemoryDLQStore) ListDLQEntries(_ context.Context, filter DLQFilter) (*DLQListResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var matching []*DLQEntryWithID

	for _, entry := range s.entries {
		if s.matchesFilter(entry, filter) {
			matching = append(matching, entry)
		}
	}

	// Sort by entered time (newest first) using efficient O(n log n) algorithm
	sort.Slice(matching, func(i, j int) bool {
		return matching[j].EnteredDLQAt.Before(matching[i].EnteredDLQAt)
	})

	total := len(matching)
	limit := defaultDLQListLimit
	if filter.Limit > 0 {
		limit = filter.Limit
	}

	offset := max(filter.Offset, 0)

	// Apply pagination
	if offset >= len(matching) {
		return &DLQListResult{
			Entries: []*DLQEntryWithID{},
			Total:   total,
			HasMore: false,
		}, nil
	}

	end := min(offset+limit, len(matching))

	return &DLQListResult{
		Entries: matching[offset:end],
		Total:   total,
		HasMore: end < len(matching),
	}, nil
}

// matchesFilter checks if an entry matches the filter criteria.
func (s *InMemoryDLQStore) matchesFilter(entry *DLQEntryWithID, filter DLQFilter) bool {
	// Check resolution status
	if !filter.IncludeResolved && entry.Resolution != nil {
		return false
	}

	// Check execution ID
	if !filter.ExecutionID.IsZero() {
		if entry.Event == nil || entry.Event.FeatureExecutionID != filter.ExecutionID {
			return false
		}
	}

	// Check event type
	if filter.EventType != "" {
		if entry.Event == nil || entry.Event.EventType != filter.EventType {
			return false
		}
	}

	// Check failure class
	if filter.FailureClass != "" {
		if entry.FailureClassification != filter.FailureClass {
			return false
		}
	}

	// Check manual review
	if filter.ManualReviewOnly && !entry.ManualReviewRequired {
		return false
	}

	// Check time range
	if !filter.EnteredAfter.IsZero() && entry.EnteredDLQAt.Before(filter.EnteredAfter) {
		return false
	}

	if !filter.EnteredBefore.IsZero() && entry.EnteredDLQAt.After(filter.EnteredBefore) {
		return false
	}

	return true
}

// GetDLQEntry returns a single DLQ entry by ID.
func (s *InMemoryDLQStore) GetDLQEntry(_ context.Context, entryID string) (*DLQEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.entries[entryID]
	if !exists {
		return nil, ErrDLQEntryNotFound
	}

	return &entry.DLQEntry, nil
}

// RequeueEntry requeues a DLQ entry for processing.
func (s *InMemoryDLQStore) RequeueEntry(ctx context.Context, entryID string, req RequeueRequest) error {
	log := util.Log(ctx)

	s.mu.Lock()
	entry, exists := s.entries[entryID]
	if !exists {
		s.mu.Unlock()
		return ErrDLQEntryNotFound
	}

	if entry.Resolution != nil {
		s.mu.Unlock()
		return ErrDLQEntryResolved
	}

	// Mark as resolved
	entry.Resolution = &DLQResolution{
		Status:     DLQResolutionRequeued,
		ResolvedBy: req.ResolvedBy,
		ResolvedAt: time.Now(),
		Notes:      req.Notes,
	}
	s.mu.Unlock()

	// Prepare event for requeue
	event := entry.Event
	if req.ResetRetryCount {
		// Clear retry metadata
		if event.Metadata.Tags != nil {
			delete(event.Metadata.Tags, "retry_attempt")
			delete(event.Metadata.Tags, "retry_level")
			delete(event.Metadata.Tags, "last_error")
		}
		event.OriginalEventID = EventID{} // Clear original event ID
	}

	// Generate new event ID
	newEventID := NewEventID()
	event.EventID = newEventID
	event.CreatedAt = time.Now().UTC()
	event.HLCTimestamp = NewHybridTimestamp()

	// Add requeue metadata
	if event.Metadata.Tags == nil {
		event.Metadata.Tags = make(map[string]string)
	}
	event.Metadata.Tags["requeued_from_dlq"] = entryID
	event.Metadata.Tags["requeued_by"] = req.ResolvedBy
	event.Metadata.Tags["requeued_at"] = time.Now().Format(time.RFC3339)

	// Store the retry event ID
	s.mu.Lock()
	entry.Resolution.RetryEventID = newEventID
	s.mu.Unlock()

	// Publish to target queue
	targetQueue := req.TargetQueue
	if targetQueue == "" {
		targetQueue = s.mainQueueName
	}

	if publishErr := s.queuePublisher.Publish(ctx, targetQueue, event); publishErr != nil {
		log.WithError(publishErr).Error("failed to publish requeued event")

		// Rollback resolution
		s.mu.Lock()
		entry.Resolution = nil
		s.mu.Unlock()

		return fmt.Errorf("publish requeued event: %w", publishErr)
	}

	log.Info("requeued DLQ entry",
		"entry_id", entryID,
		"new_event_id", newEventID.String(),
		"target_queue", targetQueue,
		"resolved_by", req.ResolvedBy)

	return nil
}

// DiscardEntry marks a DLQ entry as discarded.
func (s *InMemoryDLQStore) DiscardEntry(ctx context.Context, entryID string, req DiscardRequest) error {
	log := util.Log(ctx)

	if req.Notes == "" {
		return fmt.Errorf("%w: notes are required for discard", ErrInvalidResolution)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[entryID]
	if !exists {
		return ErrDLQEntryNotFound
	}

	if entry.Resolution != nil {
		return ErrDLQEntryResolved
	}

	entry.Resolution = &DLQResolution{
		Status:     DLQResolutionDiscarded,
		ResolvedBy: req.ResolvedBy,
		ResolvedAt: time.Now(),
		Notes:      req.Notes,
	}

	log.Info("discarded DLQ entry",
		"entry_id", entryID,
		"resolved_by", req.ResolvedBy,
		"notes", req.Notes)

	return nil
}

// GetDLQStats returns DLQ statistics.
func (s *InMemoryDLQStore) GetDLQStats(_ context.Context) (*DLQStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &DLQStats{
		ByFailureClass:     make(map[DLQFailureClass]int),
		ByEventType:        make(map[EventType]int),
		ByResolutionStatus: make(map[DLQResolutionStatus]int),
		LastUpdated:        time.Now(),
	}

	var totalAgeSeconds float64
	var pendingCount int

	for _, entry := range s.entries {
		stats.TotalEntries++

		// Count by failure class
		stats.ByFailureClass[entry.FailureClassification]++

		// Count by event type
		if entry.Event != nil {
			stats.ByEventType[entry.Event.EventType]++
		}

		// Count resolved vs pending
		if entry.Resolution != nil {
			stats.ResolvedEntries++
			stats.ByResolutionStatus[entry.Resolution.Status]++
		} else {
			stats.PendingEntries++
			pendingCount++
			totalAgeSeconds += time.Since(entry.EnteredDLQAt).Seconds()

			// Track oldest unresolved
			if stats.OldestEntry.IsZero() || entry.EnteredDLQAt.Before(stats.OldestEntry) {
				stats.OldestEntry = entry.EnteredDLQAt
			}
		}

		// Track newest
		if stats.NewestEntry.IsZero() || entry.EnteredDLQAt.After(stats.NewestEntry) {
			stats.NewestEntry = entry.EnteredDLQAt
		}

		// Count requiring review
		if entry.ManualReviewRequired && entry.Resolution == nil {
			stats.RequiresReview++
		}
	}

	// Calculate average age
	if pendingCount > 0 {
		stats.AverageAgeMinutes = (totalAgeSeconds / float64(pendingCount)) / secondsPerMinute
	}

	return stats, nil
}

// CleanupExpired removes expired DLQ entries.
func (s *InMemoryDLQStore) CleanupExpired(ctx context.Context) (int, error) {
	log := util.Log(ctx)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	removed := 0

	for id, entry := range s.entries {
		if entry.ExpiresAt.Before(now) {
			delete(s.entries, id)
			removed++
		}
	}

	if removed > 0 {
		log.Info("cleaned up expired DLQ entries", "count", removed)
	}

	return removed, nil
}

// DLQQueueHandler handles incoming DLQ messages from the queue.
// It stores entries in the DLQ store for recovery.
type DLQQueueHandler struct {
	store DLQRecoveryService
}

// NewDLQQueueHandler creates a new DLQ queue handler.
func NewDLQQueueHandler(store DLQRecoveryService) *DLQQueueHandler {
	return &DLQQueueHandler{store: store}
}

// Handle processes an incoming DLQ queue message.
// Implements FrameQueueHandler interface.
func (h *DLQQueueHandler) Handle(ctx context.Context, _ map[string]string, payload []byte) error {
	log := util.Log(ctx)

	// Try to deserialize as DLQ entry first
	var entry DLQEntry
	if err := json.Unmarshal(payload, &entry); err != nil {
		// If that fails, try as a raw event
		event, eventErr := QueuePayloadToEvent(payload)
		if eventErr != nil {
			log.WithError(eventErr).Error("failed to deserialize DLQ message")
			return nil // Don't fail - we're already in DLQ
		}

		// Create a DLQ entry from the raw event
		entry = DLQEntry{
			Event: event,
			RetryMetadata: RetryMetadata{
				LastAttemptAt: time.Now(),
			},
			FailureReason:         "direct DLQ delivery",
			FailureClassification: DLQFailureUnknown,
			EnteredDLQAt:          time.Now(),
			ExpiresAt:             time.Now().AddDate(0, 0, dlqRetentionDaysDLQ),
			ManualReviewRequired:  true,
		}
	}

	// Add to store using the interface method
	id, addErr := h.store.AddEntry(ctx, &entry)
	if addErr != nil {
		log.WithError(addErr).Error("failed to add DLQ entry")
		return addErr
	}
	log.Info("added DLQ entry", "id", id)

	return nil
}
