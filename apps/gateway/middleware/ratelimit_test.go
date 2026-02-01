//nolint:testpackage // Tests require access to internal fields for cleanup and client state verification
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	defer rl.Stop()

	assert.NotNil(t, rl)
	assert.Equal(t, 60, rl.ratePerMin)
	assert.Equal(t, 10, rl.burstSize)
	assert.NotNil(t, rl.clients)
}

func TestRateLimiter_Allow(t *testing.T) {
	// Create limiter with 10 requests per minute, burst of 5
	rl := NewRateLimiter(10, 5)
	defer rl.Stop()

	clientID := "test-client"

	// First 5 requests should be allowed (burst)
	for i := range 5 {
		assert.True(t, rl.Allow(clientID), "request %d should be allowed", i+1)
	}

	// 6th request should be rate limited (burst exhausted)
	assert.False(t, rl.Allow(clientID), "request after burst should be rate limited")
}

func TestRateLimiter_DifferentClients(t *testing.T) {
	rl := NewRateLimiter(10, 2)
	defer rl.Stop()

	// Each client gets their own bucket
	assert.True(t, rl.Allow("client1"))
	assert.True(t, rl.Allow("client1"))
	assert.False(t, rl.Allow("client1")) // Exhausted

	// Different client still has quota
	assert.True(t, rl.Allow("client2"))
	assert.True(t, rl.Allow("client2"))
	assert.False(t, rl.Allow("client2"))
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := NewRateLimiter(60, 2)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	middleware := rl.Middleware(handler)

	// First 2 requests should pass (burst)
	for i := range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code, "request %d should succeed", i+1)
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Contains(t, rr.Body.String(), "rate_limit_exceeded")
	assert.NotEmpty(t, rr.Header().Get("Retry-After"))
}

func TestRateLimiter_MiddlewareWithAPIKey(t *testing.T) {
	rl := NewRateLimiter(60, 2)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	// Requests with same API key share rate limit
	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Api-Key", "test-key")
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// 3rd request with same API key should be limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Api-Key", "test-key")
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)

	// Different API key should have separate quota
	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	req2.Header.Set("X-Api-Key", "different-key")
	req2.RemoteAddr = "192.168.1.1:12345"
	rr2 := httptest.NewRecorder()

	middleware.ServeHTTP(rr2, req2)
	assert.Equal(t, http.StatusOK, rr2.Code)
}

func TestRateLimiter_MiddlewareWithXForwardedFor(t *testing.T) {
	rl := NewRateLimiter(60, 2)
	defer rl.Stop()

	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := rl.Middleware(handler)

	// Use X-Forwarded-For for client identification
	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Forwarded-For", "10.0.0.1")
		req.RemoteAddr = "192.168.1.1:12345"
		rr := httptest.NewRecorder()

		middleware.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
	}

	// 3rd request from same X-Forwarded-For should be limited
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1")
	req.RemoteAddr = "192.168.1.1:12345"
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
}

func TestGetClientID(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*http.Request)
		expected string
	}{
		{
			name: "with API key",
			setup: func(r *http.Request) {
				r.Header.Set("X-Api-Key", "my-api-key")
				r.RemoteAddr = "192.168.1.1:12345"
			},
			expected: "apikey:my-api-key",
		},
		{
			name: "with X-Forwarded-For",
			setup: func(r *http.Request) {
				r.Header.Set("X-Forwarded-For", "10.0.0.1")
				r.RemoteAddr = "192.168.1.1:12345"
			},
			expected: "ip:10.0.0.1",
		},
		{
			name: "with remote address port",
			setup: func(r *http.Request) {
				r.RemoteAddr = "192.168.1.1:54321"
			},
			expected: "ip:192.168.1.1",
		},
		{
			name: "with remote address no port",
			setup: func(r *http.Request) {
				r.RemoteAddr = "192.168.1.1"
			},
			expected: "ip:192.168.1.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			tt.setup(req)
			assert.Equal(t, tt.expected, getClientID(req))
		})
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	defer rl.Stop()

	// Add a client
	rl.Allow("test-client")

	// Verify client exists
	rl.mu.RLock()
	_, exists := rl.clients["test-client"]
	rl.mu.RUnlock()
	require.True(t, exists)

	// Set last access to old time
	rl.mu.Lock()
	if client, ok := rl.clients["test-client"]; ok {
		client.lastAccess = time.Now().Add(-15 * time.Minute)
	}
	rl.mu.Unlock()

	// Trigger cleanup
	rl.cleanup()

	// Verify client was removed
	rl.mu.RLock()
	_, exists = rl.clients["test-client"]
	rl.mu.RUnlock()
	assert.False(t, exists, "stale client should be removed")
}

func TestRateLimiter_CalculateRetryAfter(t *testing.T) {
	rl := NewRateLimiter(60, 1)
	defer rl.Stop()

	clientID := "test-client"

	// Exhaust the burst
	rl.Allow(clientID)
	rl.Allow(clientID)

	retryAfter := rl.calculateRetryAfter(clientID)
	assert.GreaterOrEqual(t, retryAfter, 1, "retry-after should be at least 1 second")
}

func TestRateLimiter_CalculateRetryAfter_UnknownClient(t *testing.T) {
	rl := NewRateLimiter(60, 10)
	defer rl.Stop()

	// Unknown client should return 1
	retryAfter := rl.calculateRetryAfter("unknown-client")
	assert.Equal(t, 1, retryAfter)
}
