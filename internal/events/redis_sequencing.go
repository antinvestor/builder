package events

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis sequence key prefix.
const sequenceKeyPrefix = "seq:"

// RedisSequenceManager is a Redis-backed sequence manager.
// It uses Redis INCR for atomic sequence number generation.
type RedisSequenceManager struct {
	client *redis.Client
}

// NewRedisSequenceManager creates a new Redis-backed sequence manager.
func NewRedisSequenceManager(client *redis.Client) *RedisSequenceManager {
	return &RedisSequenceManager{
		client: client,
	}
}

// NextSequence returns the next sequence number for an execution.
// Uses Redis INCR for atomic increment.
func (m *RedisSequenceManager) NextSequence(ctx context.Context, executionID ExecutionID) (uint64, error) {
	key := sequenceKeyPrefix + executionID.String()

	result, err := m.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("incr: %w", err)
	}

	if result < 0 {
		return 0, errors.New("sequence counter overflow")
	}

	return uint64(result), nil
}

// CurrentSequence returns the current (last issued) sequence number.
func (m *RedisSequenceManager) CurrentSequence(ctx context.Context, executionID ExecutionID) (uint64, error) {
	key := sequenceKeyPrefix + executionID.String()

	result, err := m.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get: %w", err)
	}

	seq, err := strconv.ParseUint(result, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse: %w", err)
	}

	return seq, nil
}

// ValidateSequence checks if a sequence number is valid (not already used).
func (m *RedisSequenceManager) ValidateSequence(
	ctx context.Context,
	executionID ExecutionID,
	seq uint64,
) (bool, error) {
	current, err := m.CurrentSequence(ctx, executionID)
	if err != nil {
		return false, err
	}
	return seq > current, nil
}

// ReserveSequenceRange reserves a range of sequence numbers for batch operations.
// Uses INCRBY for atomic reservation.
func (m *RedisSequenceManager) ReserveSequenceRange(
	ctx context.Context,
	executionID ExecutionID,
	count int,
) (uint64, uint64, error) {
	if count <= 0 {
		return 0, 0, errors.New("count must be positive")
	}

	key := sequenceKeyPrefix + executionID.String()

	// Atomically increment by count to reserve the range
	result, err := m.client.IncrBy(ctx, key, int64(count)).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("incrby: %w", err)
	}

	if result < 0 {
		return 0, 0, errors.New("sequence counter overflow")
	}

	end := uint64(result)
	start := end - uint64(count) + 1

	return start, end, nil
}

// SetSequence sets the current sequence number for an execution.
// This is useful for restoring state or initializing a sequence.
func (m *RedisSequenceManager) SetSequence(ctx context.Context, executionID ExecutionID, seq uint64) error {
	key := sequenceKeyPrefix + executionID.String()
	return m.client.Set(ctx, key, seq, 0).Err()
}

// DeleteSequence removes the sequence counter for an execution.
// This should be called when an execution is complete and no longer needs tracking.
func (m *RedisSequenceManager) DeleteSequence(ctx context.Context, executionID ExecutionID) error {
	key := sequenceKeyPrefix + executionID.String()
	return m.client.Del(ctx, key).Err()
}

// GetSequenceState returns the full sequence state for an execution.
func (m *RedisSequenceManager) GetSequenceState(ctx context.Context, executionID ExecutionID) (*SequenceState, error) {
	current, err := m.CurrentSequence(ctx, executionID)
	if err != nil {
		return nil, err
	}

	return &SequenceState{
		ExecutionID:     executionID,
		CurrentSequence: current,
		LastUpdated:     time.Now(),
	}, nil
}
