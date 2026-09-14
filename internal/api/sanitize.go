package api

import "github.com/projecthelena/warden/internal/logging"

// sanitizeLog removes newlines, carriage returns, tabs, and other control
// characters from a string before it is written to log output. This prevents
// log injection attacks (gosec G706) where user-controlled values could forge
// additional log entries via embedded newlines.
//
// The result is truncated to 256 characters to prevent log flooding.
func sanitizeLog(s string) string {
	return logging.Sanitize(s)
}
