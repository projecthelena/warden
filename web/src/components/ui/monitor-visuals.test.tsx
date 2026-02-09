import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import { buildTimeSlots, UptimeHistory } from './monitor-visuals';
import { TooltipProvider } from '@/components/ui/tooltip';
import { Monitor } from '@/lib/store';

type HistoryPoint = Monitor['history'][number];

function makePoint(offsetMs: number, now: number, overrides?: Partial<HistoryPoint>): HistoryPoint {
    return {
        status: 'up',
        latency: 42,
        timestamp: new Date(now + offsetMs).toISOString(),
        statusCode: 200,
        ...overrides,
    };
}

describe('buildTimeSlots', () => {
    const NOW = new Date('2025-01-15T12:00:00Z').getTime();
    const INTERVAL = 60; // 60 seconds
    const STEP = INTERVAL * 1000; // 60000ms

    it('returns 30 empty slots when history is undefined', () => {
        const slots = buildTimeSlots(undefined, INTERVAL, NOW);
        expect(slots).toHaveLength(30);
        expect(slots.every((s) => s.point === null)).toBe(true);
    });

    it('returns 30 empty slots when history is empty', () => {
        const slots = buildTimeSlots([], INTERVAL, NOW);
        expect(slots).toHaveLength(30);
        expect(slots.every((s) => s.point === null)).toBe(true);
    });

    it('anchors slot times correctly (slot 29 = now, slot 0 = now - 29*step)', () => {
        const slots = buildTimeSlots([], INTERVAL, NOW);
        expect(slots[29].time.getTime()).toBe(NOW);
        expect(slots[0].time.getTime()).toBe(NOW - 29 * STEP);
        // Slots should be evenly spaced
        for (let i = 1; i < 30; i++) {
            expect(slots[i].time.getTime() - slots[i - 1].time.getTime()).toBe(STEP);
        }
    });

    it('populates all 30 slots with a full history', () => {
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 30; i++) {
            // One check per slot, exactly aligned
            history.push(makePoint(-(29 - i) * STEP, NOW));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        expect(slots.every((s) => s.point !== null)).toBe(true);
    });

    it('shows gray gap when service was down for several minutes', () => {
        // Checks exist for slots 0-14 and 20-29, gap at 15-19 (5 min gap with 60s interval)
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 15; i++) {
            history.push(makePoint(-(29 - i) * STEP, NOW));
        }
        for (let i = 20; i < 30; i++) {
            history.push(makePoint(-(29 - i) * STEP, NOW));
        }

        const slots = buildTimeSlots(history, INTERVAL, NOW);

        // Slots 0-14 should be populated
        for (let i = 0; i < 15; i++) {
            expect(slots[i].point).not.toBeNull();
        }
        // Slots 15-19 should be empty (gap)
        for (let i = 15; i < 20; i++) {
            expect(slots[i].point).toBeNull();
        }
        // Slots 20-29 should be populated
        for (let i = 20; i < 30; i++) {
            expect(slots[i].point).not.toBeNull();
        }
    });

    it('handles a monitor that just started (few points, rest gray)', () => {
        // Only 3 checks, the most recent ones
        const history: HistoryPoint[] = [
            makePoint(-2 * STEP, NOW),
            makePoint(-1 * STEP, NOW),
            makePoint(0, NOW),
        ];

        const slots = buildTimeSlots(history, INTERVAL, NOW);

        // First 27 slots should be empty
        for (let i = 0; i < 27; i++) {
            expect(slots[i].point).toBeNull();
        }
        // Last 3 slots should be populated
        for (let i = 27; i < 30; i++) {
            expect(slots[i].point).not.toBeNull();
        }
    });

    it('handles jittery timestamps (within half-interval tolerance)', () => {
        // Timestamps off by up to ~25 seconds on a 60s interval (tolerance = 30s)
        const history: HistoryPoint[] = [
            makePoint(-5000, NOW),   // 5s late → should still map to slot 29
            makePoint(-STEP + 8000, NOW), // 8s early → should still map to slot 28
            makePoint(-2 * STEP - 15000, NOW), // 15s late → should still map to slot 27
        ];

        const slots = buildTimeSlots(history, INTERVAL, NOW);

        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
        expect(slots[27].point).not.toBeNull();
    });

    it('rejects timestamps beyond tolerance from their nearest slot', () => {
        // Two points: one recent (anchors timeline) and one that is beyond tolerance
        // of any slot. With 60s interval and anchor at NOW, a point at NOW - 90s is
        // exactly between slot 28 (NOW-60s, dist=30s) and slot 27 (NOW-120s, dist=30s).
        // At exactly 30s it's within tolerance. Push 1ms further to be just beyond.
        const history: HistoryPoint[] = [
            makePoint(0, NOW),  // anchors slot 29 at NOW
            makePoint(-STEP / 2 - 1, NOW),  // 30001ms ago — between slot 29 (dist=30001) and slot 28 (dist=29999)
        ];

        const slots = buildTimeSlots(history, INTERVAL, NOW);

        // The anchor point fills slot 29, the marginal point should map to slot 28
        // since it's 29999ms away (within 30000ms tolerance)
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();

        // But a point truly beyond tolerance of ALL slots won't match
        const farHistory: HistoryPoint[] = [
            makePoint(0, NOW),  // anchors at NOW
            makePoint(-29 * STEP - STEP / 2 - 1, NOW), // beyond slot 0's tolerance
        ];
        const farSlots = buildTimeSlots(farHistory, INTERVAL, NOW);
        const filledCount = farSlots.filter(s => s.point !== null).length;
        expect(filledCount).toBe(1); // only the anchor point
    });

    it('defaults interval to 60s when undefined', () => {
        const history: HistoryPoint[] = [makePoint(0, NOW)];
        const slots = buildTimeSlots(history, undefined, NOW);

        expect(slots).toHaveLength(30);
        // Should use 60s step → slot 29 at NOW, slot 28 at NOW - 60000
        expect(slots[29].time.getTime()).toBe(NOW);
        expect(slots[28].time.getTime()).toBe(NOW - 60000);
        expect(slots[29].point).not.toBeNull();
    });

    it('defaults interval to 60s when zero', () => {
        const slots = buildTimeSlots([], 0, NOW);
        expect(slots[29].time.getTime() - slots[28].time.getTime()).toBe(60000);
    });

    it('latest check wins when multiple checks fall in same slot', () => {
        // Two checks both within tolerance of slot 29
        const earlier: HistoryPoint = makePoint(-10000, NOW, { status: 'down', latency: 500 });
        const later: HistoryPoint = makePoint(-2000, NOW, { status: 'up', latency: 42 });

        const slots = buildTimeSlots([earlier, later], INTERVAL, NOW);

        // The later (closer) check should win
        expect(slots[29].point).not.toBeNull();
        expect(slots[29].point!.status).toBe('up');
        expect(slots[29].point!.latency).toBe(42);
    });

    it('works with longer intervals (5 minutes)', () => {
        const longInterval = 300; // 5 min
        const longStep = longInterval * 1000;

        const history: HistoryPoint[] = [
            makePoint(0, NOW),
            makePoint(-longStep, NOW),
            makePoint(-2 * longStep, NOW),
        ];

        const slots = buildTimeSlots(history, longInterval, NOW);

        expect(slots).toHaveLength(30);
        // Window spans 29 * 300s = 8700s = 2h25m
        expect(slots[0].time.getTime()).toBe(NOW - 29 * longStep);
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
        expect(slots[27].point).not.toBeNull();
        expect(slots[26].point).toBeNull();
    });

    it('each slot has a valid time property', () => {
        const slots = buildTimeSlots([], INTERVAL, NOW);
        for (const slot of slots) {
            expect(slot.time).toBeInstanceOf(Date);
            expect(isNaN(slot.time.getTime())).toBe(false);
        }
    });

    it('preserves original history point data', () => {
        const point = makePoint(0, NOW, { status: 'degraded', latency: 999, statusCode: 503 });
        const slots = buildTimeSlots([point], INTERVAL, NOW);

        const matched = slots[29].point!;
        expect(matched.status).toBe('degraded');
        expect(matched.latency).toBe(999);
        expect(matched.statusCode).toBe(503);
    });

    it('defaults interval to 60s when negative', () => {
        const slots = buildTimeSlots([], -10, NOW);
        expect(slots[29].time.getTime() - slots[28].time.getTime()).toBe(60000);
    });

    it('drops history points outside the 30-slot window', () => {
        // 40 points: 10 are older than the window
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 40; i++) {
            history.push(makePoint(-(39 - i) * STEP, NOW));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        // All 30 slots should be filled (the 10 oldest points are beyond the window)
        expect(slots.every((s) => s.point !== null)).toBe(true);
        // The oldest slot should correspond to the 11th point, not the 1st
        const oldestSlotTs = new Date(slots[0].point!.timestamp).getTime();
        const expectedTs = NOW - 29 * STEP;
        expect(Math.abs(oldestSlotTs - expectedTs)).toBeLessThanOrEqual(STEP / 2);
    });

    it('ignores points beyond the 30-slot window from the anchor', () => {
        // The latest point anchors slot 29. A point more than 29 intervals + tolerance
        // before the anchor falls outside the window and should be dropped.
        const history: HistoryPoint[] = [
            makePoint(0, NOW),  // anchors slot 29 at NOW
            makePoint(-(30 * STEP + STEP), NOW), // 31 intervals ago — beyond slot 0's tolerance
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        // Only the anchor point should be placed
        const filledCount = slots.filter(s => s.point !== null).length;
        expect(filledCount).toBe(1);
        expect(slots[29].point).not.toBeNull();
    });

    it('accepts a point slightly in the future (within tolerance)', () => {
        // 20s into the future on a 60s interval (tolerance = 30s) → matches slot 29
        const history: HistoryPoint[] = [
            makePoint(20000, NOW, { status: 'up' }),
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        expect(slots[29].point).not.toBeNull();
        expect(slots[29].point!.status).toBe('up');
    });

    it('handles unsorted input history', () => {
        // Deliberately out of order
        const history: HistoryPoint[] = [
            makePoint(-5 * STEP, NOW, { status: 'down' }),
            makePoint(0, NOW, { status: 'up' }),
            makePoint(-10 * STEP, NOW, { status: 'degraded' }),
            makePoint(-3 * STEP, NOW, { status: 'up' }),
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        expect(slots[29].point!.status).toBe('up');
        expect(slots[26].point!.status).toBe('up');
        expect(slots[24].point!.status).toBe('down');
        expect(slots[19].point!.status).toBe('degraded');
    });

    it('includes point exactly at tolerance boundary (<=)', () => {
        // Two points: one anchors the timeline, another is exactly at tolerance distance
        // from its nearest slot. With anchor at NOW, slot 28 = NOW - 60000.
        // A point at NOW - 30000 is 30000ms from both slot 29 (filled) and slot 28.
        // 30000 <= tolerance (30000) → it should match slot 28.
        const history: HistoryPoint[] = [
            makePoint(0, NOW),       // anchors at NOW → slot 29
            makePoint(-30000, NOW),  // exactly at tolerance from slot 28
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        const filled = slots.filter((s) => s.point !== null);
        expect(filled).toHaveLength(2);
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
    });

    it('excludes point just beyond tolerance boundary', () => {
        // With anchor at NOW (from a recent point), a point beyond slot 0's tolerance
        // should not be placed. Slot 0 = NOW - 29*STEP. A point at slot 0 - tolerance - 1ms
        // is beyond tolerance for slot 0 and beyond the window entirely.
        const history: HistoryPoint[] = [
            makePoint(0, NOW), // anchors at NOW
            {
                status: 'up',
                latency: 42,
                timestamp: new Date(NOW - 29 * STEP - STEP / 2 - 1).toISOString(),
                statusCode: 200,
            },
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        // Only the anchor point should be placed
        const filledCount = slots.filter(s => s.point !== null).length;
        expect(filledCount).toBe(1);
    });

    it('works with very short intervals (10 seconds)', () => {
        const shortInterval = 10;
        const shortStep = shortInterval * 1000;

        const history: HistoryPoint[] = [
            makePoint(0, NOW),
            makePoint(-shortStep, NOW),
            makePoint(-2 * shortStep, NOW),
        ];
        const slots = buildTimeSlots(history, shortInterval, NOW);

        expect(slots).toHaveLength(30);
        // Window = 29 * 10s = 290s
        expect(slots[29].time.getTime()).toBe(NOW);
        expect(slots[0].time.getTime()).toBe(NOW - 29 * shortStep);
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
        expect(slots[27].point).not.toBeNull();
        expect(slots[26].point).toBeNull();
    });

    it('maps mixed statuses to correct slots', () => {
        const history: HistoryPoint[] = [
            makePoint(0, NOW, { status: 'up' }),
            makePoint(-STEP, NOW, { status: 'down' }),
            makePoint(-2 * STEP, NOW, { status: 'degraded' }),
            makePoint(-3 * STEP, NOW, { status: 'up' }),
            makePoint(-4 * STEP, NOW, { status: 'down' }),
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        expect(slots[29].point!.status).toBe('up');
        expect(slots[28].point!.status).toBe('down');
        expect(slots[27].point!.status).toBe('degraded');
        expect(slots[26].point!.status).toBe('up');
        expect(slots[25].point!.status).toBe('down');
    });

    it('handles duplicate timestamps (first processed wins)', () => {
        // Two points with identical timestamps — the newer (sorted first) should claim the slot
        const ts = new Date(NOW).toISOString();
        const history: HistoryPoint[] = [
            { status: 'up', latency: 100, timestamp: ts, statusCode: 200 },
            { status: 'down', latency: 500, timestamp: ts, statusCode: 500 },
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        // Only one should be placed in slot 29
        expect(slots[29].point).not.toBeNull();
        // With identical timestamps, sort is stable — but both map to same slot,
        // the first one processed claims it
        const filled = slots.filter((s) => s.point !== null);
        expect(filled).toHaveLength(1);
    });

    it('does not mutate the input history array', () => {
        const history: HistoryPoint[] = [
            makePoint(-2 * STEP, NOW),
            makePoint(0, NOW),
            makePoint(-STEP, NOW),
        ];
        const original = [...history];
        buildTimeSlots(history, INTERVAL, NOW);

        expect(history).toEqual(original);
        expect(history.length).toBe(3);
    });

    it('handles multiple scattered gaps', () => {
        // Checks at slots 0, 5, 10, 15, 20, 25, 29 — gaps everywhere in between
        const indices = [0, 5, 10, 15, 20, 25, 29];
        const history: HistoryPoint[] = indices.map((idx) =>
            makePoint(-(29 - idx) * STEP, NOW)
        );
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        for (let i = 0; i < 30; i++) {
            if (indices.includes(i)) {
                expect(slots[i].point).not.toBeNull();
            } else {
                expect(slots[i].point).toBeNull();
            }
        }
    });

    it('handles all 30 slots being down', () => {
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 30; i++) {
            history.push(makePoint(-(29 - i) * STEP, NOW, { status: 'down', statusCode: 500 }));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        expect(slots.every((s) => s.point !== null)).toBe(true);
        expect(slots.every((s) => s.point!.status === 'down')).toBe(true);
    });

    it('handles alternating up/down pattern', () => {
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 30; i++) {
            history.push(makePoint(-(29 - i) * STEP, NOW, {
                status: i % 2 === 0 ? 'up' : 'down',
            }));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        for (let i = 0; i < 30; i++) {
            expect(slots[i].point).not.toBeNull();
            expect(slots[i].point!.status).toBe(i % 2 === 0 ? 'up' : 'down');
        }
    });

    it('handles statusCode of 0', () => {
        const history: HistoryPoint[] = [
            makePoint(0, NOW, { statusCode: 0 }),
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        expect(slots[29].point!.statusCode).toBe(0);
    });

    it('handles very large history (100+ points) correctly', () => {
        // 100 points spaced 1 interval apart — only the last 30 fall in the window
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 100; i++) {
            history.push(makePoint(-(99 - i) * STEP, NOW));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        // All 30 slots should be filled from the most recent 30 points
        expect(slots.every((s) => s.point !== null)).toBe(true);
    });

    it('single point far in past anchors to now, placing point at slot 0', () => {
        // A single point 29 intervals behind now is beyond 2*step, so anchor stays at now.
        // The point lands at slot 0 (its chronological position in the window).
        const history: HistoryPoint[] = [
            makePoint(-29 * STEP, NOW, { status: 'degraded' }),
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        expect(slots[0].point).not.toBeNull();
        expect(slots[0].point!.status).toBe('degraded');
        // All other slots should be empty
        for (let i = 1; i < 30; i++) {
            expect(slots[i].point).toBeNull();
        }
    });

    it('returns a new array each call (no shared state)', () => {
        const history: HistoryPoint[] = [makePoint(0, NOW)];
        const slots1 = buildTimeSlots(history, INTERVAL, NOW);
        const slots2 = buildTimeSlots(history, INTERVAL, NOW);

        expect(slots1).not.toBe(slots2);
        expect(slots1).toEqual(slots2);
    });

    it('handles latency of 0', () => {
        const history: HistoryPoint[] = [
            makePoint(0, NOW, { latency: 0 }),
        ];
        const slots = buildTimeSlots(history, INTERVAL, NOW);
        expect(slots[29].point!.latency).toBe(0);
    });

    it('each point maps to exactly one slot (no duplicates across slots)', () => {
        // 15 points that are perfectly aligned
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 15; i++) {
            history.push(makePoint(-i * STEP, NOW));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        const filledSlots = slots.filter((s) => s.point !== null);
        expect(filledSlots).toHaveLength(15);

        // Verify no two slots share the same timestamp
        const timestamps = filledSlots.map((s) => s.point!.timestamp);
        const unique = new Set(timestamps);
        expect(unique.size).toBe(15);
    });

    it('slots are always in chronological order', () => {
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 10; i++) {
            history.push(makePoint(-i * STEP, NOW));
        }
        const slots = buildTimeSlots(history, INTERVAL, NOW);

        for (let i = 1; i < 30; i++) {
            expect(slots[i].time.getTime()).toBeGreaterThan(slots[i - 1].time.getTime());
        }
    });

    it('handles 1-second interval', () => {
        const tinyInterval = 1;
        const tinyStep = 1000;
        const history: HistoryPoint[] = [
            makePoint(0, NOW),
            makePoint(-tinyStep, NOW),
            makePoint(-2 * tinyStep, NOW),
        ];
        const slots = buildTimeSlots(history, tinyInterval, NOW);

        expect(slots).toHaveLength(30);
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
        expect(slots[27].point).not.toBeNull();
        // Window = 29 seconds
        expect(slots[0].time.getTime()).toBe(NOW - 29 * tinyStep);
    });

    it('rightmost bar always has data when history exists (no polling drift gap)', () => {
        // THE BUG: Date.now() advances between polls, creating trailing "No Data" bars.
        // With 60s interval and last check 83s ago, the old code anchored to Date.now()
        // and slot 29 was 83s ahead of the latest check → "No Data".
        // The fix: anchor to the latest history point so slot 29 always has data.
        const POLL_DRIFT = 83000; // 83 seconds since last check
        const driftedNow = NOW + POLL_DRIFT;

        const history: HistoryPoint[] = [];
        for (let i = 0; i < 10; i++) {
            history.push(makePoint(-i * STEP, NOW)); // latest at NOW, not driftedNow
        }

        const slots = buildTimeSlots(history, INTERVAL, driftedNow);

        // Slot 29 should be the latest check (at NOW), NOT "No Data"
        expect(slots[29].point).not.toBeNull();
        expect(slots[29].time.getTime()).toBe(NOW); // anchor is latest point, not driftedNow

        // All 10 points should be placed
        const filledCount = slots.filter(s => s.point !== null).length;
        expect(filledCount).toBe(10);
    });

    it('anchors to latest point even when now is far ahead', () => {
        // Simulate: React Query polled 2 minutes ago, component re-renders,
        // but history hasn't been updated yet. Date.now() is 2 minutes ahead.
        const staleNow = NOW + 2 * STEP; // 2 minutes ahead of latest check

        const history: HistoryPoint[] = [
            makePoint(0, NOW),
            makePoint(-STEP, NOW),
            makePoint(-2 * STEP, NOW),
        ];

        const slots = buildTimeSlots(history, INTERVAL, staleNow);

        // Rightmost bar should show the latest check, not "No Data"
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
        expect(slots[27].point).not.toBeNull();
        expect(slots[26].point).toBeNull();
    });

    it('empty history still uses now as anchor', () => {
        const slots = buildTimeSlots([], INTERVAL, NOW);
        expect(slots[29].time.getTime()).toBe(NOW);
        expect(slots.every(s => s.point === null)).toBe(true);
    });

    it('handles very large interval (1 hour)', () => {
        const bigInterval = 3600;
        const bigStep = bigInterval * 1000;
        const history: HistoryPoint[] = [
            makePoint(0, NOW),
            makePoint(-bigStep, NOW),
        ];
        const slots = buildTimeSlots(history, bigInterval, NOW);

        expect(slots).toHaveLength(30);
        // Window = 29 hours
        expect(slots[0].time.getTime()).toBe(NOW - 29 * bigStep);
        expect(slots[29].point).not.toBeNull();
        expect(slots[28].point).not.toBeNull();
        expect(slots[27].point).toBeNull();
    });

    it('shows gap when history is many intervals behind now (resume scenario)', () => {
        // 10 history points ending at NOW, then monitor is paused and resumed 7 intervals later.
        const resumeNow = NOW + 7 * STEP;
        const history: HistoryPoint[] = [];
        for (let i = 0; i < 10; i++) {
            history.push(makePoint(-i * STEP, NOW));
        }

        const slots = buildTimeSlots(history, INTERVAL, resumeNow);

        // Gap is 7 intervals > 2*step threshold, so anchor should be resumeNow
        expect(slots[29].time.getTime()).toBe(resumeNow);

        // Rightmost 7 slots should be empty (the pause gap)
        for (let i = 23; i < 30; i++) {
            expect(slots[i].point).toBeNull();
        }

        // The 10 old points should still be placed in the earlier slots
        const filledCount = slots.filter(s => s.point !== null).length;
        expect(filledCount).toBe(10);
    });

    it('falls back to now anchor when gap exceeds 2 intervals', () => {
        // Boundary: exactly at 2*STEP is still anchored to latest point
        const boundaryHistory: HistoryPoint[] = [makePoint(0, NOW)];
        const boundaryNow = NOW + 2 * STEP;
        const boundarySlots = buildTimeSlots(boundaryHistory, INTERVAL, boundaryNow);
        // 2*STEP gap <= 2*step threshold → anchor to latest point
        expect(boundarySlots[29].time.getTime()).toBe(NOW);

        // Just beyond: 2*STEP + 1ms → anchor to now
        const beyondNow = NOW + 2 * STEP + 1;
        const beyondSlots = buildTimeSlots(boundaryHistory, INTERVAL, beyondNow);
        expect(beyondSlots[29].time.getTime()).toBe(beyondNow);
    });
});

describe('UptimeHistory (render)', () => {
    const renderWithTooltip = (ui: React.ReactElement) =>
        render(<TooltipProvider>{ui}</TooltipProvider>);

    it('renders paused bars with bg-slate-600/40 class', () => {
        const { container } = renderWithTooltip(
            <UptimeHistory history={[]} isPaused={true} />
        );
        const bars = container.querySelectorAll('[class*="bg-slate-600/40"]');
        expect(bars.length).toBe(30);
    });

    it('renders active empty bars with bg-slate-800/30 class', () => {
        const { container } = renderWithTooltip(
            <UptimeHistory history={[]} isPaused={false} />
        );
        const bars = container.querySelectorAll('[class*="bg-slate-800/30"]');
        expect(bars.length).toBe(30);
    });

    it('defaults to active style when isPaused is omitted', () => {
        const { container } = renderWithTooltip(
            <UptimeHistory history={[]} />
        );
        const bars = container.querySelectorAll('[class*="bg-slate-800/30"]');
        expect(bars.length).toBe(30);
        const pausedBars = container.querySelectorAll('[class*="bg-slate-600/40"]');
        expect(pausedBars.length).toBe(0);
    });
});
