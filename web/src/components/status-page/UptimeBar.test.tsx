import { fireEvent, render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { UptimeBar } from "./UptimeBar";

beforeAll(() => {
    vi.stubGlobal("ResizeObserver", class {
        private callback: ResizeObserverCallback;

        constructor(callback: ResizeObserverCallback) {
            this.callback = callback;
        }

        observe() {
            this.callback([{ contentRect: { width: 500 } } as ResizeObserverEntry], this as unknown as ResizeObserver);
        }
        disconnect() {}
    });
});

const days = [{
    date: "2026-09-16",
    uptimePercent: 100,
    totalChecks: 1,
    upChecks: 1,
}];

const summary = (percent: number) => ({
    percent,
    totalChecks: 10_000,
    downChecks: percent < 100 ? 1 : 0,
    downtimeSeconds: percent < 100 ? 60 : 0,
});

describe("UptimeBar", () => {
    it.each([
        [100, "100%"],
        [99.9, "99.90%"],
        [99.99, "99.99%"],
        [99.991234, "99.991%"],
        [99.997685, "99.998%"],
        [99.999999, "99.999%"],
    ])("reserves a fixed-width percentage column for %s%% uptime", (uptime, label) => {
        render(<UptimeBar days={days} summary={summary(uptime)} rangeDays={90} intervalSeconds={60} />);

        const percentage = screen.getByText(label);
        expect(percentage).toHaveClass("w-[7ch]", "shrink-0", "text-right", "tabular-nums");
    });

    it("names the period in the accessible label and percentage detail", () => {
        render(<UptimeBar days={days} summary={summary(99.99)} rangeDays={90} intervalSeconds={60} />);

        expect(screen.getByRole("img", { name: "Last 90 days uptime: 99.99%" })).toBeInTheDocument();
        expect(screen.getByText("99.99%")).toHaveAttribute("title", expect.stringContaining("Last 90 days"));
    });

    it("uses a weighted percentage when multiple days share one visual bucket", () => {
        const manyDays = Array.from({ length: 46 }, (_, index) => ({
            date: new Date(Date.UTC(2026, 6, index + 1)).toISOString().slice(0, 10),
            uptimePercent: index === 0 ? 0 : 100,
            totalChecks: 10,
            upChecks: index === 0 ? 0 : 10,
        }));
        render(<UptimeBar days={manyDays} summary={summary(99)} rangeDays={46} intervalSeconds={60} />);

        expect(screen.getAllByRole("button")[0]).toHaveAccessibleName(/50\.00%/);
    });

    it("does not present a range with no checks as 100% uptime", () => {
        render(<UptimeBar days={days} summary={{ percent: 100, totalChecks: 0, downChecks: 0, downtimeSeconds: 0 }} rangeDays={90} intervalSeconds={60} />);

        expect(screen.getByText("No data")).toBeInTheDocument();
    });

    it("shows exact bucket downtime when a keyboard user focuses a bar", () => {
        const mixedDay = [{
            date: "2026-09-16",
            uptimePercent: 50,
            totalChecks: 2,
            upChecks: 1,
        }];
        render(<UptimeBar days={mixedDay} summary={{ percent: 50, totalChecks: 2, downChecks: 1, downtimeSeconds: 60 }} rangeDays={1} intervalSeconds={60} />);

        fireEvent.focus(screen.getByRole("button", { name: /50\.00%/ }));
        expect(screen.getByText("1m monitored downtime · 1 failed check")).toBeVisible();
    });

    it("totals a three-day bucket instead of extrapolating its worst day", () => {
        const bucketDays = Array.from({ length: 90 }, (_, index) => ({
            date: new Date(Date.UTC(2026, 5, index + 1)).toISOString().slice(0, 10),
            uptimePercent: index === 0 ? 0 : 100,
            totalChecks: 10,
            upChecks: index === 0 ? 0 : 10,
        }));
        render(<UptimeBar days={bucketDays} summary={summary(99)} rangeDays={90} intervalSeconds={60} />);

        const firstBucket = screen.getAllByRole("button")[0];
        expect(firstBucket).toHaveAccessibleName(/66\.67%/);
        fireEvent.focus(firstBucket);
        expect(screen.getByText("10m monitored downtime · 10 failed checks")).toBeVisible();
    });
});
