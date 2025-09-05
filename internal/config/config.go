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
	RefreshIntervalSeconds int
	DisableRemoteCheck     bool

	// Server
	Port string

	// Environment
	Environment string // "development" or "production"

	// DuckDB Performance Settings
	DuckDBMemoryLimit            string // e.g. "4GB", "8GB"
	DuckDBThreads                int    // Number of threads
	DuckDBCheckpointThreshold    string // e.g. "1GB"
	DuckDBPreserveInsertionOrder bool   // Allow reordering for performance
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

	refreshSeconds := 86400 // Default to 24 hours in seconds
	if s := os.Getenv("REFRESH_INTERVAL_SECONDS"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil {
			refreshSeconds = parsed
		}
	}

	// Parse DuckDB configuration
	duckDBThreads := 4 // Default
	if t := os.Getenv("DUCKDB_THREADS"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil {
			duckDBThreads = parsed
		}
	}

	preserveInsertionOrder := true // Default to true for data integrity
	if p := os.Getenv("DUCKDB_PRESERVE_INSERTION_ORDER"); p != "" {
		if parsed, err := strconv.ParseBool(p); err == nil {
			preserveInsertionOrder = parsed
		}
	}

	// Parse disable remote check flag
	disableRemoteCheck := false // Default to false (allow remote checks)
	if d := os.Getenv("DISABLE_REMOTE_CHECK"); d != "" {
		if parsed, err := strconv.ParseBool(d); err == nil {
			disableRemoteCheck = parsed
		}
	}

	return &Config{
		AuthToken:              getEnv("OPENFOODFACTS_MCP_TOKEN", "super-secret-token"),
		ParquetURL:             getEnv("PARQUET_URL", "https://huggingface.co/datasets/openfoodfacts/product-database/resolve/main/product-database.parquet"),
		DataDir:                dataDir,
		ParquetPath:            getEnv("PARQUET_PATH", filepath.Join(dataDir, "product-database.parquet")),
		MetadataPath:           getEnv("METADATA_PATH", filepath.Join(dataDir, "metadata.json")),
		LockFile:               getEnv("LOCK_FILE", filepath.Join(dataDir, "refresh.lock")),
		RefreshIntervalSeconds: refreshSeconds,
		DisableRemoteCheck:     disableRemoteCheck,
		Port:                   getEnv("PORT", "8080"),
		Environment:            getEnv("ENV", "production"),

		// DuckDB Performance Settings with sensible defaults
		DuckDBMemoryLimit:            getEnv("DUCKDB_MEMORY_LIMIT", "4GB"),
		DuckDBThreads:                duckDBThreads,
		DuckDBCheckpointThreshold:    getEnv("DUCKDB_CHECKPOINT_THRESHOLD", "1GB"),
		DuckDBPreserveInsertionOrder: preserveInsertionOrder,
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
	return time.Duration(c.RefreshIntervalSeconds) * time.Second
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
