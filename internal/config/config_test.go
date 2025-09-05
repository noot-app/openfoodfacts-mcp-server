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
				AuthToken:            "super-secret-token",
				ParquetURL:           "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:              "./data",
				ParquetPath:          "data/product-database.parquet", // filepath.Join result
				MetadataPath:         "data/metadata.json",            // filepath.Join result
				LockFile:             "data/refresh.lock",             // filepath.Join result
				RefreshIntervalHours: 24,
				Port:                 "8080",
				Environment:          "production",
			},
		},
		{
			name: "custom values",
			envVars: map[string]string{
				"AUTH_TOKEN":             "custom-token",
				"DATA_DIR":               "/custom/data",
				"REFRESH_INTERVAL_HOURS": "12",
				"PORT":                   "3000",
			},
			expected: &Config{
				AuthToken:            "custom-token",
				ParquetURL:           "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:              "/custom/data",
				ParquetPath:          "/custom/data/product-database.parquet",
				MetadataPath:         "/custom/data/metadata.json",
				LockFile:             "/custom/data/refresh.lock",
				RefreshIntervalHours: 12,
				Port:                 "3000",
				Environment:          "production",
			},
		},
		{
			name: "zero refresh interval",
			envVars: map[string]string{
				"REFRESH_INTERVAL_HOURS": "0",
			},
			expected: &Config{
				AuthToken:            "super-secret-token",
				ParquetURL:           "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet",
				DataDir:              "./data",
				ParquetPath:          "data/product-database.parquet", // filepath.Join result
				MetadataPath:         "data/metadata.json",            // filepath.Join result
				LockFile:             "data/refresh.lock",             // filepath.Join result
				RefreshIntervalHours: 0,
				Port:                 "8080",
				Environment:          "production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear ALL environment variables that might affect the test
			envVarsToClean := []string{
				"AUTH_TOKEN", "PARQUET_URL", "DATA_DIR", "PARQUET_PATH",
				"METADATA_PATH", "LOCK_FILE", "REFRESH_INTERVAL_HOURS",
				"PORT", "ENV",
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
	config := &Config{RefreshIntervalHours: 24}
	assert.Equal(t, "24h0m0s", config.RefreshInterval().String())

	config = &Config{RefreshIntervalHours: 0}
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
AUTH_TOKEN=test-token-from-env
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
		os.Unsetenv("AUTH_TOKEN")
		os.Unsetenv("PORT")
		os.Unsetenv("ANOTHER_VAR")

		// Load the .env file using mock reader
		loadEnvFileWithReader(mockReader)

		// Check that values were loaded
		assert.Equal(t, "test-token-from-env", os.Getenv("AUTH_TOKEN"))
		assert.Equal(t, "9999", os.Getenv("PORT"))
		assert.Equal(t, "value with spaces", os.Getenv("ANOTHER_VAR"))

		// Test CLI override: set an env var and reload
		os.Setenv("AUTH_TOKEN", "cli-override-token")
		loadEnvFileWithReader(mockReader)

		// CLI value should take precedence
		assert.Equal(t, "cli-override-token", os.Getenv("AUTH_TOKEN"))
		assert.Equal(t, "9999", os.Getenv("PORT")) // .env value should remain

		// Cleanup
		os.Unsetenv("AUTH_TOKEN")
		os.Unsetenv("PORT")
		os.Unsetenv("ANOTHER_VAR")
	})

	t.Run("without .env file", func(t *testing.T) {
		// Create mock file reader with no .env file
		mockReader := MockFileReader{files: map[string]string{}}

		// Clear existing env vars
		os.Unsetenv("AUTH_TOKEN")
		os.Unsetenv("PORT")

		// Set a CLI env var
		os.Setenv("AUTH_TOKEN", "cli-token")

		// Load with no .env file should not error and should not affect CLI vars
		loadEnvFileWithReader(mockReader)

		// CLI variable should remain unchanged
		assert.Equal(t, "cli-token", os.Getenv("AUTH_TOKEN"))
		assert.Equal(t, "", os.Getenv("PORT")) // Should remain empty

		// Cleanup
		os.Unsetenv("AUTH_TOKEN")
	})
}
