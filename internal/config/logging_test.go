package config

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected slog.Level
	}{
		{"debug lowercase", "debug", slog.LevelDebug},
		{"debug uppercase", "DEBUG", slog.LevelDebug},
		{"info lowercase", "info", slog.LevelInfo},
		{"info uppercase", "INFO", slog.LevelInfo},
		{"warn lowercase", "warn", slog.LevelWarn},
		{"warn uppercase", "WARN", slog.LevelWarn},
		{"warning lowercase", "warning", slog.LevelWarn},
		{"warning uppercase", "WARNING", slog.LevelWarn},
		{"error lowercase", "error", slog.LevelError},
		{"error uppercase", "ERROR", slog.LevelError},
		{"empty string", "", slog.LevelInfo},
		{"whitespace", "  ", slog.LevelInfo},
		{"invalid", "invalid", slog.LevelInfo},
		{"with whitespace", " DEBUG ", slog.LevelDebug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLogLevel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetLogLevel(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected slog.Level
	}{
		{"debug from env", "DEBUG", slog.LevelDebug},
		{"info from env", "INFO", slog.LevelInfo},
		{"warn from env", "WARN", slog.LevelWarn},
		{"error from env", "ERROR", slog.LevelError},
		{"default when empty", "", slog.LevelInfo},
		{"default when invalid", "INVALID", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Store original value
			originalValue := os.Getenv("LOG_LEVEL")
			defer func() {
				if originalValue == "" {
					os.Unsetenv("LOG_LEVEL")
				} else {
					os.Setenv("LOG_LEVEL", originalValue)
				}
			}()

			// Set test value
			if tt.envValue == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				os.Setenv("LOG_LEVEL", tt.envValue)
			}

			result := GetLogLevel()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNewLogger(t *testing.T) {
	// Store original value
	originalValue := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("LOG_LEVEL")
		} else {
			os.Setenv("LOG_LEVEL", originalValue)
		}
	}()

	// Set test log level
	os.Setenv("LOG_LEVEL", "DEBUG")

	t.Run("stdio mode logger", func(t *testing.T) {
		logger := NewLogger(true)
		assert.NotNil(t, logger)
		// We can't easily test the handler type without reflection,
		// but we can test that it doesn't panic and creates a logger
	})

	t.Run("http mode logger", func(t *testing.T) {
		logger := NewLogger(false)
		assert.NotNil(t, logger)
		// We can't easily test the handler type without reflection,
		// but we can test that it doesn't panic and creates a logger
	})
}

func TestNewTextLogger(t *testing.T) {
	// Store original value
	originalValue := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("LOG_LEVEL")
		} else {
			os.Setenv("LOG_LEVEL", originalValue)
		}
	}()

	// Set test log level
	os.Setenv("LOG_LEVEL", "DEBUG")

	var buf bytes.Buffer
	logger := NewTextLogger(&buf)
	assert.NotNil(t, logger)

	// Test that the logger actually logs at the configured level
	logger.Debug("debug message")
	logger.Info("info message")

	output := buf.String()
	assert.Contains(t, output, "debug message")
	assert.Contains(t, output, "info message")
}

func TestNewTestLogger(t *testing.T) {
	var buf bytes.Buffer

	t.Run("with explicit level", func(t *testing.T) {
		logger := NewTestLogger(&buf, "ERROR")
		assert.NotNil(t, logger)

		buf.Reset()
		logger.Debug("debug message")
		logger.Error("error message")

		output := buf.String()
		assert.NotContains(t, output, "debug message")
		assert.Contains(t, output, "error message")
	})

	t.Run("with empty level uses env", func(t *testing.T) {
		// Store original value
		originalValue := os.Getenv("LOG_LEVEL")
		defer func() {
			if originalValue == "" {
				os.Unsetenv("LOG_LEVEL")
			} else {
				os.Setenv("LOG_LEVEL", originalValue)
			}
		}()

		os.Setenv("LOG_LEVEL", "DEBUG")

		logger := NewTestLogger(&buf, "")
		assert.NotNil(t, logger)

		buf.Reset()
		logger.Debug("debug message")

		output := buf.String()
		assert.Contains(t, output, "debug message")
	})
}

func TestLogLevelIntegration(t *testing.T) {
	// Store original value
	originalValue := os.Getenv("LOG_LEVEL")
	defer func() {
		if originalValue == "" {
			os.Unsetenv("LOG_LEVEL")
		} else {
			os.Setenv("LOG_LEVEL", originalValue)
		}
	}()

	testCases := []struct {
		logLevel string
		messages map[string]bool // message -> should be logged
	}{
		{
			logLevel: "DEBUG",
			messages: map[string]bool{
				"debug": true,
				"info":  true,
				"warn":  true,
				"error": true,
			},
		},
		{
			logLevel: "INFO",
			messages: map[string]bool{
				"debug": false,
				"info":  true,
				"warn":  true,
				"error": true,
			},
		},
		{
			logLevel: "WARN",
			messages: map[string]bool{
				"debug": false,
				"info":  false,
				"warn":  true,
				"error": true,
			},
		},
		{
			logLevel: "ERROR",
			messages: map[string]bool{
				"debug": false,
				"info":  false,
				"warn":  false,
				"error": true,
			},
		},
	}

	for _, tc := range testCases {
		t.Run("log level "+tc.logLevel, func(t *testing.T) {
			os.Setenv("LOG_LEVEL", tc.logLevel)

			var buf bytes.Buffer
			logger := NewTextLogger(&buf)

			// Log messages at different levels
			logger.Debug("debug message")
			logger.Info("info message")
			logger.Warn("warn message")
			logger.Error("error message")

			output := buf.String()

			for level, shouldBeLogged := range tc.messages {
				message := level + " message"
				if shouldBeLogged {
					assert.Contains(t, output, message, "Expected %s to be logged at level %s", level, tc.logLevel)
				} else {
					assert.NotContains(t, output, message, "Expected %s NOT to be logged at level %s", level, tc.logLevel)
				}
			}
		})
	}
}
