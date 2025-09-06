package config

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockFileReader implements FileReader for testing
type MockFileReader struct {
	files map[string]string // filename -> content
}

func (m MockFileReader) Open(filename string) (io.ReadCloser, error) {
	if content, exists := m.files[filename]; exists {
		return io.NopCloser(strings.NewReader(content)), nil
	}
	return nil, os.ErrNotExist
}

func (m MockFileReader) Stat(filename string) (os.FileInfo, error) {
	if _, exists := m.files[filename]; exists {
		// Return a minimal mock FileInfo - we just need it to not error
		return nil, nil
	}
	return nil, os.ErrNotExist
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		expected *Config
	}{
		{
			name:    "default values",
			envVars: map[string]string{},
			expected: &Config{
				AuthToken:              "super-secret-token",
				ParquetURL:             "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:                "./data",
				ParquetPath:            "data/product-database.parquet", // filepath.Join result
				MetadataPath:           "data/metadata.json",            // filepath.Join result
				LockFile:               "data/refresh.lock",             // filepath.Join result
				RefreshIntervalSeconds: 86400,
				DisableRemoteCheck:     false,
				IgnoreLock:             false,
				Port:                   "8080",
				Environment:            "production",
				// DuckDB defaults
				DuckDBMemoryLimit:            "4GB",
				DuckDBThreads:                4,
				DuckDBCheckpointThreshold:    "1GB",
				DuckDBPreserveInsertionOrder: true,
				// Connection pool defaults
				DuckDBMaxOpenConns:    4,
				DuckDBMaxIdleConns:    2,
				DuckDBConnMaxLifetime: 60,
			},
		},
		{
			name: "custom values",
			envVars: map[string]string{
				"OPENFOODFACTS_MCP_TOKEN":  "custom-token",
				"DATA_DIR":                 "/custom/data",
				"REFRESH_INTERVAL_SECONDS": "43200",
				"PORT":                     "3000",
			},
			expected: &Config{
				AuthToken:              "custom-token",
				ParquetURL:             "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:                "/custom/data",
				ParquetPath:            "/custom/data/product-database.parquet",
				MetadataPath:           "/custom/data/metadata.json",
				LockFile:               "/custom/data/refresh.lock",
				RefreshIntervalSeconds: 43200,
				DisableRemoteCheck:     false,
				IgnoreLock:             false,
				Port:                   "3000",
				Environment:            "production",
				// DuckDB defaults
				DuckDBMemoryLimit:            "4GB",
				DuckDBThreads:                4,
				DuckDBCheckpointThreshold:    "1GB",
				DuckDBPreserveInsertionOrder: true,
				// Connection pool defaults
				DuckDBMaxOpenConns:    4,
				DuckDBMaxIdleConns:    2,
				DuckDBConnMaxLifetime: 60,
			},
		},
		{
			name: "zero refresh interval",
			envVars: map[string]string{
				"REFRESH_INTERVAL_SECONDS": "0",
			},
			expected: &Config{
				AuthToken:              "super-secret-token",
				ParquetURL:             "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:                "./data",
				ParquetPath:            "data/product-database.parquet", // filepath.Join result
				MetadataPath:           "data/metadata.json",            // filepath.Join result
				LockFile:               "data/refresh.lock",             // filepath.Join result
				RefreshIntervalSeconds: 0,
				DisableRemoteCheck:     false,
				IgnoreLock:             false,
				Port:                   "8080",
				Environment:            "production",
				// DuckDB defaults
				DuckDBMemoryLimit:            "4GB",
				DuckDBThreads:                4,
				DuckDBCheckpointThreshold:    "1GB",
				DuckDBPreserveInsertionOrder: true,
				// Connection pool defaults
				DuckDBMaxOpenConns:    4,
				DuckDBMaxIdleConns:    2,
				DuckDBConnMaxLifetime: 60,
			},
		},
		{
			name: "disable remote check",
			envVars: map[string]string{
				"DISABLE_REMOTE_CHECK": "true",
			},
			expected: &Config{
				AuthToken:              "super-secret-token",
				ParquetURL:             "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:                "./data",
				ParquetPath:            "data/product-database.parquet", // filepath.Join result
				MetadataPath:           "data/metadata.json",            // filepath.Join result
				LockFile:               "data/refresh.lock",             // filepath.Join result
				RefreshIntervalSeconds: 86400,
				DisableRemoteCheck:     true,
				IgnoreLock:             false,
				Port:                   "8080",
				Environment:            "production",
				// DuckDB defaults
				DuckDBMemoryLimit:            "4GB",
				DuckDBThreads:                4,
				DuckDBCheckpointThreshold:    "1GB",
				DuckDBPreserveInsertionOrder: true,
				// Connection pool defaults
				DuckDBMaxOpenConns:    4,
				DuckDBMaxIdleConns:    2,
				DuckDBConnMaxLifetime: 60,
			},
		},
		{
			name: "ignore lock",
			envVars: map[string]string{
				"IGNORE_LOCK": "true",
			},
			expected: &Config{
				AuthToken:              "super-secret-token",
				ParquetURL:             "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:                "./data",
				ParquetPath:            "data/product-database.parquet", // filepath.Join result
				MetadataPath:           "data/metadata.json",            // filepath.Join result
				LockFile:               "data/refresh.lock",             // filepath.Join result
				RefreshIntervalSeconds: 86400,
				DisableRemoteCheck:     false,
				IgnoreLock:             true,
				Port:                   "8080",
				Environment:            "production",
				// DuckDB defaults
				DuckDBMemoryLimit:            "4GB",
				DuckDBThreads:                4,
				DuckDBCheckpointThreshold:    "1GB",
				DuckDBPreserveInsertionOrder: true,
				// Connection pool defaults
				DuckDBMaxOpenConns:    4,
				DuckDBMaxIdleConns:    2,
				DuckDBConnMaxLifetime: 60,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear ALL environment variables that might affect the test
			envVarsToClean := []string{
				"OPENFOODFACTS_MCP_TOKEN", "PARQUET_URL", "DATA_DIR", "PARQUET_PATH",
				"METADATA_PATH", "LOCK_FILE", "REFRESH_INTERVAL_SECONDS",
				"PORT", "ENV", "DISABLE_REMOTE_CHECK", "IGNORE_LOCK",
				// DuckDB configuration variables
				"DUCKDB_MEMORY_LIMIT", "DUCKDB_THREADS", "DUCKDB_CHECKPOINT_THRESHOLD",
				"DUCKDB_PRESERVE_INSERTION_ORDER", "DUCKDB_MAX_OPEN_CONNS", "DUCKDB_MAX_IDLE_CONNS", "DUCKDB_CONN_MAX_LIFETIME",
			}

			// Save original values
			originalVars := make(map[string]string)
			for _, key := range envVarsToClean {
				originalVars[key] = os.Getenv(key)
				os.Unsetenv(key)
			}

			// Set test env vars
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Use mock file reader that has no .env file to ensure consistent testing
			mockReader := MockFileReader{files: map[string]string{}}
			config := LoadWithFileReader(mockReader)
			assert.Equal(t, tt.expected, config)

			// Cleanup - restore original values
			for _, key := range envVarsToClean {
				os.Unsetenv(key)
				if originalVal, existed := originalVars[key]; existed && originalVal != "" {
					os.Setenv(key, originalVal)
				}
			}
		})
	}
}

func TestRefreshInterval(t *testing.T) {
	config := &Config{RefreshIntervalSeconds: 86400}
	assert.Equal(t, "24h0m0s", config.RefreshInterval().String())

	config = &Config{RefreshIntervalSeconds: 0}
	assert.Equal(t, "0s", config.RefreshInterval().String())
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		name        string
		environment string
		expected    bool
	}{
		{
			name:        "production mode",
			environment: "production",
			expected:    false,
		},
		{
			name:        "development mode",
			environment: "development",
			expected:    true,
		},
		{
			name:        "empty environment",
			environment: "",
			expected:    false,
		},
		{
			name:        "other environment",
			environment: "staging",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Environment: tt.environment}
			assert.Equal(t, tt.expected, cfg.IsDevelopment())
		})
	}
}

func TestLoadEnvFile(t *testing.T) {
	t.Run("with .env file", func(t *testing.T) {
		// Test .env file content
		envContent := `# Test .env file
OPENFOODFACTS_MCP_TOKEN=test-token-from-env
PORT=9999
# Comment line
EMPTY_LINE_ABOVE=yes

INVALID_LINE_NO_EQUALS
ANOTHER_VAR=value with spaces
`

		// Create mock file reader with .env file
		mockReader := MockFileReader{
			files: map[string]string{
				".env": envContent,
			},
		}

		// Clear existing env vars that might be set
		os.Unsetenv("OPENFOODFACTS_MCP_TOKEN")
		os.Unsetenv("PORT")
		os.Unsetenv("ANOTHER_VAR")

		// Load the .env file using mock reader
		loadEnvFileWithReader(mockReader)

		// Check that values were loaded
		assert.Equal(t, "test-token-from-env", os.Getenv("OPENFOODFACTS_MCP_TOKEN"))
		assert.Equal(t, "9999", os.Getenv("PORT"))
		assert.Equal(t, "value with spaces", os.Getenv("ANOTHER_VAR"))

		// Test CLI override: set an env var and reload
		os.Setenv("OPENFOODFACTS_MCP_TOKEN", "cli-override-token")
		loadEnvFileWithReader(mockReader)

		// CLI value should take precedence
		assert.Equal(t, "cli-override-token", os.Getenv("OPENFOODFACTS_MCP_TOKEN"))
		assert.Equal(t, "9999", os.Getenv("PORT")) // .env value should remain

		// Cleanup
		os.Unsetenv("OPENFOODFACTS_MCP_TOKEN")
		os.Unsetenv("PORT")
		os.Unsetenv("ANOTHER_VAR")
	})

	t.Run("without .env file", func(t *testing.T) {
		// Create mock file reader with no .env file
		mockReader := MockFileReader{files: map[string]string{}}

		// Clear existing env vars
		os.Unsetenv("OPENFOODFACTS_MCP_TOKEN")
		os.Unsetenv("PORT")

		// Set a CLI env var
		os.Setenv("OPENFOODFACTS_MCP_TOKEN", "cli-token")

		// Load with no .env file should not error and should not affect CLI vars
		loadEnvFileWithReader(mockReader)

		// CLI variable should remain unchanged
		assert.Equal(t, "cli-token", os.Getenv("OPENFOODFACTS_MCP_TOKEN"))
		assert.Equal(t, "", os.Getenv("PORT")) // Should remain empty

		// Cleanup
		os.Unsetenv("OPENFOODFACTS_MCP_TOKEN")
	})
}
