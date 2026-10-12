package uptime

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Run explicitly against an isolated target, using its non-loopback address to
// exercise Linux's ephemeral-port limit rather than loopback TIME_WAIT reuse.
func TestHTTPConnectionReuseLoad(t *testing.T) {
	target := os.Getenv("WARDEN_HTTP_REUSE_LOAD_TARGET")
	if target == "" {
		t.Skip("set WARDEN_HTTP_REUSE_LOAD_TARGET for the 3-minute, 500 checks/s load test")
	}
	const count = 90000
	var failures, reused, fresh atomic.Int64
	var firstError atomic.Pointer[string]
	jobs := make(chan struct{}, 1000)
	var workers sync.WaitGroup
	for i := 0; i < 50; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			transport := checkTransport()
			defer transport.CloseIdleConnections()
			for range jobs {
				result := runCheck(Job{URL: target, CaptureDiagnostics: true}, transport)
				if !result.Status {
					failures.Add(1)
					firstError.CompareAndSwap(nil, &result.Error)
				}
				if result.Diagnostics != nil {
					for _, attempt := range result.Diagnostics.Attempts {
						for _, hop := range attempt.Hops {
							if hop.Reused != nil {
								if *hop.Reused {
									reused.Add(1)
								} else {
									fresh.Add(1)
								}
							}
						}
					}
				}
			}
		}()
	}
	started := time.Now()
	for i := 0; i < count; i++ {
		if delay := time.Until(started.Add(time.Duration(i) * 2 * time.Millisecond)); delay > 0 {
			time.Sleep(delay)
		}
		jobs <- struct{}{}
	}
	close(jobs)
	workers.Wait()
	elapsed := time.Since(started)
	t.Logf("checks=%d duration=%s rate=%.2f/s failed=%d reused=%d fresh=%d", count, elapsed, float64(count)/elapsed.Seconds(), failures.Load(), reused.Load(), fresh.Load())
	if err := firstError.Load(); err != nil {
		t.Logf("first error: %s", *err)
	}
	if failures.Load() != 0 {
		t.Errorf("%d checks failed", failures.Load())
	}
	if float64(count)/elapsed.Seconds() < 495 {
		t.Error("throughput below 99 percent of 500/s")
	}
	if reused.Load() < count*99/100 {
		t.Error("less than 99 percent of checks reused a connection")
	}
}
