package server

import "time"

// HTTP server constants
const (
	// HTTP timeouts
	HTTPReadTimeout  = 15 * time.Second
	HTTPWriteTimeout = 15 * time.Second
	HTTPIdleTimeout  = 60 * time.Second

	// Shutdown timeout
	HTTPShutdownTimeout = 30 * time.Second

	// Query limits
	MaxQueryLimit     = 100
	DefaultQueryLimit = 10

	// JSON debugging constants
	MaxJSONDebugLength = 100
)
