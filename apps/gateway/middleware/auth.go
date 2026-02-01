package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/pitabwire/frame/security"
	"github.com/pitabwire/util"
)

const (
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
		token, ok := extractBearerToken(r.Header.Get("Authorization"))
		if !ok {
			log.Debug("missing or invalid authorization header")
			am.unauthorized(w, "Missing or invalid authorization header. Expected: Bearer <token>")
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
		userID := getUserIDFromClaims(authCtx, claims)

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
	response := map[string]string{
		"error":   "unauthorized",
		"message": message,
	}
	_ = json.NewEncoder(w).Encode(response)
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
		token, ok := extractBearerToken(r.Header.Get("Authorization"))
		if !ok {
			// No valid auth header - continue without authentication
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
			userID := getUserIDFromClaims(authCtx, claims)
			log.Info("authenticated request (optional)",
				"user_id", userID,
				"path", r.URL.Path,
			)
		}
		next.ServeHTTP(w, r.WithContext(authCtx))
	})
}

// extractBearerToken extracts the token from an Authorization header.
// Returns the token and true if extraction was successful, or empty string and false otherwise.
func extractBearerToken(authHeader string) (string, bool) {
	if authHeader == "" {
		return "", false
	}

	parts := strings.SplitN(authHeader, " ", authHeaderParts)
	if len(parts) != authHeaderParts || !strings.EqualFold(parts[0], bearerScheme) {
		return "", false
	}

	token := parts[1]
	if token == "" {
		return "", false
	}

	return token, true
}

// getUserIDFromClaims safely extracts the user ID from claims.
func getUserIDFromClaims(ctx context.Context, claims *security.AuthenticationClaims) string {
	if claims == nil {
		return ""
	}
	userID, err := claims.GetSubject()
	if err != nil {
		util.Log(ctx).Warn("could not get subject from claims", "error", err)
		return ""
	}
	return userID
}
