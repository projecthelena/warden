package insights

import (
	"testing"
	"time"
)

// prod3Series is the real hourly latency of homedepot-nucleus-prod-3 between 10 and 17
// August 2026, read off the live instance. It is the reference case for the sawtooth
// detector: a flat 254ms baseline with periodic climbs to 400-758ms and hard resets.
var prod3Series = []int64{
	275, 255, 254, 288, 302, 417, 562, 399, 450, 438, 501, 410, 758, 255, 255, 258, 250, 251,
	257, 248, 252, 267, 271, 365, 257, 262, 256, 253, 250, 261, 255, 255, 262, 264, 305, 339,
	315, 255, 279, 385, 463, 269, 734, 254, 251, 249, 256, 253, 255, 254, 265, 254, 254, 246,
	255, 250, 252, 253, 257, 259, 252, 249, 252, 254, 324, 252, 622, 265, 325, 254, 254, 258,
	256, 253, 251, 251, 262, 255, 250, 251, 257, 257, 277, 267, 280, 327, 270, 259, 257, 266,
	266, 652, 256, 270, 257, 275, 305, 322, 253, 256, 252, 255, 257, 258, 252, 259, 257, 256,
	260, 260, 665, 263, 267, 265, 310, 456, 418, 280, 320, 253, 254, 256, 252, 252, 256, 254,
	261, 257, 256, 253, 278, 276, 285, 323, 270, 252, 254, 277, 263, 285, 294, 324, 426, 326,
	646, 251, 249, 257, 258, 256, 259, 259, 261, 273, 306, 260, 255, 255, 258, 256, 252, 253,
	253, 337, 278, 256, 252, 263, 275,
}

func seriesFrom(values []int64, start time.Time) []Sample {
	out := make([]Sample, 0, len(values))
	for i, v := range values {
		out = append(out, Sample{Hour: start.Add(time.Duration(i) * time.Hour), LatencyMs: v})
	}
	return out
}

func TestDetectSawtooth_FindsTheRealPattern(t *testing.T) {
	start := time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)
	ramps, baseline, found := DetectSawtooth(seriesFrom(prod3Series, start), DefaultSawtoothConfig())

	if !found {
		t.Fatalf("the reference series was not recognised as a sawtooth (%d ramps)", len(ramps))
	}
	if baseline < 250 || baseline > 262 {
		t.Errorf("baseline = %dms, expected the flat ~254ms floor", baseline)
	}
	if len(ramps) < 5 {
		t.Errorf("expected at least 5 ramps in a week, got %d", len(ramps))
	}

	for _, r := range ramps {
		if r.Hours < 3 {
			t.Errorf("ramp shorter than the minimum run: %+v", r)
		}
		if r.PeakMs <= r.StartMs {
			t.Errorf("ramp does not actually climb: %+v", r)
		}
		if r.ResetMs > baseline*11/10 {
			t.Errorf("ramp does not reset to baseline: reset=%dms baseline=%dms", r.ResetMs, baseline)
		}
	}
}

// A service that is merely slow, or merely noisy, is not a sawtooth. Without this the
// detector would label half the fleet.
func TestDetectSawtooth_RejectsNonPatterns(t *testing.T) {
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	flat := make([]int64, 168)
	for i := range flat {
		flat[i] = 250
	}
	if _, _, found := DetectSawtooth(seriesFrom(flat, start), DefaultSawtoothConfig()); found {
		t.Error("a perfectly flat series was called a sawtooth")
	}

	// Noise around a mean: goes up and down, never climbs for hours and never resets from
	// a real peak.
	noisy := make([]int64, 168)
	for i := range noisy {
		if i%2 == 0 {
			noisy[i] = 250
		} else {
			noisy[i] = 270
		}
	}
	if _, _, found := DetectSawtooth(seriesFrom(noisy, start), DefaultSawtoothConfig()); found {
		t.Error("alternating noise was called a sawtooth")
	}

	// A staircase that climbs and stays up is drift, not a sawtooth: no reset.
	climbing := make([]int64, 168)
	for i := range climbing {
		climbing[i] = int64(200 + i*5)
	}
	if _, _, found := DetectSawtooth(seriesFrom(climbing, start), DefaultSawtoothConfig()); found {
		t.Error("a series that climbs without ever resetting was called a sawtooth")
	}

	if _, _, found := DetectSawtooth(nil, DefaultSawtoothConfig()); found {
		t.Error("an empty series produced a finding")
	}
}

func TestSawtoothFinding_ReadsLikeSomethingActionable(t *testing.T) {
	start := time.Date(2026, 8, 10, 1, 0, 0, 0, time.UTC)
	ramps, baseline, _ := DetectSawtooth(seriesFrom(prod3Series, start), DefaultSawtoothConfig())

	f := SawtoothFinding("homedepot-nucleus-prod-3", ramps, baseline, 7)
	if f.Kind != KindSawtooth {
		t.Errorf("kind = %q", f.Kind)
	}
	if f.Confidence != "high" {
		t.Errorf("confidence = %q, want high for a pattern this repeated", f.Confidence)
	}
	if f.Detail["ramps"].(int) != len(ramps) {
		t.Errorf("detail lost the ramp count: %+v", f.Detail)
	}
	if f.Summary == "" {
		t.Error("empty summary")
	}
}

// Telling a scheduler from a load-driven pattern is half the diagnosis: one points at a
// cron or a restart policy, the other at traffic.
func TestDetectPeriodicity(t *testing.T) {
	base := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	var regular []time.Time
	for i := 0; i < 8; i++ {
		regular = append(regular, base.Add(time.Duration(i*6)*time.Hour))
	}
	period, ok := DetectPeriodicity(regular, 3, 0.25)
	if !ok {
		t.Error("a clean 6-hour cadence was not recognised")
	}
	if period < 5*time.Hour+45*time.Minute || period > 6*time.Hour+15*time.Minute {
		t.Errorf("period = %v, want about 6h", period)
	}

	// The real prod-3 peaks: 6h, 11h, 12h, 5h, 19h, 21h, 27h apart. That is not a cron,
	// and saying so is what stops someone hunting for a schedule that does not exist.
	irregularGaps := []int{0, 6, 17, 29, 34, 53, 74, 101}
	var irregular []time.Time
	for _, g := range irregularGaps {
		irregular = append(irregular, base.Add(time.Duration(g)*time.Hour))
	}
	if _, ok := DetectPeriodicity(irregular, 3, 0.25); ok {
		t.Error("the real irregular peak spacing was called periodic")
	}

	if _, ok := DetectPeriodicity(regular[:2], 3, 0.25); ok {
		t.Error("two events were enough to claim a cadence")
	}
}

func TestDetectTimeOfDay(t *testing.T) {
	base := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)

	// The real prod-3 distribution: 25 of 32 degraded events between 18:00 and 02:00 UTC.
	hours := []int{20, 0, 0, 23, 23, 23, 23, 23, 23, 22, 22, 22, 22, 21, 20, 18, 20, 21, 1,
		11, 11, 13, 14, 14, 14, 15, 20, 21, 22, 23, 19, 18}
	var times []time.Time
	for i, h := range hours {
		times = append(times, base.AddDate(0, 0, i%14).Add(time.Duration(h)*time.Hour))
	}

	startHour, width, share, found := DetectTimeOfDay(times, 8, 8, 0.6)
	if !found {
		t.Fatalf("a 78%%-concentrated distribution was not detected (share %.2f)", share)
	}
	if width != 8 {
		t.Errorf("width = %d, want 8", width)
	}
	if startHour < 17 || startHour > 20 {
		t.Errorf("band starts at %02d:00, expected the evening band", startHour)
	}

	// Events spread evenly across the day are not a schedule.
	var flat []time.Time
	for h := 0; h < 24; h++ {
		for r := 0; r < 3; r++ {
			flat = append(flat, base.Add(time.Duration(h)*time.Hour))
		}
	}
	if _, _, share, found := DetectTimeOfDay(flat, 8, 8, 0.6); found {
		t.Errorf("a uniform distribution was called a schedule (share %.2f)", share)
	}
}

func TestTimeOfDayFinding_TranslatesToLocalTime(t *testing.T) {
	bogota := time.FixedZone("-05", -5*3600)
	f := TimeOfDayFinding("prod-3", 18, 8, 0.78, 32, bogota)

	if f.Kind != KindTimeOfDay {
		t.Errorf("kind = %q", f.Kind)
	}
	// 18:00-02:00 UTC is 13:00-21:00 in Bogota. An operator reading this at midnight
	// should not have to do that arithmetic.
	if want := "13:00–21:00 your time"; !contains(f.Summary, want) {
		t.Errorf("summary does not translate the band: %q", f.Summary)
	}
	if !contains(f.Summary, "18:00 and 02:00 UTC") {
		t.Errorf("summary lost the UTC band: %q", f.Summary)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

func TestOverlap(t *testing.T) {
	base := time.Date(2026, 8, 12, 19, 29, 0, 0, time.UTC)
	now := base.Add(time.Hour)

	// The 12-Aug event: two monitors down over almost exactly the same window.
	a := []Interval{{Start: base, End: base.Add(14 * time.Minute)}}
	b := []Interval{{Start: base.Add(time.Minute), End: base.Add(13 * time.Minute)}}
	if got := Overlap(a, b, now); got < 0.8 {
		t.Errorf("overlap = %.2f, want a high share for monitors failing together", got)
	}

	// Unrelated outages on different days share nothing.
	c := []Interval{{Start: base.AddDate(0, 0, -3), End: base.AddDate(0, 0, -3).Add(10 * time.Minute)}}
	if got := Overlap(a, c, now); got != 0 {
		t.Errorf("overlap = %.2f, want 0 for unrelated outages", got)
	}

	// An open outage is measured up to now rather than treated as zero-length.
	open := []Interval{{Start: base}}
	if got := Overlap(open, open, now); got < 0.99 {
		t.Errorf("an open outage overlapping itself = %.2f, want ~1", got)
	}

	if got := Overlap(nil, b, now); got != 0 {
		t.Errorf("empty input = %.2f, want 0", got)
	}
}

func TestDetectDrift(t *testing.T) {
	base := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	mk := func(v int64, n int) []Sample {
		out := make([]Sample, 0, n)
		for i := 0; i < n; i++ {
			out = append(out, Sample{Hour: base.Add(time.Duration(i) * time.Hour), LatencyMs: v})
		}
		return out
	}

	// A service that got 40% slower week over week without ever tripping a threshold.
	pct, recent, prev, found := DetectDrift(mk(350, 168), mk(250, 168), 25, 20)
	if !found {
		t.Errorf("a 40%% slowdown was missed (%.1f%%)", pct)
	}
	if recent != 350 || prev != 250 {
		t.Errorf("medians = %d / %d, want 350 / 250", recent, prev)
	}

	// Small wobble is not drift.
	if _, _, _, found := DetectDrift(mk(255, 168), mk(250, 168), 25, 20); found {
		t.Error("a 2% wobble was reported as drift")
	}

	// A large relative change on a tiny absolute base is not worth anyone's attention:
	// 3ms to 5ms is 66% and means nothing.
	if _, _, _, found := DetectDrift(mk(5, 168), mk(3, 168), 25, 20); found {
		t.Error("a 2ms change on a 3ms base was reported as drift")
	}

	// Improvements are worth knowing too, and must not be silently dropped.
	pct, _, _, found = DetectDrift(mk(150, 168), mk(250, 168), 25, 20)
	if !found || pct >= 0 {
		t.Errorf("an improvement was not reported: found=%v pct=%.1f", found, pct)
	}

	if _, _, _, found := DetectDrift(nil, mk(250, 168), 25, 20); found {
		t.Error("empty input produced a finding")
	}
}
