package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
)

// contextKey is a type for context keys.
type contextKey string

const (
	// UserContextKey is the context key for the authenticated user.
	UserContextKey contextKey = "authenticated_user"

	// authHeaderParts is the number of parts in a valid Authorization header.
	authHeaderParts = 2
	// bearerScheme is the authentication scheme for Bearer tokens.
	bearerScheme = "bearer"
)

// AuthMiddleware creates an HTTP middleware that validates authentication tokens.
type AuthMiddleware struct {
	authenticator security.Authenticator
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(authenticator security.Authenticator) *AuthMiddleware {
	return &AuthMiddleware{
		authenticator: authenticator,
	}
}

// Middleware creates an HTTP middleware that validates authentication.
func (am *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := util.Log(ctx)

		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			log.Debug("missing authorization header")
			am.unauthorized(w, "Missing authorization header")
			return
		}

		// Validate Bearer token format
		parts := strings.SplitN(authHeader, " ", authHeaderParts)
		if len(parts) != authHeaderParts || !strings.EqualFold(parts[0], bearerScheme) {
			log.Debug("invalid authorization header format")
			am.unauthorized(w, "Invalid authorization header format. Expected: Bearer <token>")
			return
		}

		token := parts[1]
		if token == "" {
			log.Debug("empty token")
			am.unauthorized(w, "Empty token")
			return
		}

		// Validate token with Frame's authenticator
		authCtx, err := am.authenticator.Authenticate(ctx, token)
		if err != nil {
			log.Debug("token validation failed", "error", err.Error())
			am.unauthorized(w, "Invalid or expired token")
			return
		}

		// Extract user identity from the authenticated context
		claims := security.ClaimsFromContext(authCtx)
		userID := ""
		if claims != nil {
			userID, _ = claims.GetSubject()
		}

		log.Info("authenticated request",
			"user_id", userID,
			"path", r.URL.Path,
		)

		// Continue with authenticated context
		next.ServeHTTP(w, r.WithContext(authCtx))
	})
}

// unauthorized writes an unauthorized response.
func (am *AuthMiddleware) unauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", `Bearer realm="feature-gateway"`)
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized","message":"` + message + `"}`))
}

// GetUserFromContext retrieves the authenticated user claims from context.
func GetUserFromContext(ctx context.Context) *security.AuthenticationClaims {
	return security.ClaimsFromContext(ctx)
}

// OptionalAuthMiddleware creates middleware that attempts authentication but doesn't require it.
func (am *AuthMiddleware) OptionalAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		log := util.Log(ctx)

		// Extract token from Authorization header if present
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			// No auth header - continue without authentication
			next.ServeHTTP(w, r)
			return
		}

		// Try to validate token
		parts := strings.SplitN(authHeader, " ", authHeaderParts)
		if len(parts) != authHeaderParts || !strings.EqualFold(parts[0], bearerScheme) {
			next.ServeHTTP(w, r)
			return
		}

		token := parts[1]
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}

		authCtx, err := am.authenticator.Authenticate(ctx, token)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}

		claims := security.ClaimsFromContext(authCtx)
		if claims != nil {
			userID, _ := claims.GetSubject()
			log.Info("authenticated request (optional)",
				"user_id", userID,
				"path", r.URL.Path,
			)
		}
		next.ServeHTTP(w, r.WithContext(authCtx))
	})
}
