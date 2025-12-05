package logging

import (
	"log"
	"os"
)

// New returns a logger with a consistent prefix to simplify traceability.
func New(component string) *log.Logger {
	prefix := component
	if prefix != "" {
		prefix = "[" + component + "] "
	}

	return log.New(os.Stdout, prefix, log.LstdFlags|log.Lmicroseconds)
}
