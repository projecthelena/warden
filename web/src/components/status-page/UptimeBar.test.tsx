import { render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { UptimeBar } from "./UptimeBar";
import { formatDowntime } from "./uptimeCalculations";

beforeAll(() => {
    vi.stubGlobal("ResizeObserver", class {
        observe() {}
        disconnect() {}
    });
});

const days = [{
    date: "2026-09-16",
    uptimePercent: 100,
    totalChecks: 1,
}];

describe("UptimeBar", () => {
    it.each([
        [100, "100%"],
        [99.9, "99.90%"],
        [99.99, "99.99%"],
        [99.991234, "99.991%"],
        [99.997685, "99.998%"],
        [99.999999, "99.999%"],
    ])("reserves a fixed-width percentage column for %s%% uptime", (uptime, label) => {
        render(<UptimeBar days={days} overallUptime={uptime} intervalSeconds={60} />);

        const percentage = screen.getByText(label);
        expect(percentage).toHaveClass("w-[7ch]", "shrink-0", "text-right", "tabular-nums");
    });

    it("derives downtime from failed checks rather than the visual day bucket", () => {
        expect(formatDowntime(1, 60)).toBe("~1m downtime");
        expect(formatDowntime(3, 60)).toBe("~3m downtime");
        expect(formatDowntime(0, 60)).toBeNull();
    });
});
