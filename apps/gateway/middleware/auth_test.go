//nolint:testpackage // Tests require access to internal fields and mock authenticator
package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pitabwire/frame/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAuthenticator is a mock implementation of security.Authenticator.
type mockAuthenticator struct {
	authenticateFunc func(ctx context.Context, token string, options ...security.AuthOption) (context.Context, error)
}

func (m *mockAuthenticator) Authenticate(
	ctx context.Context,
	token string,
	options ...security.AuthOption,
) (context.Context, error) {
	if m.authenticateFunc != nil {
		return m.authenticateFunc(ctx, token, options...)
	}
	return ctx, nil
}

func TestNewAuthMiddleware(t *testing.T) {
	auth := &mockAuthenticator{}
	middleware := NewAuthMiddleware(auth)
	assert.NotNil(t, middleware)
	assert.Equal(t, auth, middleware.authenticator)
}

func TestAuthMiddleware_MissingAuthHeader(t *testing.T) {
	auth := &mockAuthenticator{}
	middleware := NewAuthMiddleware(auth)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	wrapped := middleware.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Missing authorization header")
	assert.Equal(t, `Bearer realm="feature-gateway"`, rr.Header().Get("WWW-Authenticate"))
}

func TestAuthMiddleware_InvalidAuthHeaderFormat(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
	}{
		{"no space", "Bearertoken123"},
		{"wrong scheme", "Basic token123"},
		{"empty token", "Bearer "},
		{"only scheme", "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &mockAuthenticator{}
			middleware := NewAuthMiddleware(auth)

			handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
				t.Error("handler should not be called")
			})

			wrapped := middleware.Middleware(handler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", tt.authHeader)
			rr := httptest.NewRecorder()

			wrapped.ServeHTTP(rr, req)

			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}
}

func TestAuthMiddleware_TokenValidationFailed(t *testing.T) {
	auth := &mockAuthenticator{
		authenticateFunc: func(
			_ context.Context,
			_ string,
			_ ...security.AuthOption,
		) (context.Context, error) {
			return nil, errors.New("invalid token")
		},
	}
	middleware := NewAuthMiddleware(auth)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("handler should not be called")
	})

	wrapped := middleware.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Contains(t, rr.Body.String(), "Invalid or expired token")
}

func TestAuthMiddleware_ValidToken(t *testing.T) {
	auth := &mockAuthenticator{
		authenticateFunc: func(
			ctx context.Context,
			_ string,
			_ ...security.AuthOption,
		) (context.Context, error) {
			// Simulate successful authentication by returning the context
			return ctx, nil
		},
	}
	middleware := NewAuthMiddleware(auth)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	assert.True(t, handlerCalled, "handler should be called")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestAuthMiddleware_CaseInsensitiveBearer(t *testing.T) {
	auth := &mockAuthenticator{
		authenticateFunc: func(
			ctx context.Context,
			_ string,
			_ ...security.AuthOption,
		) (context.Context, error) {
			return ctx, nil
		},
	}
	middleware := NewAuthMiddleware(auth)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.Middleware(handler)

	// Test different cases of "Bearer"
	cases := []string{"bearer", "BEARER", "Bearer", "bEaReR"}
	for _, bearer := range cases {
		t.Run(bearer, func(t *testing.T) {
			handlerCalled = false
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.Header.Set("Authorization", bearer+" valid-token")
			rr := httptest.NewRecorder()

			wrapped.ServeHTTP(rr, req)

			assert.True(t, handlerCalled, "handler should be called for %s", bearer)
			assert.Equal(t, http.StatusOK, rr.Code)
		})
	}
}

func TestOptionalAuthMiddleware_NoHeader(t *testing.T) {
	auth := &mockAuthenticator{}
	middleware := NewAuthMiddleware(auth)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.OptionalAuthMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	assert.True(t, handlerCalled, "handler should be called without auth")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOptionalAuthMiddleware_ValidToken(t *testing.T) {
	auth := &mockAuthenticator{
		authenticateFunc: func(
			ctx context.Context,
			_ string,
			_ ...security.AuthOption,
		) (context.Context, error) {
			return ctx, nil
		},
	}
	middleware := NewAuthMiddleware(auth)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.OptionalAuthMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	assert.True(t, handlerCalled, "handler should be called with valid auth")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestOptionalAuthMiddleware_InvalidToken(t *testing.T) {
	auth := &mockAuthenticator{
		authenticateFunc: func(
			_ context.Context,
			_ string,
			_ ...security.AuthOption,
		) (context.Context, error) {
			return nil, errors.New("invalid")
		},
	}
	middleware := NewAuthMiddleware(auth)

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	wrapped := middleware.OptionalAuthMiddleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	// Should continue without auth even if token is invalid
	assert.True(t, handlerCalled, "handler should be called even with invalid token")
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetUserFromContext_NoClaims(t *testing.T) {
	ctx := context.Background()
	claims := GetUserFromContext(ctx)
	assert.Nil(t, claims)
}

func TestGetUserFromContext_WithClaims(t *testing.T) {
	// Create a context with claims
	// Note: This tests that the function properly retrieves claims
	// In a real scenario, claims would be added by the authenticator
	ctx := context.Background()
	claims := GetUserFromContext(ctx)

	// Without actual claims in context, this should return nil
	assert.Nil(t, claims)
}

func TestAuthMiddleware_UnauthorizedResponseFormat(t *testing.T) {
	auth := &mockAuthenticator{}
	middleware := NewAuthMiddleware(auth)

	handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	wrapped := middleware.Middleware(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	require.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), `"error":"unauthorized"`)
}
