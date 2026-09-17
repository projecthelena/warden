import { describe, expect, it } from "vitest";
import { formatDuration, formatUptime, formatUptimeDetail, formatUptimePeriod, formatUptimeSummary } from "./uptime";

describe("formatUptime", () => {
    it("does not round a small amount of downtime up to 100%", () => {
        expect(formatUptime(99.997685)).toBe("99.998%");
        expect(formatUptime(99.9999)).toBe("99.999%");
    });

    it("keeps ordinary percentages compact", () => {
        expect(formatUptime(99.98)).toBe("99.98%");
        expect(formatUptime(100)).toBe("100%");
    });
});

describe("formatDuration", () => {
    it("formats downtime for metric tooltips", () => {
        expect(formatDuration(180)).toBe("3m");
        expect(formatDuration(9_060)).toBe("2h 31m");
    });
});

describe("uptime summaries", () => {
    it("distinguishes no data from perfect uptime", () => {
        expect(formatUptimeSummary({ percent: 100, totalChecks: 0, downChecks: 0, downtimeSeconds: 0 })).toBe("No data");
        expect(formatUptimeSummary({ percent: 100, totalChecks: 60, downChecks: 0, downtimeSeconds: 0 })).toBe("100%");
    });

    it("uses the same downtime copy across uptime surfaces", () => {
        const detail = formatUptimeDetail({ percent: 99.9, totalChecks: 1_440, downChecks: 3, downtimeSeconds: 180 });
        expect(detail).toBe("3m monitored downtime · 3 failed checks");
        expect(formatUptimePeriod(90)).toBe("Last 90 days");
    });
});
