import { render, screen } from "@testing-library/react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { UptimeBar } from "./UptimeBar";

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
        [100, "100.00%"],
        [99.9, "99.90%"],
    ])("reserves a fixed-width percentage column for %s%% uptime", (uptime, label) => {
        render(<UptimeBar days={days} overallUptime={uptime} />);

        const percentage = screen.getByText(label);
        expect(percentage).toHaveClass("w-[7ch]", "shrink-0", "text-right", "tabular-nums");
    });
});
