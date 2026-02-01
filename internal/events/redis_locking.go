package events

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis lock key prefix.
const lockKeyPrefix = "lock:"

// Lock acquisition polling constants.
const (
	lockPollInterval   = 50 * time.Millisecond
	defaultLockTimeout = 30 * time.Second
)

// RedisLockManager is a Redis-backed distributed lock manager.
// It implements a simplified Redlock-style algorithm for single Redis instance.
type RedisLockManager struct {
	client *redis.Client
}

// NewRedisLockManager creates a new Redis-backed lock manager.
func NewRedisLockManager(client *redis.Client) *RedisLockManager {
	return &RedisLockManager{
		client: client,
	}
}

// Acquire attempts to acquire a lock, blocking until acquired or context is done.
func (m *RedisLockManager) Acquire(ctx context.Context, key, owner string, ttl time.Duration) (DistributedLock, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(defaultLockTimeout)
	}

	ticker := time.NewTicker(lockPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			lock, acquired, err := m.TryAcquire(ctx, key, owner, ttl)
			if err != nil {
				return nil, err
			}
			if acquired {
				return lock, nil
			}
			if time.Now().After(deadline) {
				return nil, ErrLockNotAcquired
			}
		}
	}
}

// TryAcquire attempts to acquire a lock without blocking.
func (m *RedisLockManager) TryAcquire(
	ctx context.Context,
	key, owner string,
	ttl time.Duration,
) (DistributedLock, bool, error) {
	redisKey := lockKeyPrefix + key

	// Use a loop instead of recursion to avoid stack overflow under high contention
	for {
		// Try to set with NX (only if not exists)
		ok, err := m.client.SetNX(ctx, redisKey, owner, ttl).Result()
		if err != nil {
			return nil, false, fmt.Errorf("setnx: %w", err)
		}

		if ok {
			// Lock acquired
			lock := &redisLock{
				key:       key,
				owner:     owner,
				expiresAt: time.Now().Add(ttl),
				manager:   m,
			}
			return lock, true, nil
		}

		// Check if we already own the lock
		currentOwner, err := m.client.Get(ctx, redisKey).Result()
		if errors.Is(err, redis.Nil) {
			// Lock expired between SetNX and Get, retry
			continue
		}
		if err != nil {
			return nil, false, fmt.Errorf("get owner: %w", err)
		}

		if currentOwner == owner {
			// We already own it, extend the TTL atomically to prevent race conditions.
			// Use Lua script to atomically check owner and extend TTL.
			script := redis.NewScript(`
				if redis.call("GET", KEYS[1]) == ARGV[1] then
					return redis.call("PEXPIRE", KEYS[1], ARGV[2])
				else
					return 0
				end
			`)
			result, scriptErr := script.Run(ctx, m.client, []string{redisKey}, owner, ttl.Milliseconds()).Result()
			if scriptErr != nil {
				return nil, false, fmt.Errorf("extend on re-acquire: %w", scriptErr)
			}

			count, isInt := result.(int64)
			if !isInt || count == 0 {
				// Lock was lost or expired, the polling loop in Acquire will retry
				return nil, false, nil
			}

			lock := &redisLock{
				key:       key,
				owner:     owner,
				expiresAt: time.Now().Add(ttl),
				manager:   m,
			}
			return lock, true, nil
		}

		// Lock held by someone else
		return nil, false, nil
	}
}

// Release releases a lock.
func (m *RedisLockManager) Release(ctx context.Context, key, owner string) error {
	redisKey := lockKeyPrefix + key

	// Lua script to atomically check owner and delete
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, m.client, []string{redisKey}, owner).Result()
	if err != nil {
		return fmt.Errorf("release script: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	if count == 0 {
		return ErrLockNotHeld
	}

	return nil
}

// GetLockInfo returns information about a lock.
func (m *RedisLockManager) GetLockInfo(ctx context.Context, key string) (*LockInfo, error) {
	redisKey := lockKeyPrefix + key

	// Get owner and TTL
	owner, err := m.client.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil //nolint:nilnil // nil info is valid for non-existent locks
	}
	if err != nil {
		return nil, fmt.Errorf("get: %w", err)
	}

	ttl, err := m.client.TTL(ctx, redisKey).Result()
	if err != nil {
		return nil, fmt.Errorf("ttl: %w", err)
	}

	// TTL returns -1 if no expiry, -2 if key doesn't exist
	if ttl < 0 {
		return nil, nil //nolint:nilnil // nil info is valid for expired locks
	}

	return &LockInfo{
		Key:       key,
		Owner:     owner,
		ExpiresAt: time.Now().Add(ttl),
	}, nil
}

// IsLocked returns true if the key is locked.
func (m *RedisLockManager) IsLocked(ctx context.Context, key string) (bool, error) {
	redisKey := lockKeyPrefix + key
	exists, err := m.client.Exists(ctx, redisKey).Result()
	if err != nil {
		return false, fmt.Errorf("exists: %w", err)
	}
	return exists > 0, nil
}

// redisLock implements the DistributedLock interface.
type redisLock struct {
	key       string
	owner     string
	expiresAt time.Time
	manager   *RedisLockManager
	released  bool
}

// Key returns the lock key.
func (l *redisLock) Key() string {
	return l.key
}

// Owner returns the lock owner.
func (l *redisLock) Owner() string {
	return l.owner
}

// ExpiresAt returns when the lock expires.
func (l *redisLock) ExpiresAt() time.Time {
	return l.expiresAt
}

// Unlock releases the lock.
func (l *redisLock) Unlock(ctx context.Context) error {
	if l.released {
		return nil
	}
	err := l.manager.Release(ctx, l.key, l.owner)
	if err == nil {
		l.released = true
	}
	return err
}

// Extend extends the lock TTL.
func (l *redisLock) Extend(ctx context.Context, duration time.Duration) error {
	if l.released {
		return ErrLockExpired
	}

	redisKey := lockKeyPrefix + l.key

	// Lua script to atomically check owner and extend
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.manager.client, []string{redisKey}, l.owner, duration.Milliseconds()).Result()
	if err != nil {
		return fmt.Errorf("extend script: %w", err)
	}

	count, ok := result.(int64)
	if !ok {
		return fmt.Errorf("unexpected result type: %T", result)
	}

	if count == 0 {
		return ErrLockNotHeld
	}

	l.expiresAt = time.Now().Add(duration)
	return nil
}

// IsHeld returns true if the lock is still held.
func (l *redisLock) IsHeld(ctx context.Context) (bool, error) {
	if l.released {
		return false, nil
	}

	redisKey := lockKeyPrefix + l.key
	owner, err := l.manager.client.Get(ctx, redisKey).Result()
	if errors.Is(err, redis.Nil) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get: %w", err)
	}

	return owner == l.owner, nil
}
