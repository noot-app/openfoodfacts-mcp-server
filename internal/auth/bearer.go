package auth

import (
	"net/http"
	"strings"
)

// BearerTokenAuth handles Bearer token authentication
type BearerTokenAuth struct {
	token string
}

// NewBearerTokenAuth creates a new Bearer token authenticator
func NewBearerTokenAuth(token string) *BearerTokenAuth {
	return &BearerTokenAuth{token: token}
}

// IsAuthorized validates Bearer token from Authorization header
func (b *BearerTokenAuth) IsAuthorized(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	const bearerPrefix = "Bearer "
	if !strings.HasPrefix(authHeader, bearerPrefix) {
		return false
	}

	token := strings.TrimPrefix(authHeader, bearerPrefix)
	if token == "" {
		return false
	}

	return token == b.token
}

// SetUnauthorizedHeaders sets standard WWW-Authenticate header for Bearer auth
func (b *BearerTokenAuth) SetUnauthorizedHeaders(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
}
