package mcpgo

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/auth"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/noot-app/openfoodfacts-mcp-server/internal/query"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_checkHealthWithCache(t *testing.T) {
	t.Run("first call performs health check", func(t *testing.T) {
		logger := config.NewTestLogger(io.Discard, "debug")
		mockEngine := query.NewMockEngine(logger)
		auth := auth.NewBearerTokenAuth("test-token")

		server := NewServer(mockEngine, auth, logger)
		ctx := context.Background()

		// First call should perform actual health check
		err := server.checkHealthWithCache(ctx)
		assert.NoError(t, err)

		// Verify that the cache was updated
		assert.False(t, server.lastHealthCheck.IsZero())
		assert.NoError(t, server.lastHealthError)
	})

	t.Run("subsequent calls within 10 seconds use cache", func(t *testing.T) {
		logger := config.NewTestLogger(io.Discard, "debug")
		mockEngine := query.NewMockEngine(logger)
		auth := auth.NewBearerTokenAuth("test-token")

		server := NewServer(mockEngine, auth, logger)
		ctx := context.Background()

		// First call
		err1 := server.checkHealthWithCache(ctx)
		assert.NoError(t, err1)
		firstCheckTime := server.lastHealthCheck

		// Second call immediately after should use cache
		err2 := server.checkHealthWithCache(ctx)
		assert.NoError(t, err2)

		// Verify the timestamp didn't change (cache was used)
		assert.Equal(t, firstCheckTime, server.lastHealthCheck)
	})

	t.Run("caches error results", func(t *testing.T) {
		logger := config.NewTestLogger(io.Discard, "debug")
		mockEngine := query.NewMockEngine(logger)
		testError := errors.New("database connection failed")
		mockEngine.SetError(testError)

		auth := auth.NewBearerTokenAuth("test-token")

		server := NewServer(mockEngine, auth, logger)
		ctx := context.Background()

		// First call should get error and cache it
		err1 := server.checkHealthWithCache(ctx)
		assert.Error(t, err1)
		assert.Equal(t, testError, err1)
		assert.Equal(t, testError, server.lastHealthError)

		// Fix the mock engine
		mockEngine.SetError(nil)

		// Second call should still return cached error
		err2 := server.checkHealthWithCache(ctx)
		assert.Error(t, err2)
		assert.Equal(t, testError, err2)
	})

	t.Run("cache expires after 10 seconds", func(t *testing.T) {
		logger := config.NewTestLogger(io.Discard, "debug")
		mockEngine := query.NewMockEngine(logger)
		auth := auth.NewBearerTokenAuth("test-token")

		server := NewServer(mockEngine, auth, logger)
		ctx := context.Background()

		// First call
		err1 := server.checkHealthWithCache(ctx)
		assert.NoError(t, err1)

		// Manually set the cache time to 11 seconds ago
		server.lastHealthCheck = time.Now().Add(-11 * time.Second)

		// Next call should perform a new health check
		err2 := server.checkHealthWithCache(ctx)
		assert.NoError(t, err2)

		// Verify new timestamp is recent (within last second)
		assert.True(t, time.Since(server.lastHealthCheck) < time.Second)
	})

	t.Run("concurrent calls handle race conditions safely", func(t *testing.T) {
		logger := config.NewTestLogger(io.Discard, "debug")
		mockEngine := query.NewMockEngine(logger)
		auth := auth.NewBearerTokenAuth("test-token")

		server := NewServer(mockEngine, auth, logger)
		ctx := context.Background()

		// Set cache as expired
		server.lastHealthCheck = time.Now().Add(-11 * time.Second)

		// Run multiple concurrent health checks
		errChan := make(chan error, 10)
		for i := 0; i < 10; i++ {
			go func() {
				err := server.checkHealthWithCache(ctx)
				errChan <- err
			}()
		}

		// Collect all results
		var errors []error
		for i := 0; i < 10; i++ {
			errors = append(errors, <-errChan)
		}

		// All should succeed
		for _, err := range errors {
			assert.NoError(t, err)
		}

		// Cache should be updated
		assert.True(t, time.Since(server.lastHealthCheck) < time.Second)
	})
}

func TestServer_HealthCacheIntegration(t *testing.T) {
	t.Run("health endpoint uses cached results", func(t *testing.T) {
		logger := config.NewTestLogger(io.Discard, "debug")
		mockEngine := query.NewMockEngine(logger)
		auth := auth.NewBearerTokenAuth("test-token")

		server := NewServer(mockEngine, auth, logger)

		// Pre-populate cache with successful result
		ctx := context.Background()
		err := server.checkHealthWithCache(ctx)
		require.NoError(t, err)

		// Now break the mock engine
		mockEngine.SetError(errors.New("database is down"))

		// Health check should still return success due to cache
		err = server.checkHealthWithCache(ctx)
		assert.NoError(t, err, "Should use cached success result even though mock engine is now broken")
	})
}
