package events_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/antinvestor/builder/internal/events"
)

func getRedisClient(t *testing.T) *redis.Client {
	t.Helper()

	url := os.Getenv("REDIS_URL")
	if url == "" {
		url = "redis://localhost:6379"
	}

	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Skipf("invalid redis URL: %v", err)
	}

	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if pingErr := client.Ping(ctx).Err(); pingErr != nil {
		t.Skipf("redis not available: %v", pingErr)
	}

	// Clean up test keys before and after test
	t.Cleanup(func() {
		cleanupTestKeys(context.Background(), client)
		client.Close()
	})
	cleanupTestKeys(context.Background(), client)

	return client
}

func cleanupTestKeys(ctx context.Context, client *redis.Client) {
	// Clean up dedup keys
	iter := client.Scan(ctx, 0, "dedup:*", 0).Iterator()
	for iter.Next(ctx) {
		client.Del(ctx, iter.Val())
	}

	// Clean up lock keys
	iter = client.Scan(ctx, 0, "lock:*", 0).Iterator()
	for iter.Next(ctx) {
		client.Del(ctx, iter.Val())
	}

	// Clean up seq keys
	iter = client.Scan(ctx, 0, "seq:*", 0).Iterator()
	for iter.Next(ctx) {
		client.Del(ctx, iter.Val())
	}
}

func TestRedisDeduplicationStore(t *testing.T) {
	client := getRedisClient(t)
	store := events.NewRedisDeduplicationStore(client, time.Hour)
	ctx := context.Background()

	t.Run("mark and check processed", func(t *testing.T) {
		eventID := events.NewEventID()
		executionID := events.NewExecutionID()

		// Not processed initially
		processed, err := store.IsProcessed(ctx, eventID)
		require.NoError(t, err)
		assert.False(t, processed)

		// Mark as processed
		err = store.MarkProcessed(ctx, eventID, executionID)
		require.NoError(t, err)

		// Now it should be processed
		processed, err = store.IsProcessed(ctx, eventID)
		require.NoError(t, err)
		assert.True(t, processed)
	})

	t.Run("mark with result and retrieve", func(t *testing.T) {
		eventID := events.NewEventID()
		executionID := events.NewExecutionID()

		result := &events.ProcessingResult{
			EventID:     eventID,
			ExecutionID: executionID,
			ProcessedAt: time.Now(),
			Success:     true,
			DurationMS:  100,
		}

		err := store.MarkProcessedWithResult(ctx, eventID, executionID, result)
		require.NoError(t, err)

		// Retrieve the result
		retrieved, err := store.GetProcessingResult(ctx, eventID)
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, result.Success, retrieved.Success)
		assert.Equal(t, result.DurationMS, retrieved.DurationMS)
	})

	t.Run("get result for non-existent event", func(t *testing.T) {
		eventID := events.NewEventID()

		result, err := store.GetProcessingResult(ctx, eventID)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestRedisLockManager(t *testing.T) {
	client := getRedisClient(t)
	manager := events.NewRedisLockManager(client)
	ctx := context.Background()

	t.Run("acquire and release lock", func(t *testing.T) {
		key := "test-lock-1"
		owner := "owner-1"
		ttl := 5 * time.Second

		lock, err := manager.Acquire(ctx, key, owner, ttl)
		require.NoError(t, err)
		require.NotNil(t, lock)

		assert.Equal(t, key, lock.Key())
		assert.Equal(t, owner, lock.Owner())

		// Check it's locked
		locked, err := manager.IsLocked(ctx, key)
		require.NoError(t, err)
		assert.True(t, locked)

		// Release
		err = lock.Unlock(ctx)
		require.NoError(t, err)

		// Check it's unlocked
		locked, err = manager.IsLocked(ctx, key)
		require.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("lock contention", func(t *testing.T) {
		key := "test-lock-2"
		owner1 := "owner-1"
		owner2 := "owner-2"
		ttl := 5 * time.Second

		// Owner1 acquires
		lock1, err := manager.Acquire(ctx, key, owner1, ttl)
		require.NoError(t, err)
		defer lock1.Unlock(ctx)

		// Owner2 tries to acquire (should fail quickly with try)
		lock2, acquired, err := manager.TryAcquire(ctx, key, owner2, ttl)
		require.NoError(t, err)
		assert.False(t, acquired)
		assert.Nil(t, lock2)
	})

	t.Run("same owner reacquires", func(t *testing.T) {
		key := "test-lock-3"
		owner := "owner-1"
		ttl := 5 * time.Second

		// First acquire
		lock1, err := manager.Acquire(ctx, key, owner, ttl)
		require.NoError(t, err)

		// Same owner acquires again (should extend)
		lock2, acquired, err := manager.TryAcquire(ctx, key, owner, ttl)
		require.NoError(t, err)
		assert.True(t, acquired)
		assert.NotNil(t, lock2)

		// Both should refer to the same lock
		err = lock1.Unlock(ctx)
		require.NoError(t, err)

		// Lock should be released
		locked, err := manager.IsLocked(ctx, key)
		require.NoError(t, err)
		assert.False(t, locked)
	})

	t.Run("extend lock", func(t *testing.T) {
		key := "test-lock-4"
		owner := "owner-1"
		ttl := 2 * time.Second

		lock, err := manager.Acquire(ctx, key, owner, ttl)
		require.NoError(t, err)
		defer lock.Unlock(ctx)

		originalExpiry := lock.ExpiresAt()

		// Wait a bit
		time.Sleep(100 * time.Millisecond)

		// Extend
		err = lock.Extend(ctx, 5*time.Second)
		require.NoError(t, err)

		// New expiry should be later
		assert.True(t, lock.ExpiresAt().After(originalExpiry))
	})

	t.Run("lock info", func(t *testing.T) {
		key := "test-lock-5"
		owner := "owner-1"
		ttl := 5 * time.Second

		lock, err := manager.Acquire(ctx, key, owner, ttl)
		require.NoError(t, err)
		defer lock.Unlock(ctx)

		info, err := manager.GetLockInfo(ctx, key)
		require.NoError(t, err)
		require.NotNil(t, info)
		assert.Equal(t, key, info.Key)
		assert.Equal(t, owner, info.Owner)
	})

	t.Run("is held check", func(t *testing.T) {
		key := "test-lock-6"
		owner := "owner-1"
		ttl := 5 * time.Second

		lock, err := manager.Acquire(ctx, key, owner, ttl)
		require.NoError(t, err)

		held, err := lock.IsHeld(ctx)
		require.NoError(t, err)
		assert.True(t, held)

		err = lock.Unlock(ctx)
		require.NoError(t, err)

		held, err = lock.IsHeld(ctx)
		require.NoError(t, err)
		assert.False(t, held)
	})
}

func TestRedisSequenceManager(t *testing.T) {
	client := getRedisClient(t)
	manager := events.NewRedisSequenceManager(client)
	ctx := context.Background()

	t.Run("next sequence increments", func(t *testing.T) {
		execID := events.NewExecutionID()

		seq1, err := manager.NextSequence(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), seq1)

		seq2, err := manager.NextSequence(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), seq2)

		seq3, err := manager.NextSequence(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), seq3)
	})

	t.Run("current sequence", func(t *testing.T) {
		execID := events.NewExecutionID()

		// Initial should be 0
		current, err := manager.CurrentSequence(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), current)

		// After increment
		_, err = manager.NextSequence(ctx, execID)
		require.NoError(t, err)

		current, err = manager.CurrentSequence(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), current)
	})

	t.Run("validate sequence", func(t *testing.T) {
		execID := events.NewExecutionID()

		// Sequence 1 should be valid initially
		valid, err := manager.ValidateSequence(ctx, execID, 1)
		require.NoError(t, err)
		assert.True(t, valid)

		// Use sequence 1
		_, err = manager.NextSequence(ctx, execID)
		require.NoError(t, err)

		// Sequence 1 should now be invalid
		valid, err = manager.ValidateSequence(ctx, execID, 1)
		require.NoError(t, err)
		assert.False(t, valid)

		// Sequence 2 should be valid
		valid, err = manager.ValidateSequence(ctx, execID, 2)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("reserve sequence range", func(t *testing.T) {
		execID := events.NewExecutionID()

		start, end, err := manager.ReserveSequenceRange(ctx, execID, 5)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), start)
		assert.Equal(t, uint64(5), end)

		// Next sequence should be 6
		seq, err := manager.NextSequence(ctx, execID)
		require.NoError(t, err)
		assert.Equal(t, uint64(6), seq)
	})

	t.Run("isolated executions", func(t *testing.T) {
		execID1 := events.NewExecutionID()
		execID2 := events.NewExecutionID()

		// Increment exec1 a few times
		for range 5 {
			_, err := manager.NextSequence(ctx, execID1)
			require.NoError(t, err)
		}

		// exec2 should start at 1
		seq, err := manager.NextSequence(ctx, execID2)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), seq)

		// exec1 should be at 6
		seq, err = manager.NextSequence(ctx, execID1)
		require.NoError(t, err)
		assert.Equal(t, uint64(6), seq)
	})
}

func TestBackendFactory(t *testing.T) {
	t.Run("memory backends", func(t *testing.T) {
		ctx := context.Background()
		cfg := events.DefaultBackendConfig()

		backends, err := events.NewBackends(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, backends)

		assert.NotNil(t, backends.Deduplication)
		assert.NotNil(t, backends.Locking)
		assert.NotNil(t, backends.Sequencing)

		err = backends.Close()
		require.NoError(t, err)
	})

	t.Run("redis backends require URL", func(t *testing.T) {
		ctx := context.Background()
		cfg := events.BackendConfig{
			DeduplicationBackend: events.BackendRedis,
			LockingBackend:       events.BackendMemory,
			SequencingBackend:    events.BackendMemory,
			RedisURL:             "", // Empty URL should fail
		}

		_, err := events.NewBackends(ctx, cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "redis URL required")
	})

	t.Run("fallback to memory on redis failure", func(t *testing.T) {
		ctx := context.Background()
		cfg := events.BackendConfig{
			DeduplicationBackend: events.BackendRedis,
			LockingBackend:       events.BackendRedis,
			SequencingBackend:    events.BackendRedis,
			RedisURL:             "redis://nonexistent:6379", // Should fail
		}

		backends, err := events.NewBackendsWithFallback(ctx, cfg)
		require.NoError(t, err)
		require.NotNil(t, backends)

		// Should have fallen back to memory backends
		assert.NotNil(t, backends.Deduplication)
		assert.NotNil(t, backends.Locking)
		assert.NotNil(t, backends.Sequencing)

		err = backends.Close()
		require.NoError(t, err)
	})
}
