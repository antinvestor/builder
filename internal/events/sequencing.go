package events

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SequenceManager manages event sequence numbers for feature executions.
// It ensures strictly monotonic sequence numbers within each execution.
type SequenceManager interface {
	// NextSequence returns the next sequence number for an execution.
	// This is atomic and guarantees uniqueness.
	NextSequence(ctx context.Context, executionID ExecutionID) (uint64, error)

	// CurrentSequence returns the current (last issued) sequence number.
	CurrentSequence(ctx context.Context, executionID ExecutionID) (uint64, error)

	// ValidateSequence checks if a sequence number is valid (not already used).
	ValidateSequence(ctx context.Context, executionID ExecutionID, seq uint64) (bool, error)

	// ReserveSequenceRange reserves a range of sequence numbers for batch operations.
	ReserveSequenceRange(ctx context.Context, executionID ExecutionID, count int) (start uint64, end uint64, err error)
}

// SequenceState tracks sequence state for a single execution.
type SequenceState struct {
	ExecutionID     ExecutionID `json:"execution_id"`
	CurrentSequence uint64      `json:"current_sequence"`
	LastUpdated     time.Time   `json:"last_updated"`
}

// InMemorySequenceManager is an in-memory implementation for testing.
type InMemorySequenceManager struct {
	mu       sync.RWMutex
	sequences map[string]*SequenceState
}

// NewInMemorySequenceManager creates a new in-memory sequence manager.
func NewInMemorySequenceManager() *InMemorySequenceManager {
	return &InMemorySequenceManager{
		sequences: make(map[string]*SequenceState),
	}
}

// NextSequence returns the next sequence number for an execution.
func (m *InMemorySequenceManager) NextSequence(ctx context.Context, executionID ExecutionID) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := executionID.String()
	state, ok := m.sequences[key]
	if !ok {
		state = &SequenceState{
			ExecutionID:     executionID,
			CurrentSequence: 0,
		}
		m.sequences[key] = state
	}

	state.CurrentSequence++
	state.LastUpdated = time.Now()
	return state.CurrentSequence, nil
}

// CurrentSequence returns the current sequence number.
func (m *InMemorySequenceManager) CurrentSequence(ctx context.Context, executionID ExecutionID) (uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := executionID.String()
	state, ok := m.sequences[key]
	if !ok {
		return 0, nil
	}
	return state.CurrentSequence, nil
}

// ValidateSequence checks if a sequence number is valid.
func (m *InMemorySequenceManager) ValidateSequence(ctx context.Context, executionID ExecutionID, seq uint64) (bool, error) {
	current, err := m.CurrentSequence(ctx, executionID)
	if err != nil {
		return false, err
	}
	return seq > current, nil
}

// ReserveSequenceRange reserves a range of sequence numbers.
func (m *InMemorySequenceManager) ReserveSequenceRange(ctx context.Context, executionID ExecutionID, count int) (uint64, uint64, error) {
	if count <= 0 {
		return 0, 0, fmt.Errorf("count must be positive")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := executionID.String()
	state, ok := m.sequences[key]
	if !ok {
		state = &SequenceState{
			ExecutionID:     executionID,
			CurrentSequence: 0,
		}
		m.sequences[key] = state
	}

	start := state.CurrentSequence + 1
	end := state.CurrentSequence + uint64(count)
	state.CurrentSequence = end
	state.LastUpdated = time.Now()

	return start, end, nil
}

// HLCManager manages hybrid logical clock timestamps.
type HLCManager struct {
	mu          sync.Mutex
	lastPhysical int64
	lastLogical  uint32
}

// NewHLCManager creates a new HLC manager.
func NewHLCManager() *HLCManager {
	return &HLCManager{}
}

// Now returns a new HLC timestamp guaranteed to be greater than any previous.
func (h *HLCManager) Now() HybridTimestamp {
	h.mu.Lock()
	defer h.mu.Unlock()

	physical := time.Now().UnixMilli()

	if physical > h.lastPhysical {
		h.lastPhysical = physical
		h.lastLogical = 0
	} else {
		h.lastLogical++
	}

	return HybridTimestamp{
		PhysicalMS: h.lastPhysical,
		Logical:    h.lastLogical,
	}
}

// Update updates the HLC based on a received timestamp.
// Returns the updated timestamp.
func (h *HLCManager) Update(received HybridTimestamp) HybridTimestamp {
	h.mu.Lock()
	defer h.mu.Unlock()

	physical := time.Now().UnixMilli()
	maxPhysical := max(physical, max(h.lastPhysical, received.PhysicalMS))

	var logical uint32
	if maxPhysical == h.lastPhysical && maxPhysical == received.PhysicalMS {
		logical = max(h.lastLogical, received.Logical) + 1
	} else if maxPhysical == h.lastPhysical {
		logical = h.lastLogical + 1
	} else if maxPhysical == received.PhysicalMS {
		logical = received.Logical + 1
	} else {
		logical = 0
	}

	h.lastPhysical = maxPhysical
	h.lastLogical = logical

	return HybridTimestamp{
		PhysicalMS: maxPhysical,
		Logical:    logical,
	}
}

// Compare compares two HLC timestamps.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (h HybridTimestamp) Compare(other HybridTimestamp) int {
	if h.PhysicalMS < other.PhysicalMS {
		return -1
	}
	if h.PhysicalMS > other.PhysicalMS {
		return 1
	}
	if h.Logical < other.Logical {
		return -1
	}
	if h.Logical > other.Logical {
		return 1
	}
	return 0
}

// String returns the string representation of an HLC timestamp.
func (h HybridTimestamp) String() string {
	return fmt.Sprintf("%d.%d", h.PhysicalMS, h.Logical)
}

// EventSequencer combines sequence and HLC management for event creation.
type EventSequencer struct {
	sequences SequenceManager
	hlc       *HLCManager
}

// NewEventSequencer creates a new event sequencer.
func NewEventSequencer(sequences SequenceManager) *EventSequencer {
	return &EventSequencer{
		sequences: sequences,
		hlc:       NewHLCManager(),
	}
}

// PrepareEvent assigns sequence number and HLC timestamp to an event builder.
func (s *EventSequencer) PrepareEvent(ctx context.Context, builder *EventBuilder) (*EventBuilder, error) {
	seq, err := s.sequences.NextSequence(ctx, builder.event.FeatureExecutionID)
	if err != nil {
		return nil, fmt.Errorf("get next sequence: %w", err)
	}

	hlc := s.hlc.Now()

	return builder.
		WithSequence(seq).
		withHLC(hlc), nil
}

// withHLC sets the HLC timestamp on the builder.
func (b *EventBuilder) withHLC(hlc HybridTimestamp) *EventBuilder {
	b.event.HLCTimestamp = hlc
	return b
}

// OrderingKey represents a key for ordering events.
type OrderingKey struct {
	ExecutionID    ExecutionID
	SequenceNumber uint64
}

// String returns the string representation.
func (k OrderingKey) String() string {
	return fmt.Sprintf("%s:%d", k.ExecutionID.String(), k.SequenceNumber)
}

// EventOrdering provides utilities for event ordering.
type EventOrdering struct{}

// CompareEvents compares two events for ordering.
// Returns -1 if a should come before b, 0 if equal, 1 if a should come after b.
func (EventOrdering) CompareEvents(a, b *Event) int {
	// Same execution: use sequence number
	if a.FeatureExecutionID == b.FeatureExecutionID {
		if a.SequenceNumber < b.SequenceNumber {
			return -1
		}
		if a.SequenceNumber > b.SequenceNumber {
			return 1
		}
		return 0
	}

	// Different executions: use HLC
	return a.HLCTimestamp.Compare(b.HLCTimestamp)
}

// IsCausallyBefore returns true if event a is causally before event b.
func (EventOrdering) IsCausallyBefore(a, b *Event) bool {
	// Same execution: use sequence
	if a.FeatureExecutionID == b.FeatureExecutionID {
		return a.SequenceNumber < b.SequenceNumber
	}

	// Check causation chain
	if b.CausationID == a.EventID {
		return true
	}

	// Fall back to HLC
	return a.HLCTimestamp.Compare(b.HLCTimestamp) < 0
}

// EventStream represents a stream of events for a single execution.
type EventStream struct {
	ExecutionID ExecutionID
	Events      []*Event
}

// Validate checks that the stream is properly ordered.
func (s *EventStream) Validate() error {
	for i := 1; i < len(s.Events); i++ {
		prev := s.Events[i-1]
		curr := s.Events[i]

		if curr.SequenceNumber != prev.SequenceNumber+1 {
			return fmt.Errorf("sequence gap: expected %d, got %d at index %d",
				prev.SequenceNumber+1, curr.SequenceNumber, i)
		}

		if prev.HLCTimestamp.Compare(curr.HLCTimestamp) >= 0 {
			return fmt.Errorf("HLC not increasing at index %d", i)
		}
	}
	return nil
}

// FindGaps returns any sequence gaps in the stream.
func (s *EventStream) FindGaps() []uint64 {
	var gaps []uint64
	for i := 1; i < len(s.Events); i++ {
		prev := s.Events[i-1]
		curr := s.Events[i]

		for seq := prev.SequenceNumber + 1; seq < curr.SequenceNumber; seq++ {
			gaps = append(gaps, seq)
		}
	}
	return gaps
}
