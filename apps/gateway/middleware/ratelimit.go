package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pitabwire/util"
	"golang.org/x/time/rate"
)

const (
	cleanupInterval    = 5 * time.Minute
	secondsPerMinute   = 60.0
	apiKeyHeader       = "X-Api-Key" //nolint:gosec // This is a header name, not a credential
	xForwardedForHdr   = "X-Forwarded-For"
	staleClientMinutes = 10
)

// RateLimiter is a token bucket rate limiter that tracks clients by IP.
type RateLimiter struct {
	clients     map[string]*clientLimiter
	mu          sync.RWMutex
	ratePerMin  int
	burstSize   int
	cleanupTick time.Duration
	stopCleanup chan struct{}
}

// clientLimiter tracks a client's rate limiter and last access time.
type clientLimiter struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(requestsPerMinute, burstSize int) *RateLimiter {
	rl := &RateLimiter{
		clients:     make(map[string]*clientLimiter),
		ratePerMin:  requestsPerMinute,
		burstSize:   burstSize,
		cleanupTick: cleanupInterval,
		stopCleanup: make(chan struct{}),
	}

	// Start cleanup goroutine to remove stale entries
	go rl.cleanupLoop()

	return rl
}

// Stop stops the rate limiter's cleanup goroutine.
func (rl *RateLimiter) Stop() {
	close(rl.stopCleanup)
}

// getClientLimiter retrieves or creates a rate limiter for a client.
func (rl *RateLimiter) getClientLimiter(clientID string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	if client, exists := rl.clients[clientID]; exists {
		client.lastAccess = time.Now()
		return client.limiter
	}

	// Calculate rate as requests per second
	ratePerSec := float64(rl.ratePerMin) / secondsPerMinute
	limiter := rate.NewLimiter(rate.Limit(ratePerSec), rl.burstSize)

	rl.clients[clientID] = &clientLimiter{
		limiter:    limiter,
		lastAccess: time.Now(),
	}

	return limiter
}

// cleanupLoop periodically removes stale client limiters.
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.cleanupTick)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stopCleanup:
			return
		}
	}
}

// cleanup removes client limiters that haven't been accessed recently.
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	staleThreshold := time.Now().Add(-staleClientMinutes * time.Minute)
	for clientID, client := range rl.clients {
		if client.lastAccess.Before(staleThreshold) {
			delete(rl.clients, clientID)
		}
	}
}

// Allow checks if a request from the given client is allowed.
func (rl *RateLimiter) Allow(clientID string) bool {
	limiter := rl.getClientLimiter(clientID)
	return limiter.Allow()
}

// getClientID extracts a unique identifier for the client.
// Uses X-Forwarded-For if behind a proxy, otherwise the remote address.
func getClientID(r *http.Request) string {
	// Check for X-Api-Key header first
	if apiKey := r.Header.Get(apiKeyHeader); apiKey != "" {
		return "apikey:" + apiKey
	}

	// Check for X-Forwarded-For (behind proxy/load balancer)
	if xff := r.Header.Get(xForwardedForHdr); xff != "" {
		// X-Forwarded-For can contain multiple IPs (client, proxy1, proxy2), use the first one
		ips := strings.Split(xff, ",")
		firstIP := strings.TrimSpace(ips[0])
		// The firstIP might still have a port, so we try to split it
		if host, _, err := net.SplitHostPort(firstIP); err == nil {
			return "ip:" + host
		}
		return "ip:" + firstIP
	}

	// Use remote address
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "ip:" + r.RemoteAddr
	}
	return "ip:" + host
}

// calculateRetryAfter calculates when the client can retry.
func (rl *RateLimiter) calculateRetryAfter(clientID string) int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()

	client, exists := rl.clients[clientID]
	if !exists {
		return 1
	}

	reservation := client.limiter.Reserve()
	delay := reservation.Delay()
	reservation.Cancel()

	if delay <= 0 {
		return 1
	}
	return int(delay.Seconds()) + 1
}

// Middleware creates an HTTP middleware that applies rate limiting.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := util.Log(ctx)

		clientID := getClientID(r)

		if !rl.Allow(clientID) {
			retryAfter := rl.calculateRetryAfter(clientID)

			log.Warn("rate limit exceeded",
				"client_id", clientID,
				"path", r.URL.Path,
				"retry_after", retryAfter,
			)

			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			w.WriteHeader(http.StatusTooManyRequests)
			response := map[string]any{
				"error":       "rate_limit_exceeded",
				"message":     "Too many requests. Please retry after " + strconv.Itoa(retryAfter) + " seconds.",
				"retry_after": retryAfter,
			}
			_ = json.NewEncoder(w).Encode(response)
			return
		}

		next.ServeHTTP(w, r)
	})
}
