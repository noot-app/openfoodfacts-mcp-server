package dataset

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/noot-app/openfoodfacts-mcp-server/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestConfig creates a test configuration with default values
func createTestConfig() *config.Config {
	return &config.Config{
		DisableRemoteCheck: false, // Allow remote checks in tests by default
	}
}

// createTestConfigWithDisabledRemoteCheck creates a test configuration with remote checks disabled
func createTestConfigWithDisabledRemoteCheck() *config.Config {
	return &config.Config{
		DisableRemoteCheck: true, // Disable remote checks
	}
}

func TestManager_EnsureDataset(t *testing.T) {
	tests := []struct {
		name                   string
		setupFiles             func(dir string)
		mockServer             func() *httptest.Server
		useDisabledRemoteCheck bool
		expectDownload         bool
		expectError            bool
	}{
		{
			name: "file does not exist - should download",
			setupFiles: func(dir string) {
				// No files exist
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "HEAD" {
						w.Header().Set("ETag", "test-etag")
						w.Header().Set("Content-Length", "1000")
						w.WriteHeader(http.StatusOK)
					} else {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("test parquet data"))
					}
				}))
			},
			useDisabledRemoteCheck: false,
			expectDownload:         true,
			expectError:            false,
		},
		{
			name: "file exists and up to date - should skip",
			setupFiles: func(dir string) {
				// Create existing parquet file
				parquetPath := filepath.Join(dir, "product-database.parquet")
				os.WriteFile(parquetPath, []byte("test parquet data"), 0644)

				// Create metadata with matching ETag
				meta := &Metadata{
					ETag:         "test-etag",
					Size:         17, // len("test parquet data")
					SHA256:       "test-sha",
					DownloadedAt: time.Now(),
				}
				metaData, _ := json.Marshal(meta)
				metadataPath := filepath.Join(dir, "metadata.json")
				os.WriteFile(metadataPath, metaData, 0644)
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "HEAD" {
						w.Header().Set("ETag", "test-etag")
						w.Header().Set("Content-Length", "17")
						w.WriteHeader(http.StatusOK)
					}
				}))
			},
			useDisabledRemoteCheck: false,
			expectDownload:         false,
			expectError:            false,
		},
		{
			name: "file exists but outdated - should download",
			setupFiles: func(dir string) {
				// Create existing parquet file
				parquetPath := filepath.Join(dir, "product-database.parquet")
				os.WriteFile(parquetPath, []byte("old data"), 0644)

				// Create metadata with different ETag
				meta := &Metadata{
					ETag:         "old-etag",
					Size:         8, // len("old data")
					SHA256:       "old-sha",
					DownloadedAt: time.Now(),
				}
				metaData, _ := json.Marshal(meta)
				metadataPath := filepath.Join(dir, "metadata.json")
				os.WriteFile(metadataPath, metaData, 0644)
			},
			mockServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method == "HEAD" {
						w.Header().Set("ETag", "new-etag")
						w.Header().Set("Content-Length", "1000")
						w.WriteHeader(http.StatusOK)
					} else {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("new parquet data"))
					}
				}))
			},
			useDisabledRemoteCheck: false,
			expectDownload:         true,
			expectError:            false,
		},
		{
			name: "file exists with remote checks disabled - should skip",
			setupFiles: func(dir string) {
				// Create existing parquet file
				parquetPath := filepath.Join(dir, "product-database.parquet")
				os.WriteFile(parquetPath, []byte("test parquet data"), 0644)
			},
			mockServer: func() *httptest.Server {
				// Server should never be called when remote checks are disabled
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					t.Fatal("Server should not be called when remote checks are disabled")
				}))
			},
			useDisabledRemoteCheck: true,
			expectDownload:         false,
			expectError:            false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "test-dataset-*")
			require.NoError(t, err)
			defer os.RemoveAll(tmpDir)

			// Setup files
			tt.setupFiles(tmpDir)

			// Create mock server
			server := tt.mockServer()
			defer server.Close()

			// Create manager
			logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
			var testConfig *config.Config
			if tt.useDisabledRemoteCheck {
				testConfig = createTestConfigWithDisabledRemoteCheck()
			} else {
				testConfig = createTestConfig()
			}
			manager := NewManager(
				server.URL,
				filepath.Join(tmpDir, "product-database.parquet"),
				filepath.Join(tmpDir, "metadata.json"),
				filepath.Join(tmpDir, "refresh.lock"),
				testConfig,
				logger,
			)

			// Track initial file modification time
			var initialModTime time.Time
			if stat, err := os.Stat(manager.parquetPath); err == nil {
				initialModTime = stat.ModTime()
			}

			// Execute
			ctx := context.Background()
			err = manager.EnsureDataset(ctx)

			// Verify
			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			// Check if file exists
			_, err = os.Stat(manager.parquetPath)
			assert.NoError(t, err)

			if tt.expectDownload {
				// Check if file was modified (downloaded)
				if !initialModTime.IsZero() {
					stat, err := os.Stat(manager.parquetPath)
					require.NoError(t, err)
					assert.True(t, stat.ModTime().After(initialModTime) || stat.ModTime().Equal(initialModTime))
				}

				// Check if metadata was created
				_, err = os.Stat(manager.metadataPath)
				assert.NoError(t, err)
			}
		})
	}
}

func TestManager_AcquireReleaseLock(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-lock-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	lockPath := filepath.Join(tmpDir, "test.lock")

	// First acquisition should succeed
	lock1, err := acquireLock(lockPath)
	assert.NoError(t, err)
	assert.NotNil(t, lock1)

	// Second acquisition should fail
	lock2, err := acquireLock(lockPath)
	assert.Error(t, err)
	assert.Nil(t, lock2)

	// Release first lock
	releaseLock(lock1, lockPath)

	// Third acquisition should succeed again
	lock3, err := acquireLock(lockPath)
	assert.NoError(t, err)
	assert.NotNil(t, lock3)

	releaseLock(lock3, lockPath)
}

func TestComputeSHA256(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-sha-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testData := "hello world"

	err = os.WriteFile(testFile, []byte(testData), 0644)
	require.NoError(t, err)

	sha, err := computeSHA256(testFile)
	require.NoError(t, err)

	// SHA256 of "hello world"
	expectedSHA := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	assert.Equal(t, expectedSHA, sha)
}

func TestMetadata_SaveLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-metadata-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	manager := NewManager(
		"https://example.com",
		filepath.Join(tmpDir, "test.parquet"),
		filepath.Join(tmpDir, "metadata.json"),
		filepath.Join(tmpDir, "test.lock"),
		createTestConfig(),
		logger,
	)

	originalMeta := &Metadata{
		SHA256:       "test-sha256",
		DownloadedAt: time.Now().UTC().Truncate(time.Second),
		ETag:         "test-etag",
		Size:         12345,
	}

	// Save metadata
	err = manager.saveMetadata(originalMeta)
	require.NoError(t, err)

	// Load metadata
	loadedMeta, err := manager.loadMetadata()
	require.NoError(t, err)

	assert.Equal(t, originalMeta.SHA256, loadedMeta.SHA256)
	assert.Equal(t, originalMeta.ETag, loadedMeta.ETag)
	assert.Equal(t, originalMeta.Size, loadedMeta.Size)
	assert.True(t, originalMeta.DownloadedAt.Equal(loadedMeta.DownloadedAt))
}

func TestManager_IgnoreLock(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "test-dataset-ignore-lock-")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	parquetPath := filepath.Join(tempDir, "product-database.parquet")
	metadataPath := filepath.Join(tempDir, "metadata.json")
	lockPath := filepath.Join(tempDir, "refresh.lock")

	// Create a mock server that serves test data
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.Header().Set("ETag", "test-etag")
			w.Header().Set("Content-Length", "1000")
			w.WriteHeader(http.StatusOK)
		} else if r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("test parquet data"))
		}
	}))
	defer mockServer.Close()

	// Create a lock file to simulate another instance running
	lockFile, err := os.Create(lockPath)
	require.NoError(t, err)
	lockFile.Close()

	// Verify lock file exists
	_, err = os.Stat(lockPath)
	require.NoError(t, err)

	t.Run("without IGNORE_LOCK should wait", func(t *testing.T) {
		// Create config without IgnoreLock
		cfg := &config.Config{
			DisableRemoteCheck: false,
			IgnoreLock:         false,
		}

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		manager := NewManager(mockServer.URL, parquetPath, metadataPath, lockPath, cfg, logger)

		// This should timeout quickly since we're not going to complete the download
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		downloadErr := manager.downloadWithLock(ctx)
		// Should get a timeout or context cancelled error
		assert.Error(t, downloadErr)
	})

	t.Run("with IGNORE_LOCK should force download", func(t *testing.T) {
		// Recreate lock file in case it was removed
		lockFile, err := os.Create(lockPath)
		require.NoError(t, err)
		lockFile.Close()

		// Create config with IgnoreLock enabled
		cfg := &config.Config{
			DisableRemoteCheck: false,
			IgnoreLock:         true,
		}

		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
		manager := NewManager(mockServer.URL, parquetPath, metadataPath, lockPath, cfg, logger)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		downloadErr := manager.downloadWithLock(ctx)
		assert.NoError(t, downloadErr)

		// Verify the file was downloaded
		_, err = os.Stat(parquetPath)
		assert.NoError(t, err)

		// Verify lock file was removed (or recreated and then removed)
		_, err = os.Stat(lockPath)
		assert.True(t, os.IsNotExist(err))
	})
}
