package config

import (
	"io"
	"log/slog"
	"os"
	"strings"
)

// LogLevel represents the available log levels
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarn
	LogLevelError
)

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo // Default to INFO if invalid/empty
	}
}

// GetLogLevel returns the log level from LOG_LEVEL environment variable
// Defaults to INFO if not set or invalid
func GetLogLevel() slog.Level {
	return parseLogLevel(os.Getenv("LOG_LEVEL"))
}

// NewLogger creates a new structured logger with the configured log level
// For production/HTTP mode, it uses JSON format to stdout
// For development/stdio mode, it uses text format to stderr
func NewLogger(isStdioMode bool) *slog.Logger {
	level := GetLogLevel()

	opts := &slog.HandlerOptions{
		Level: level,
	}

	if isStdioMode {
		// For stdio mode (Claude Desktop), use text format to stderr
		// to avoid interfering with MCP communication on stdout
		return slog.New(slog.NewTextHandler(os.Stderr, opts))
	} else {
		// For HTTP mode, use JSON format to stdout for structured logging
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
}

// NewTextLogger creates a text-based logger with the configured log level
// Useful for fetch mode and tests where structured text output is preferred
func NewTextLogger(output io.Writer) *slog.Logger {
	level := GetLogLevel()

	opts := &slog.HandlerOptions{
		Level: level,
	}

	return slog.New(slog.NewTextHandler(output, opts))
}

// NewTestLogger creates a logger for testing with configurable level and output
// If level is empty, uses LOG_LEVEL environment variable
func NewTestLogger(output io.Writer, level string) *slog.Logger {
	var logLevel slog.Level
	if level == "" {
		logLevel = GetLogLevel()
	} else {
		logLevel = parseLogLevel(level)
	}

	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	return slog.New(slog.NewTextHandler(output, opts))
}
