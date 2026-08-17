// Package insights reads a monitor's own history and names the shapes in it.
//
// Alerting answers "is it broken right now". This answers the slower question an operator
// only gets to by staring at charts: this one climbs for four hours and then restarts,
// that one only misbehaves during business hours, these two always fail together. The
// detectors here are deliberately explicit rules rather than anomaly detection, because a
// finding you cannot explain is a finding nobody acts on.
package insights

import (
	"fmt"
	"math"
	"sort"
	"time"
)

// Kind identifies what was found. Stored as a string so old findings stay readable after
// a detector is added or retired.
type Kind string

const (
	KindSawtooth       Kind = "latency_sawtooth"
	KindPeriodicReset  Kind = "periodic_reset"
	KindTimeOfDay      Kind = "time_of_day"
	KindRepeatOffender Kind = "repeat_offender"
	KindCoFailure      Kind = "co_failure"
	KindLatencyDrift   Kind = "latency_drift"
)

// Finding is one thing worth telling a human, in their words rather than the detector's.
type Finding struct {
	Kind    Kind   `json:"kind"`
	Summary string `json:"summary"`
	// Detail carries the numbers behind the summary, so the dashboard and the MCP can show
	// their work without re-deriving it.
	Detail map[string]any `json:"detail,omitempty"`
	// Confidence is "high" when the rule matched with room to spare and "medium" when it
	// only just did. Nothing here is ever certain, and saying so is cheaper than being
	// wrong confidently.
	Confidence string `json:"confidence"`
}

// Sample is one hour of a monitor's latency history.
type Sample struct {
	Hour       time.Time
	LatencyMs  int64
	HadFailure bool
}

// Interval is one outage, used for co-failure analysis.
type Interval struct {
	Start time.Time
	End   time.Time // zero means still open
}

// --- sawtooth -------------------------------------------------------------------------

// SawtoothConfig tunes the ramp-and-reset detector.
type SawtoothConfig struct {
	MinRunHours int     // how many consecutive rising hours count as a ramp
	PeakFactor  float64 // how far above the quiet baseline a ramp must end
	ResetFactor float64 // how close to baseline it must fall for the drop to be a reset
	MinRamps    int     // how many ramps make a pattern rather than a coincidence
}

func DefaultSawtoothConfig() SawtoothConfig {
	return SawtoothConfig{MinRunHours: 3, PeakFactor: 1.3, ResetFactor: 1.1, MinRamps: 3}
}

// Ramp is one climb followed by a fall back to normal.
type Ramp struct {
	Start          time.Time
	Peak           time.Time
	StartMs        int64
	PeakMs         int64
	Hours          int
	SlopeMsPerHour float64
	ResetMs        int64
}

// DetectSawtooth finds the shape where latency climbs steadily for hours and then drops
// straight back to normal — the signature of something being recycled, whether by a
// scheduler, an OOM kill or a connection pool being rebuilt.
//
// Warden cannot say which of those it is. It can say the shape is there, how steep it is
// and how often it repeats, which is the part that takes a human an afternoon of squinting.
func DetectSawtooth(samples []Sample, cfg SawtoothConfig) ([]Ramp, int64, bool) {
	if len(samples) < cfg.MinRunHours*2 {
		return nil, 0, false
	}

	baseline := quietBaseline(samples)
	if baseline <= 0 {
		return nil, 0, false
	}

	var ramps []Ramp
	i := 0
	for i < len(samples)-1 {
		// Walk while the series is not falling.
		j := i
		for j+1 < len(samples) && samples[j+1].LatencyMs >= samples[j].LatencyMs {
			j++
		}

		runHours := j - i + 1
		peak := samples[j].LatencyMs
		if runHours >= cfg.MinRunHours && float64(peak) >= float64(baseline)*cfg.PeakFactor {
			// The fall matters as much as the climb: without a reset this is just a service
			// getting slower, which is drift, not a sawtooth.
			if reset, ok := resetAfter(samples, j, baseline, cfg.ResetFactor); ok {
				ramps = append(ramps, Ramp{
					Start:          samples[i].Hour,
					Peak:           samples[j].Hour,
					StartMs:        samples[i].LatencyMs,
					PeakMs:         peak,
					Hours:          runHours,
					SlopeMsPerHour: float64(peak-samples[i].LatencyMs) / float64(runHours-1),
					ResetMs:        reset,
				})
			}
		}

		if j == i {
			i++
		} else {
			i = j
		}
	}

	return ramps, baseline, len(ramps) >= cfg.MinRamps
}

// resetAfter looks just past a peak for a fall back to normal. Two hours of slack, because
// the hour containing the restart often still averages high.
func resetAfter(samples []Sample, peakIdx int, baseline int64, factor float64) (int64, bool) {
	limit := float64(baseline) * factor
	for k := peakIdx + 1; k < len(samples) && k <= peakIdx+2; k++ {
		if float64(samples[k].LatencyMs) <= limit {
			return samples[k].LatencyMs, true
		}
	}
	return 0, false
}

// quietBaseline is the 25th percentile of the series: what the monitor looks like when
// nothing is wrong. The median would already be pulled up by a service that spends much of
// its day climbing.
func quietBaseline(samples []Sample) int64 {
	vals := make([]int64, 0, len(samples))
	for _, s := range samples {
		if s.LatencyMs > 0 {
			vals = append(vals, s.LatencyMs)
		}
	}
	if len(vals) == 0 {
		return 0
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals[len(vals)/4]
}

// SawtoothFinding renders the ramps as something worth reading.
func SawtoothFinding(monitorName string, ramps []Ramp, baseline int64, windowDays int) Finding {
	slopes := make([]float64, 0, len(ramps))
	peaks := make([]int64, 0, len(ramps))
	for _, r := range ramps {
		slopes = append(slopes, r.SlopeMsPerHour)
		peaks = append(peaks, r.PeakMs)
	}
	sort.Float64s(slopes)
	sort.Slice(peaks, func(i, j int) bool { return peaks[i] < peaks[j] })

	medianSlope := slopes[len(slopes)/2]
	worstPeak := peaks[len(peaks)-1]

	confidence := "medium"
	if len(ramps) >= 5 {
		confidence = "high"
	}

	return Finding{
		Kind: KindSawtooth,
		Summary: fmt.Sprintf(
			"%s climbs and resets: %d ramps in %d days, rising about %.0fms/h from a normal of %dms to as much as %dms, then dropping straight back. That shape usually means something is being recycled — a restart, an OOM kill, or a pool being rebuilt.",
			monitorName, len(ramps), windowDays, medianSlope, baseline, worstPeak),
		Detail: map[string]any{
			"ramps":                len(ramps),
			"baselineMs":           baseline,
			"medianSlopeMsPerHour": math.Round(medianSlope),
			"worstPeakMs":          worstPeak,
			"windowDays":           windowDays,
		},
		Confidence: confidence,
	}
}

// --- periodicity ----------------------------------------------------------------------

// DetectPeriodicity reports whether events repeat on a regular cadence. Low variation
// between gaps means a scheduler; high variation means something load-driven, and telling
// those two apart is half the diagnosis.
func DetectPeriodicity(times []time.Time, minEvents int, maxVariation float64) (period time.Duration, regular bool) {
	if len(times) < minEvents {
		return 0, false
	}
	sorted := append([]time.Time(nil), times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })

	gaps := make([]float64, 0, len(sorted)-1)
	for i := 1; i < len(sorted); i++ {
		gaps = append(gaps, sorted[i].Sub(sorted[i-1]).Hours())
	}
	if len(gaps) < minEvents-1 {
		return 0, false
	}

	mean := 0.0
	for _, g := range gaps {
		mean += g
	}
	mean /= float64(len(gaps))
	if mean <= 0 {
		return 0, false
	}

	variance := 0.0
	for _, g := range gaps {
		variance += (g - mean) * (g - mean)
	}
	variance /= float64(len(gaps))

	// Coefficient of variation: standard deviation as a share of the mean, so "regular" is
	// judged relative to the cadence rather than in absolute hours.
	cv := math.Sqrt(variance) / mean
	return time.Duration(mean * float64(time.Hour)), cv <= maxVariation
}

// --- time of day ----------------------------------------------------------------------

// DetectTimeOfDay finds the busiest contiguous band of hours. A monitor whose trouble piles
// into one part of the day is reacting to load, not to chance, and that rules out a whole
// class of explanations.
//
// Returns the band's start hour (UTC), its width, and the share of events inside it.
func DetectTimeOfDay(times []time.Time, bandHours int, minEvents int, minShare float64) (startHour, width int, share float64, found bool) {
	if len(times) < minEvents || bandHours <= 0 || bandHours >= 24 {
		return 0, 0, 0, false
	}

	var counts [24]int
	for _, t := range times {
		counts[t.UTC().Hour()]++
	}

	best, bestStart := 0, 0
	for start := 0; start < 24; start++ {
		sum := 0
		for k := 0; k < bandHours; k++ {
			sum += counts[(start+k)%24]
		}
		if sum > best {
			best, bestStart = sum, start
		}
	}

	share = float64(best) / float64(len(times))
	if share < minShare {
		return 0, 0, share, false
	}
	return bestStart, bandHours, share, true
}

// TimeOfDayFinding renders the band, in UTC and in the operator's own zone — an operator
// reading "18:00–02:00 UTC" at midnight should not have to do the arithmetic.
func TimeOfDayFinding(monitorName string, startHour, width int, share float64, n int, loc *time.Location) Finding {
	endHour := (startHour + width) % 24

	local := ""
	if loc != nil && loc != time.UTC {
		_, offset := time.Date(2026, 1, 1, startHour, 0, 0, 0, time.UTC).In(loc).Zone()
		ls := ((startHour+offset/3600)%24 + 24) % 24
		le := ((endHour+offset/3600)%24 + 24) % 24
		local = fmt.Sprintf(" (%02d:00–%02d:00 your time)", ls, le)
	}

	confidence := "medium"
	if share >= 0.75 {
		confidence = "high"
	}

	return Finding{
		Kind: KindTimeOfDay,
		Summary: fmt.Sprintf(
			"%s misbehaves on a schedule: %.0f%% of its %d recent problems fall between %02d:00 and %02d:00 UTC%s. That points at load rather than chance.",
			monitorName, share*100, n, startHour, endHour, local),
		Detail: map[string]any{
			"startHourUTC": startHour,
			"endHourUTC":   endHour,
			"share":        math.Round(share*100) / 100,
			"events":       n,
		},
		Confidence: confidence,
	}
}

// --- co-failure -----------------------------------------------------------------------

// Overlap reports what share of a's outage time is also b's outage time. Two monitors that
// keep failing together share a cause, and finding that out from a dashboard means holding
// two charts side by side.
func Overlap(a, b []Interval, now time.Time) float64 {
	total := 0.0
	shared := 0.0

	for _, ia := range a {
		endA := ia.End
		if endA.IsZero() {
			endA = now
		}
		d := endA.Sub(ia.Start).Seconds()
		if d <= 0 {
			continue
		}
		total += d

		for _, ib := range b {
			endB := ib.End
			if endB.IsZero() {
				endB = now
			}
			start := maxTime(ia.Start, ib.Start)
			end := minTime(endA, endB)
			if end.After(start) {
				shared += end.Sub(start).Seconds()
			}
		}
	}

	if total <= 0 {
		return 0
	}
	return math.Min(shared/total, 1)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// --- drift ----------------------------------------------------------------------------

// DetectDrift compares two consecutive windows of the same length. It catches the slow
// degradation that never trips any threshold, because every individual day looks like the
// one before it.
func DetectDrift(recent, previous []Sample, minChangePercent float64, minChangeMs int64) (changePct float64, recentMedian, prevMedian int64, found bool) {
	if len(recent) == 0 || len(previous) == 0 {
		return 0, 0, 0, false
	}
	recentMedian = median(recent)
	prevMedian = median(previous)
	if prevMedian <= 0 {
		return 0, recentMedian, prevMedian, false
	}

	diff := recentMedian - prevMedian
	changePct = float64(diff) / float64(prevMedian) * 100

	if math.Abs(changePct) < minChangePercent || abs64(diff) < minChangeMs {
		return changePct, recentMedian, prevMedian, false
	}
	return changePct, recentMedian, prevMedian, true
}

func median(samples []Sample) int64 {
	vals := make([]int64, 0, len(samples))
	for _, s := range samples {
		if s.LatencyMs > 0 {
			vals = append(vals, s.LatencyMs)
		}
	}
	if len(vals) == 0 {
		return 0
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals[len(vals)/2]
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
