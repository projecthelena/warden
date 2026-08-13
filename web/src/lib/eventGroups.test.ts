import { describe, it, expect } from "vitest";
import { groupConsecutiveEvents, cameFromCheck, formatLatency } from "./eventGroups";
import { EnrichedMonitorEvent } from "@/hooks/useMonitorEvents";

// Events arrive newest-first from the API, so these fixtures are written that way too.
function ev(over: Partial<EnrichedMonitorEvent> & { id: string; timestamp: string }): EnrichedMonitorEvent {
    return {
        type: "down",
        message: "Monitor is down (Status: 500)",
        statusCode: 500,
        ...over,
    };
}

describe("groupConsecutiveEvents", () => {
    it("collapses a run of identical events into one group", () => {
        const events = [
            ev({ id: "3", timestamp: "2026-08-13T03:00:20Z", latency: 3 }),
            ev({ id: "2", timestamp: "2026-08-13T03:00:10Z", latency: 1 }),
            ev({ id: "1", timestamp: "2026-08-13T03:00:00Z", latency: 2 }),
        ];

        const groups = groupConsecutiveEvents(events);

        expect(groups).toHaveLength(1);
        expect(groups[0].events).toHaveLength(3);
        expect(groups[0].first.id).toBe("1");
        expect(groups[0].last.id).toBe("3");
        expect(groups[0].medianLatency).toBe(2);
        expect(groups[0].intervalSeconds).toBe(10);
    });

    it("splits when the status code changes — that is the signal worth seeing", () => {
        const events = [
            ev({ id: "3", timestamp: "2026-08-13T03:00:20Z" }),
            ev({ id: "2", timestamp: "2026-08-13T03:00:10Z", statusCode: 502, message: "Monitor is down (Status: 502)" }),
            ev({ id: "1", timestamp: "2026-08-13T03:00:00Z" }),
        ];

        const groups = groupConsecutiveEvents(events);

        expect(groups.map(g => g.events.length)).toEqual([1, 1, 1]);
        expect(groups[1].first.statusCode).toBe(502);
    });

    it("splits on a different error message even when the status matches", () => {
        const events = [
            ev({ id: "2", timestamp: "2026-08-13T03:00:10Z", errorMessage: "connection refused" }),
            ev({ id: "1", timestamp: "2026-08-13T03:00:00Z", errorMessage: "context deadline exceeded" }),
        ];

        expect(groupConsecutiveEvents(events)).toHaveLength(2);
    });

    it("ignores response bodies, which differ on every check", () => {
        const events = [
            ev({ id: "2", timestamp: "2026-08-13T03:00:10Z", responseBody: '{"traceId":"aaa"}' }),
            ev({ id: "1", timestamp: "2026-08-13T03:00:00Z", responseBody: '{"traceId":"bbb"}' }),
        ];

        expect(groupConsecutiveEvents(events)).toHaveLength(1);
    });

    it("does not merge two runs separated by a different event", () => {
        const events = [
            ev({ id: "4", timestamp: "2026-08-13T03:00:30Z" }),
            ev({ id: "3", timestamp: "2026-08-13T03:00:20Z", type: "recovered", message: "Monitor recovered", statusCode: 200 }),
            ev({ id: "2", timestamp: "2026-08-13T03:00:10Z" }),
            ev({ id: "1", timestamp: "2026-08-13T03:00:00Z" }),
        ];

        const groups = groupConsecutiveEvents(events);

        expect(groups.map(g => g.events.length)).toEqual([1, 1, 2]);
    });

    it("handles an empty list and a single event", () => {
        expect(groupConsecutiveEvents([])).toEqual([]);

        const one = groupConsecutiveEvents([ev({ id: "1", timestamp: "2026-08-13T03:00:00Z" })]);
        expect(one).toHaveLength(1);
        expect(one[0].intervalSeconds).toBeUndefined();
        expect(one[0].first.id).toBe(one[0].last.id);
    });

    it("leaves medianLatency undefined when nothing measured one", () => {
        const events = [
            ev({ id: "2", timestamp: "2026-08-13T03:00:10Z" }),
            ev({ id: "1", timestamp: "2026-08-13T03:00:00Z" }),
        ];

        expect(groupConsecutiveEvents(events)[0].medianLatency).toBeUndefined();
    });
});

describe("formatLatency", () => {
    it("shows a measured value", () => {
        expect(formatLatency(12, true)).toBe("12ms");
    });

    it("reports a sub-millisecond check as <1ms rather than blank", () => {
        expect(formatLatency(undefined, true)).toBe("<1ms");
        expect(formatLatency(0, true)).toBe("<1ms");
    });

    it("claims nothing for events that never measured latency", () => {
        expect(formatLatency(undefined, false)).toBeUndefined();
    });
});

describe("cameFromCheck", () => {
    it("recognises events tied to an HTTP response", () => {
        expect(cameFromCheck(ev({ id: "1", timestamp: "x" }))).toBe(true);
        expect(cameFromCheck({ id: "1", type: "down", message: "m", timestamp: "x", errorMessage: "boom" })).toBe(true);
    });

    it("rejects events that are not tied to a response", () => {
        expect(cameFromCheck({ id: "1", type: "flapping", message: "Monitor is flapping", timestamp: "x" })).toBe(false);
    });
});
