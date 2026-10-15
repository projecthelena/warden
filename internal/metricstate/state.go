// Package metricstate controls process-wide Prometheus instrumentation.
package metricstate

import (
	"sync/atomic"
	"time"
)

var disabled atomic.Bool

// Configure is called at startup before workers and HTTP servers are started.
func Configure(enabled bool) { disabled.Store(!enabled) }

func Enabled() bool { return !disabled.Load() }

// Now avoids clock reads used only for instrumentation when metrics are disabled.
func Now() time.Time {
	if !Enabled() {
		return time.Time{}
	}
	return time.Now()
}
