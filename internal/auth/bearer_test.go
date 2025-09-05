package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBearerTokenAuth_IsAuthorized(t *testing.T) {
	auth := NewBearerTokenAuth("secret-token")

	tests := []struct {
		name       string
		authHeader string
		expected   bool
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer secret-token",
			expected:   true,
		},
		{
			name:       "invalid token",
			authHeader: "Bearer wrong-token",
			expected:   false,
		},
		{
			name:       "missing bearer prefix",
			authHeader: "secret-token",
			expected:   false,
		},
		{
			name:       "empty header",
			authHeader: "",
			expected:   false,
		},
		{
			name:       "only bearer",
			authHeader: "Bearer",
			expected:   false,
		},
		{
			name:       "bearer with space only",
			authHeader: "Bearer ",
			expected:   false,
		},
		{
			name:       "case sensitive token",
			authHeader: "Bearer SECRET-TOKEN",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := auth.IsAuthorized(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBearerTokenAuth_SetUnauthorizedHeaders(t *testing.T) {
	auth := NewBearerTokenAuth("test-token")
	w := httptest.NewRecorder()

	auth.SetUnauthorizedHeaders(w)

	assert.Equal(t, "Bearer", w.Header().Get("WWW-Authenticate"))
}
