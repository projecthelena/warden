package metricstate

import "testing"

func TestDisabledClock(t *testing.T) {
	Configure(false)
	t.Cleanup(func() { Configure(true) })
	if Enabled() || !Now().IsZero() {
		t.Fatal("disabled instrumentation read the clock")
	}
	Configure(true)
	if !Enabled() || Now().IsZero() {
		t.Fatal("instrumentation did not enable")
	}
}
