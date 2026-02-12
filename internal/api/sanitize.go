package api

import "strings"

// sanitizeLog removes newlines, carriage returns, tabs, and other control
// characters from a string before it is written to log output. This prevents
// log injection attacks (gosec G706) where user-controlled values could forge
// additional log entries via embedded newlines.
//
// The result is truncated to 256 characters to prevent log flooding.
func sanitizeLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7F {
			continue
		}
		b.WriteRune(r)
	}
	s = b.String()

	const maxLen = 256
	if len(s) > maxLen {
		s = s[:maxLen] + "..."
	}
	return s
}
