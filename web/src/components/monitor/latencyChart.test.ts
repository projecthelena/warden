import { describe, expect, it } from "vitest";
import { averageSuccessfulLatency, bucketDuration, chartDomain, toChartData, type LatencyChartPoint } from "./latencyChart";

const point = (timestamp: string, latency: number | null, state: LatencyChartPoint["state"] = "up"): LatencyChartPoint => ({
    timestamp,
    latency,
    state,
    totalChecks: state === "no_data" ? 0 : 1,
    successfulChecks: latency == null ? 0 : 1,
    failedChecks: state === "down" ? 1 : 0,
});

describe("latency chart model", () => {
    it("keeps explicit no-data buckets between observed values", () => {
        const data = toChartData([
            point("2026-09-14T00:00:00Z", 200),
            point("2026-09-07T00:00:00Z", null, "no_data"),
            point("2026-09-06T00:00:00Z", 100),
        ]);

        expect(data.map(item => item.latency)).toEqual([100, null, 200]);
        expect(data[1].state).toBe("no_data");
    });

    it("does not include failures or missing data in average latency", () => {
        expect(averageSuccessfulLatency([
            point("2026-09-01T00:00:00Z", 100),
            point("2026-09-02T00:00:00Z", null, "down"),
            point("2026-09-03T00:00:00Z", null, "no_data"),
            point("2026-09-04T00:00:00Z", 300, "mixed"),
        ])).toBe(200);
    });

    it("extends the domain through the final bucket", () => {
        const data = toChartData([
            point("2026-09-17T08:00:00Z", 100),
            point("2026-09-17T09:00:00Z", 120),
        ]);
        expect(chartDomain(data, "24h")).toEqual([
            Date.parse("2026-09-17T08:00:00Z"),
            Date.parse("2026-09-17T10:00:00Z"),
        ]);
        expect(bucketDuration("1h")).toBe(60_000);
        expect(bucketDuration("30d")).toBe(86_400_000);
    });
});
