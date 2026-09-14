package logging

import (
	"log"
	"os"
	"strings"
)

// New returns a logger with a consistent prefix to simplify traceability.
func New(component string) *log.Logger {
	prefix := component
	if prefix != "" {
		prefix = "[" + component + "] "
	}

	return log.New(os.Stdout, prefix, log.LstdFlags|log.Lmicroseconds)
}

// Sanitize removes control characters and bounds user-controlled values before logging.
func Sanitize(s string) string {
	s = strings.NewReplacer("\n", "\\n", "\r", "\\r", "\t", "\\t").Replace(s)

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}

	const maxLen = 256
	if b.Len() > maxLen {
		return b.String()[:maxLen] + "..."
	}
	return b.String()
}
