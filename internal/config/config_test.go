package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
				DataDir:              "/data/off",
				ParquetPath:          "/data/off/product-database.parquet",
				MetadataPath:         "/data/off/metadata.json",
				LockFile:             "/data/off/refresh.lock",
				RefreshIntervalHours: 24,
				Port:                 "8080",
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
				DataDir:              "/data/off",
				ParquetPath:          "/data/off/product-database.parquet",
				MetadataPath:         "/data/off/metadata.json",
				LockFile:             "/data/off/refresh.lock",
				RefreshIntervalHours: 0,
				Port:                 "8080",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			envKeys := []string{"AUTH_TOKEN", "DATA_DIR", "PARQUET_PATH", "METADATA_PATH", "LOCK_FILE", "REFRESH_INTERVAL_HOURS", "PORT"}
			for _, key := range envKeys {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			config := Load()
			assert.Equal(t, tt.expected, config)

			// Cleanup
			for key := range tt.envVars {
				os.Unsetenv(key)
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
