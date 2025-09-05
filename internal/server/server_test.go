package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
	"github.com/stretchr/testify/assert"
)

func init() {
	// Use mock query engine for all server tests
	os.Setenv("QUERY_ENGINE_MOCK", "true")
}

func TestServer_HandleHealth(t *testing.T) {
	tests := []struct {
		name           string
		serverReady    bool
		expectedStatus int
		expectedReady  bool
	}{
		{
			name:           "server ready",
			serverReady:    true,
			expectedStatus: http.StatusOK,
			expectedReady:  true,
		},
		{
			name:           "server not ready",
			serverReady:    false,
			expectedStatus: http.StatusServiceUnavailable,
			expectedReady:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{AuthToken: "test-token"}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			server := New(cfg, logger)
			server.ready = tt.serverReady

			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()

			server.handleHealth(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response HealthResponse
			err := json.NewDecoder(w.Body).Decode(&response)
			assert.NoError(t, err)
			assert.Equal(t, "ok", response.Status)
			assert.Equal(t, tt.expectedReady, response.Ready)
		})
	}
}

func TestServer_HandleQuery_Authorization(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		expectedStatus int
	}{
		{
			name:           "valid token",
			authHeader:     "Bearer test-token",
			expectedStatus: http.StatusServiceUnavailable, // Server not ready, but auth passed
		},
		{
			name:           "missing token",
			authHeader:     "",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "invalid bearer format",
			authHeader:     "test-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "wrong token",
			authHeader:     "Bearer wrong-token",
			expectedStatus: http.StatusUnauthorized,
		},
		{
			name:           "empty bearer",
			authHeader:     "Bearer ",
			expectedStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{AuthToken: "test-token"}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			server := New(cfg, logger)
			// Don't set ready to true, so we can test auth without needing a real query engine

			reqBody := QueryRequest{
				Name:  "test",
				Limit: 10,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/query", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			w := httptest.NewRecorder()
			server.handleQuery(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestServer_HandleQuery_MethodValidation(t *testing.T) {
	cfg := &config.Config{AuthToken: "test-token"}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	server := New(cfg, logger)

	// Test GET method (should fail)
	req := httptest.NewRequest("GET", "/query", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()

	server.handleQuery(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestServer_HandleQuery_RequestParsing(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    string
		contentType    string
		expectedStatus int
	}{
		{
			name:           "valid json",
			requestBody:    `{"name":"test","limit":5}`,
			contentType:    "application/json",
			expectedStatus: http.StatusServiceUnavailable, // Server not ready
		},
		{
			name:           "invalid json",
			requestBody:    `{"name":"test"`,
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "empty body",
			requestBody:    "",
			contentType:    "application/json",
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{AuthToken: "test-token"}
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

			server := New(cfg, logger)

			// Set server as ready for tests that expect to test JSON parsing
			if tt.expectedStatus == http.StatusBadRequest {
				server.ready = true
			}

			req := httptest.NewRequest("POST", "/query", bytes.NewReader([]byte(tt.requestBody)))
			req.Header.Set("Authorization", "Bearer test-token")
			req.Header.Set("Content-Type", tt.contentType)

			w := httptest.NewRecorder()
			server.handleQuery(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestServer_IsAuthorized(t *testing.T) {
	cfg := &config.Config{AuthToken: "secret-token"}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	server := New(cfg, logger)

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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			result := server.isAuthorized(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestQueryRequest_LimitHandling(t *testing.T) {
	// This tests the logic that would be in handleQuery for limit validation
	tests := []struct {
		name          string
		inputLimit    int
		expectedLimit int
	}{
		{
			name:          "zero limit gets default",
			inputLimit:    0,
			expectedLimit: 10,
		},
		{
			name:          "normal limit unchanged",
			inputLimit:    5,
			expectedLimit: 5,
		},
		{
			name:          "max limit capped",
			inputLimit:    150,
			expectedLimit: 100,
		},
		{
			name:          "exactly max limit allowed",
			inputLimit:    100,
			expectedLimit: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := tt.inputLimit

			// Apply the same logic as in handleQuery
			if limit == 0 {
				limit = 10
			}
			if limit > 100 {
				limit = 100
			}

			assert.Equal(t, tt.expectedLimit, limit)
		})
	}
}

// Mock for testing query responses
type mockQueryEngine struct {
	products []query.Product
	err      error
}

func (m *mockQueryEngine) SearchProducts(ctx context.Context, name, brand string, limit int) ([]query.Product, error) {
	return m.products, m.err
}

func (m *mockQueryEngine) SearchByBarcode(ctx context.Context, barcode string) (*query.Product, error) {
	if len(m.products) > 0 {
		return &m.products[0], nil
	}
	return nil, m.err
}

func (m *mockQueryEngine) TestConnection(ctx context.Context) error {
	return m.err
}

func (m *mockQueryEngine) Close() error {
	return nil
}
