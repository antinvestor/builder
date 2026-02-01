package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key prefixes and TTLs.
const (
	dedupKeyPrefix  = "dedup:"
	defaultDedupTTL = 24 * time.Hour
)

// RedisDeduplicationStore is a Redis-backed deduplication store.
type RedisDeduplicationStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisDeduplicationStore creates a new Redis-backed deduplication store.
func NewRedisDeduplicationStore(client *redis.Client, ttl time.Duration) *RedisDeduplicationStore {
	if ttl <= 0 {
		ttl = defaultDedupTTL
	}
	return &RedisDeduplicationStore{
		client: client,
		ttl:    ttl,
	}
}

// MarkProcessed marks an event as processed.
func (s *RedisDeduplicationStore) MarkProcessed(ctx context.Context, eventID EventID, executionID ExecutionID) error {
	entry := &redisDeduplicationEntry{
		EventID:     eventID.String(),
		ExecutionID: executionID.String(),
		ProcessedAt: time.Now(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	key := dedupKeyPrefix + eventID.String()
	if setErr := s.client.Set(ctx, key, data, s.ttl).Err(); setErr != nil {
		return fmt.Errorf("set key: %w", setErr)
	}

	return nil
}

// IsProcessed checks if an event has been processed.
func (s *RedisDeduplicationStore) IsProcessed(ctx context.Context, eventID EventID) (bool, error) {
	key := dedupKeyPrefix + eventID.String()
	exists, err := s.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("check exists: %w", err)
	}
	return exists > 0, nil
}

// MarkProcessedWithResult marks an event as processed with its result.
func (s *RedisDeduplicationStore) MarkProcessedWithResult(
	ctx context.Context,
	eventID EventID,
	executionID ExecutionID,
	result *ProcessingResult,
) error {
	entry := &redisDeduplicationEntry{
		EventID:     eventID.String(),
		ExecutionID: executionID.String(),
		ProcessedAt: time.Now(),
		Result:      result,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal entry: %w", err)
	}

	key := dedupKeyPrefix + eventID.String()
	if setErr := s.client.Set(ctx, key, data, s.ttl).Err(); setErr != nil {
		return fmt.Errorf("set key: %w", setErr)
	}

	return nil
}

// redisDeduplicationEntry is the JSON-serializable form for Redis storage.
type redisDeduplicationEntry struct {
	EventID     string            `json:"event_id"`
	ExecutionID string            `json:"execution_id"`
	ProcessedAt time.Time         `json:"processed_at"`
	Result      *ProcessingResult `json:"result,omitempty"`
}

// GetProcessingResult returns the result of a processed event.
func (s *RedisDeduplicationStore) GetProcessingResult(ctx context.Context, eventID EventID) (*ProcessingResult, error) {
	key := dedupKeyPrefix + eventID.String()
	data, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil //nolint:nilnil // nil result is valid for non-existent events
	}
	if err != nil {
		return nil, fmt.Errorf("get key: %w", err)
	}

	var entry redisDeduplicationEntry
	if unmarshalErr := json.Unmarshal(data, &entry); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal entry: %w", unmarshalErr)
	}

	return entry.Result, nil
}

// Cleanup removes old deduplication entries.
// Redis handles TTL-based cleanup automatically, but this can be used for manual cleanup.
func (s *RedisDeduplicationStore) Cleanup(_ context.Context, _ time.Duration) (int, error) {
	// Redis handles TTL-based cleanup automatically.
	// This is a no-op for Redis since keys expire automatically.
	// For explicit cleanup, we would need to scan and delete keys,
	// but that's expensive and not recommended for production.
	return 0, nil
}
