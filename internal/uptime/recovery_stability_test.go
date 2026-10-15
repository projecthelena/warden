package uptime

import (
	"testing"
	"time"
)

func TestRecoveryRequiresContinuousFreshEvidence(t *testing.T) {
	base := time.Now()
	mon := &Monitor{interval: time.Second}
	for i := 0; i <= 180; i++ {
		mon.RecordResult(true, 1, base.Add(time.Duration(i)*time.Second), 200, "", false)
	}
	if !mon.RecoveryIsStable(base, base.Add(180*time.Second)) {
		t.Fatal("stable recovery was not recognized after history rolled over")
	}
	if mon.RecoveryIsStable(base, base.Add(220*time.Second)) {
		t.Fatal("stale observations announced recovery")
	}
	mon.RecordResult(true, 5000, base.Add(181*time.Second), 200, "", true)
	if mon.RecoveryIsStable(base, base.Add(181*time.Second)) {
		t.Fatal("degraded monitor announced recovery")
	}
	mon.RecordResult(true, 1, base.Add(182*time.Second), 200, "", false)
	if mon.RecoveryIsStable(base, base.Add(182*time.Second)) {
		t.Fatal("brief improvement announced recovery")
	}
	for i := 183; i <= 362; i++ {
		mon.RecordResult(true, 1, base.Add(time.Duration(i)*time.Second), 200, "", false)
	}
	if !mon.RecoveryIsStable(base, base.Add(362*time.Second)) {
		t.Fatal("new stable window was not recognized")
	}
	mon.RecordResult(true, 1, base.Add(500*time.Second), 200, "", false)
	if mon.RecoveryIsStable(base, base.Add(500*time.Second)) {
		t.Fatal("observation gap counted as healthy time")
	}
}
