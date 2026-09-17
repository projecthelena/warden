export type LatencyRange = "1h" | "24h" | "7d" | "30d";

export type LatencyChartPoint = {
    timestamp: string;
    latency: number | null;
    totalChecks: number;
    successfulChecks: number;
    failedChecks: number;
    state: "up" | "mixed" | "down" | "no_data";
};

export type RenderedLatencyPoint = LatencyChartPoint & { timestampMs: number };

export function toChartData(points: LatencyChartPoint[]): RenderedLatencyPoint[] {
    return points
        .map(point => ({ ...point, timestampMs: new Date(point.timestamp).getTime() }))
        .filter(point => Number.isFinite(point.timestampMs))
        .sort((a, b) => a.timestampMs - b.timestampMs);
}

export function averageSuccessfulLatency(points: LatencyChartPoint[]): number | null {
    const values = points.flatMap(point => typeof point.latency === "number" ? [point.latency] : []);
    return values.length ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : null;
}

export function bucketDuration(range: LatencyRange): number {
    if (range === "1h") return 60_000;
    if (range === "30d") return 86_400_000;
    return 3_600_000;
}

export function chartDomain(points: RenderedLatencyPoint[], range: LatencyRange): [number, number] {
    if (points.length === 0) return [0, 0];
    return [points[0].timestampMs, points[points.length - 1].timestampMs + bucketDuration(range)];
}
