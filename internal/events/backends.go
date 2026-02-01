package events

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/pitabwire/util"
	"github.com/redis/go-redis/v9"
)

// BackendType represents the type of backend storage.
type BackendType string

// Backend type constants.
const (
	BackendMemory   BackendType = "memory"
	BackendRedis    BackendType = "redis"
	BackendPostgres BackendType = "postgres" // Not yet implemented
)

// BackendConfig contains configuration for backend storage.
type BackendConfig struct {
	// DeduplicationBackend is the backend for deduplication storage.
	DeduplicationBackend BackendType

	// LockingBackend is the backend for distributed locking.
	LockingBackend BackendType

	// SequencingBackend is the backend for sequence management.
	SequencingBackend BackendType

	// RedisURL is the Redis connection string.
	RedisURL string

	// DeduplicationTTL is the TTL for deduplication entries.
	DeduplicationTTL time.Duration
}

// DefaultBackendConfig returns the default configuration with in-memory backends.
func DefaultBackendConfig() BackendConfig {
	return BackendConfig{
		DeduplicationBackend: BackendMemory,
		LockingBackend:       BackendMemory,
		SequencingBackend:    BackendMemory,
		DeduplicationTTL:     defaultDedupTTL,
	}
}

// Backends holds all the backend implementations.
type Backends struct {
	Deduplication DeduplicationStore
	Locking       LockManager
	Sequencing    SequenceManager

	redisClient *redis.Client
}

// Close closes any resources held by the backends.
func (b *Backends) Close() error {
	if b.redisClient != nil {
		return b.redisClient.Close()
	}
	return nil
}

// NewBackends creates backend implementations based on the configuration.
func NewBackends(ctx context.Context, cfg BackendConfig) (*Backends, error) {
	log := util.Log(ctx)
	backends := &Backends{}

	// Initialize Redis client if needed
	needsRedis := cfg.DeduplicationBackend == BackendRedis ||
		cfg.LockingBackend == BackendRedis ||
		cfg.SequencingBackend == BackendRedis

	if needsRedis {
		if cfg.RedisURL == "" {
			return nil, errors.New("redis URL required when using redis backend")
		}

		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return nil, fmt.Errorf("parse redis URL: %w", err)
		}

		backends.redisClient = redis.NewClient(opts)

		// Test connection
		if pingErr := backends.redisClient.Ping(ctx).Err(); pingErr != nil {
			return nil, fmt.Errorf("redis ping: %w", pingErr)
		}

		log.Info("connected to Redis", "url", sanitizeRedisURL(cfg.RedisURL))
	}

	// Initialize deduplication store
	switch cfg.DeduplicationBackend {
	case BackendRedis:
		backends.Deduplication = NewRedisDeduplicationStore(backends.redisClient, cfg.DeduplicationTTL)
		log.Info("using Redis deduplication store")
	case BackendMemory:
		backends.Deduplication = NewInMemoryDeduplicationStore()
		log.Info("using in-memory deduplication store")
	case BackendPostgres:
		return nil, errors.New("postgres deduplication backend not yet implemented")
	}

	// Initialize lock manager
	switch cfg.LockingBackend {
	case BackendRedis:
		backends.Locking = NewRedisLockManager(backends.redisClient)
		log.Info("using Redis lock manager")
	case BackendMemory:
		backends.Locking = NewInMemoryLockManager()
		log.Info("using in-memory lock manager")
	case BackendPostgres:
		return nil, errors.New("postgres locking backend not yet implemented")
	}

	// Initialize sequence manager
	switch cfg.SequencingBackend {
	case BackendRedis:
		backends.Sequencing = NewRedisSequenceManager(backends.redisClient)
		log.Info("using Redis sequence manager")
	case BackendMemory:
		backends.Sequencing = NewInMemorySequenceManager()
		log.Info("using in-memory sequence manager")
	case BackendPostgres:
		return nil, errors.New("postgres sequencing backend not yet implemented")
	}

	return backends, nil
}

// NewBackendsWithFallback creates backends with fallback to in-memory if Redis fails.
func NewBackendsWithFallback(ctx context.Context, cfg BackendConfig) (*Backends, error) {
	log := util.Log(ctx)

	backends, err := NewBackends(ctx, cfg)
	if err != nil {
		// Log warning and fall back to in-memory
		log.Warn("falling back to in-memory backends", "error", err.Error())

		cfg.DeduplicationBackend = BackendMemory
		cfg.LockingBackend = BackendMemory
		cfg.SequencingBackend = BackendMemory

		return NewBackends(ctx, cfg)
	}

	return backends, nil
}

// sanitizeRedisURL removes password from Redis URL for logging.
func sanitizeRedisURL(url string) string {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return "[invalid]"
	}

	sanitized := fmt.Sprintf("redis://%s/%d", opts.Addr, opts.DB)
	if opts.Username != "" {
		sanitized = fmt.Sprintf("redis://%s@%s/%d", opts.Username, opts.Addr, opts.DB)
	}

	return sanitized
}

// HealthCheck performs a health check on all backends.
func (b *Backends) HealthCheck(ctx context.Context) error {
	if b.redisClient != nil {
		if err := b.redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis health check: %w", err)
		}
	}
	return nil
}
