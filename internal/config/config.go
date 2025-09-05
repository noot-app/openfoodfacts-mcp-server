package config

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FileReader interface for dependency injection in tests
type FileReader interface {
	Open(name string) (io.ReadCloser, error)
}

// OSFileReader implements FileReader using the real file system
type OSFileReader struct{}

func (OSFileReader) Open(name string) (io.ReadCloser, error) {
	return os.Open(name)
}

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

	// Environment
	Environment string // "development" or "production"
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// Load reads configuration from environment variables
func Load() *Config {
	return LoadWithFileReader(OSFileReader{})
}

// LoadWithFileReader reads configuration from environment variables with injectable file reader
func LoadWithFileReader(fileReader FileReader) *Config {
	// Load .env file if it exists (CLI env vars will override)
	loadEnvFileWithReader(fileReader)

	dataDir := getEnv("DATA_DIR", "./data")

	refreshHours := 24
	if h := os.Getenv("REFRESH_INTERVAL_HOURS"); h != "" {
		if parsed, err := strconv.Atoi(h); err == nil {
			refreshHours = parsed
		}
	}

	return &Config{
		AuthToken:            getEnv("OPENFOODFACTS_MCP_TOKEN", "super-secret-token"),
		ParquetURL:           getEnv("PARQUET_URL", "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet"),
		DataDir:              dataDir,
		ParquetPath:          getEnv("PARQUET_PATH", filepath.Join(dataDir, "product-database.parquet")),
		MetadataPath:         getEnv("METADATA_PATH", filepath.Join(dataDir, "metadata.json")),
		LockFile:             getEnv("LOCK_FILE", filepath.Join(dataDir, "refresh.lock")),
		RefreshIntervalHours: refreshHours,
		Port:                 getEnv("PORT", "8080"),
		Environment:          getEnv("ENV", "production"),
	}
}

func loadEnvFileWithReader(fileReader FileReader) {
	file, err := fileReader.Open(".env")
	if err != nil {
		// .env file doesn't exist or can't be read, continue with CLI env vars only
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse KEY=VALUE format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// Only set if not already set in environment (CLI takes precedence)
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
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
