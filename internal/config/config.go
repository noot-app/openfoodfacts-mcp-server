package config

import (
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Config holds all configuration for the MCP server
type Config struct {
	// Auth
	AuthToken string

	// Dataset config
	ParquetURL   string
	DataDir      string
	ParquetPath  string
	MetadataPath string
	LockFile     string

	// Refresh behavior
	RefreshIntervalHours int

	// Server
	Port string
}

// Load reads configuration from environment variables
func Load() *Config {
	dataDir := getEnv("DATA_DIR", "./data")

	refreshHours := 24
	if h := os.Getenv("REFRESH_INTERVAL_HOURS"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			refreshHours = parsed
		}
	}

	return &Config{
		AuthToken:            getEnv("AUTH_TOKEN", "super-secret-token"),
		ParquetURL:           getEnv("PARQUET_URL", "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet"),
		DataDir:              dataDir,
		ParquetPath:          getEnv("PARQUET_PATH", filepath.Join(dataDir, "product-database.parquet")),
		MetadataPath:         getEnv("METADATA_PATH", filepath.Join(dataDir, "metadata.json")),
		LockFile:             getEnv("LOCK_FILE", filepath.Join(dataDir, "refresh.lock")),
		RefreshIntervalHours: refreshHours,
		Port:                 getEnv("PORT", "8080"),
	}
}

// RefreshInterval returns the refresh interval as a duration
func (c *Config) RefreshInterval() time.Duration {
	return time.Duration(c.RefreshIntervalHours) * time.Hour
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
