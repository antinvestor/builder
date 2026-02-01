package events

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// Backoff configuration constants.
const (
	lockBaseBackoff    = 100 * time.Millisecond
	lockMaxBackoff     = 30 * time.Second
	lockJitterFraction = 0.3
	lockMaxAttemptCap  = 10 // Maximum exponent for backoff calculation
)

// Common locking errors.
var (
	ErrLockNotAcquired = errors.New("lock not acquired")
	ErrLockExpired     = errors.New("lock expired")
	ErrLockNotHeld     = errors.New("lock not held by caller")
	ErrLockExists      = errors.New("lock already exists")
)

// DistributedLock represents a distributed lock.
type DistributedLock interface {
	// Key returns the lock key.
	Key() string

	// Owner returns the lock owner.
	Owner() string

	// ExpiresAt returns when the lock expires.
	ExpiresAt() time.Time

	// Unlock releases the lock.
	Unlock(ctx context.Context) error

	// Extend extends the lock TTL.
	Extend(ctx context.Context, duration time.Duration) error

	// IsHeld returns true if the lock is still held.
	IsHeld(ctx context.Context) (bool, error)
}

// LockManager manages distributed locks.
type LockManager interface {
	// Acquire attempts to acquire a lock.
	// Returns ErrLockNotAcquired if the lock is held by another owner.
	Acquire(ctx context.Context, key string, owner string, ttl time.Duration) (DistributedLock, error)

	// TryAcquire attempts to acquire a lock without blocking.
	TryAcquire(ctx context.Context, key string, owner string, ttl time.Duration) (DistributedLock, bool, error)

	// Release releases a lock.
	Release(ctx context.Context, key string, owner string) error

	// GetLockInfo returns information about a lock.
	GetLockInfo(ctx context.Context, key string) (*LockInfo, error)

	// IsLocked returns true if the key is locked.
	IsLocked(ctx context.Context, key string) (bool, error)
}

// ExecutionLock provides execution-specific locking utilities.
type ExecutionLock struct {
	manager LockManager
}

// NewExecutionLock creates a new execution lock helper.
func NewExecutionLock(manager LockManager) *ExecutionLock {
	return &ExecutionLock{manager: manager}
}

// AcquireRepositoryBranchLock acquires an exclusive lock for a repository+branch.
// This ensures only one execution can modify a branch at a time.
func (e *ExecutionLock) AcquireRepositoryBranchLock(
	ctx context.Context,
	executionID ExecutionID,
	repositoryID string,
	branchName string,
	ttl time.Duration,
) (DistributedLock, error) {
	key := fmt.Sprintf("repo:%s:branch:%s", repositoryID, branchName)
	return e.manager.Acquire(ctx, key, executionID.String(), ttl)
}

// AcquireWorkspaceLock acquires an exclusive lock for a workspace path.
func (e *ExecutionLock) AcquireWorkspaceLock(
	ctx context.Context,
	executionID ExecutionID,
	workspacePath string,
	ttl time.Duration,
) (DistributedLock, error) {
	key := fmt.Sprintf("workspace:%s", workspacePath)
	return e.manager.Acquire(ctx, key, executionID.String(), ttl)
}

// AcquireExecutionLock acquires a lock for the entire execution.
// This prevents duplicate execution starts.
func (e *ExecutionLock) AcquireExecutionLock(
	ctx context.Context,
	executionID ExecutionID,
	ttl time.Duration,
) (DistributedLock, error) {
	key := fmt.Sprintf("execution:%s", executionID.String())
	return e.manager.Acquire(ctx, key, executionID.String(), ttl)
}

// InMemoryLockManager is an in-memory implementation for testing.
type InMemoryLockManager struct {
	mu        sync.RWMutex
	locks     map[string]*inMemoryLock
	stopCh    chan struct{}
	stoppedCh chan struct{}
}

type inMemoryLock struct {
	key       string
	owner     string
	expiresAt time.Time
	manager   *InMemoryLockManager
	released  bool
}

// NewInMemoryLockManager creates a new in-memory lock manager.
func NewInMemoryLockManager() *InMemoryLockManager {
	m := &InMemoryLockManager{
		locks:     make(map[string]*inMemoryLock),
		stopCh:    make(chan struct{}),
		stoppedCh: make(chan struct{}),
	}
	// Start cleanup goroutine
	go m.cleanupExpired()
	return m
}

// Close stops the lock manager's cleanup goroutine gracefully.
func (m *InMemoryLockManager) Close() error {
	close(m.stopCh)
	<-m.stoppedCh
	return nil
}

func (m *InMemoryLockManager) cleanupExpired() {
	defer close(m.stoppedCh)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.mu.Lock()
			now := time.Now()
			for key, lock := range m.locks {
				if lock.expiresAt.Before(now) {
					delete(m.locks, key)
				}
			}
			m.mu.Unlock()
		}
	}
}

// Acquire attempts to acquire a lock with exponential backoff.
func (m *InMemoryLockManager) Acquire(ctx context.Context, key string, owner string, ttl time.Duration) (DistributedLock, error) {
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}

	attempt := 0
	for {
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

		// Calculate exponential backoff with jitter
		backoff := calculateLockBackoff(attempt)
		attempt++

		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
			// Continue to next attempt
		}
	}
}

// calculateLockBackoff computes backoff duration with exponential increase and jitter.
func calculateLockBackoff(attempt int) time.Duration {
	// Exponential backoff: base * 2^attempt
	backoff := min(
		lockBaseBackoff*time.Duration(1<<min(attempt, lockMaxAttemptCap)),
		lockMaxBackoff,
	)

	// Add jitter: +/- jitterFraction * backoff
	// Using math/rand/v2 is acceptable for jitter as it's not security-critical.
	jitterRange := float64(backoff) * lockJitterFraction
	jitter := time.Duration((rand.Float64()*2 - 1) * jitterRange) //nolint:gosec // jitter doesn't need cryptographic randomness

	return backoff + jitter
}

// TryAcquire attempts to acquire a lock without blocking.
func (m *InMemoryLockManager) TryAcquire(ctx context.Context, key string, owner string, ttl time.Duration) (DistributedLock, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// Check if lock exists and is still valid
	existing, ok := m.locks[key]
	if ok {
		if existing.expiresAt.After(now) {
			// Lock is held
			if existing.owner == owner {
				// We already own it, extend
				existing.expiresAt = now.Add(ttl)
				return existing, true, nil
			}
			return nil, false, nil
		}
		// Lock expired, can acquire
	}

	// Acquire the lock
	lock := &inMemoryLock{
		key:       key,
		owner:     owner,
		expiresAt: now.Add(ttl),
		manager:   m,
	}
	m.locks[key] = lock
	return lock, true, nil
}

// Release releases a lock.
func (m *InMemoryLockManager) Release(ctx context.Context, key string, owner string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locks[key]
	if !ok {
		return nil // Already released
	}

	if existing.owner != owner {
		return ErrLockNotHeld
	}

	delete(m.locks, key)
	return nil
}

// GetLockInfo returns information about a lock.
func (m *InMemoryLockManager) GetLockInfo(ctx context.Context, key string) (*LockInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, ok := m.locks[key]
	if !ok {
		return nil, nil
	}

	if lock.expiresAt.Before(time.Now()) {
		return nil, nil
	}

	return &LockInfo{
		Key:        lock.key,
		Owner:      lock.owner,
		AcquiredAt: lock.expiresAt.Add(-time.Since(lock.expiresAt)), // Approximate
		ExpiresAt:  lock.expiresAt,
	}, nil
}

// IsLocked returns true if the key is locked.
func (m *InMemoryLockManager) IsLocked(ctx context.Context, key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	lock, ok := m.locks[key]
	if !ok {
		return false, nil
	}

	return lock.expiresAt.After(time.Now()), nil
}

// Key returns the lock key.
func (l *inMemoryLock) Key() string {
	return l.key
}

// Owner returns the lock owner.
func (l *inMemoryLock) Owner() string {
	return l.owner
}

// ExpiresAt returns when the lock expires.
func (l *inMemoryLock) ExpiresAt() time.Time {
	return l.expiresAt
}

// Unlock releases the lock.
func (l *inMemoryLock) Unlock(ctx context.Context) error {
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
func (l *inMemoryLock) Extend(ctx context.Context, duration time.Duration) error {
	l.manager.mu.Lock()
	defer l.manager.mu.Unlock()

	existing, ok := l.manager.locks[l.key]
	if !ok {
		return ErrLockExpired
	}

	if existing.owner != l.owner {
		return ErrLockNotHeld
	}

	if existing.expiresAt.Before(time.Now()) {
		return ErrLockExpired
	}

	existing.expiresAt = time.Now().Add(duration)
	l.expiresAt = existing.expiresAt
	return nil
}

// IsHeld returns true if the lock is still held.
func (l *inMemoryLock) IsHeld(ctx context.Context) (bool, error) {
	l.manager.mu.RLock()
	defer l.manager.mu.RUnlock()

	existing, ok := l.manager.locks[l.key]
	if !ok {
		return false, nil
	}

	return existing.owner == l.owner && existing.expiresAt.After(time.Now()), nil
}

// LockGuard provides RAII-style lock management.
type LockGuard struct {
	lock     DistributedLock
	released bool
}

// NewLockGuard creates a new lock guard.
func NewLockGuard(lock DistributedLock) *LockGuard {
	return &LockGuard{lock: lock}
}

// Release releases the lock.
func (g *LockGuard) Release(ctx context.Context) error {
	if g.released {
		return nil
	}
	err := g.lock.Unlock(ctx)
	if err == nil {
		g.released = true
	}
	return err
}

// Extend extends the lock.
func (g *LockGuard) Extend(ctx context.Context, duration time.Duration) error {
	if g.released {
		return ErrLockExpired
	}
	return g.lock.Extend(ctx, duration)
}

// WithLock executes a function while holding a lock.
func WithLock(ctx context.Context, manager LockManager, key, owner string, ttl time.Duration, fn func(ctx context.Context) error) error {
	lock, err := manager.Acquire(ctx, key, owner, ttl)
	if err != nil {
		return fmt.Errorf("acquire lock: %w", err)
	}
	defer lock.Unlock(ctx)

	return fn(ctx)
}

// LockExtender automatically extends locks while work is ongoing.
type LockExtender struct {
	lock      DistributedLock
	interval  time.Duration
	extension time.Duration
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewLockExtender creates a lock extender that automatically renews the lock.
func NewLockExtender(lock DistributedLock, interval, extension time.Duration) *LockExtender {
	return &LockExtender{
		lock:      lock,
		interval:  interval,
		extension: extension,
		done:      make(chan struct{}),
	}
}

// Start begins automatic lock extension.
func (e *LockExtender) Start(ctx context.Context) {
	ctx, e.cancel = context.WithCancel(ctx)

	go func() {
		defer close(e.done)
		ticker := time.NewTicker(e.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := e.lock.Extend(ctx, e.extension); err != nil {
					// Log error but continue - the lock might have expired
					return
				}
			}
		}
	}()
}

// Stop stops automatic lock extension and releases the lock.
func (e *LockExtender) Stop(ctx context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	<-e.done
	return e.lock.Unlock(ctx)
}
