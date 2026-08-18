import { render, screen } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { InsightsCard } from "./InsightsCard";
import { MonitorInsight } from "@/hooks/useInsights";

const mockUseMonitorInsights = vi.fn();

vi.mock("@/hooks/useInsights", async (importOriginal) => {
    const actual = await importOriginal<typeof import("@/hooks/useInsights")>();
    return {
        ...actual,
        useMonitorInsights: (...args: unknown[]) => mockUseMonitorInsights(...args),
    };
});

function insight(over: Partial<MonitorInsight> = {}): MonitorInsight {
    return {
        id: 1,
        monitorId: "m1",
        monitorName: "prod-3",
        kind: "latency_sawtooth",
        summary: "prod-3 climbs and resets: 9 ramps in 14 days.",
        confidence: "high",
        detectedAt: "2026-08-17T02:00:00Z",
        ...over,
    };
}

describe("InsightsCard", () => {
    beforeEach(() => {
        mockUseMonitorInsights.mockReset();
    });

    it("renders each finding with a readable label", () => {
        mockUseMonitorInsights.mockReturnValue({
            data: [
                insight(),
                insight({ id: 2, kind: "time_of_day", summary: "prod-3 misbehaves in the evening." }),
            ],
            isLoading: false,
            error: null,
        });

        render(<InsightsCard monitorId="m1" />);

        expect(screen.getByText("Patterns (2)")).toBeInTheDocument();
        expect(screen.getByText("Climbs and resets")).toBeInTheDocument();
        expect(screen.getByText("Time of day")).toBeInTheDocument();
        expect(screen.getByText(/9 ramps in 14 days/)).toBeInTheDocument();
    });

    // A panel that is permanently empty trains people to stop looking at it, so a monitor
    // with nothing to report renders nothing at all rather than an empty state.
    it("renders nothing when there is nothing to say", () => {
        mockUseMonitorInsights.mockReturnValue({ data: [], isLoading: false, error: null });
        const { container } = render(<InsightsCard monitorId="m1" />);
        expect(container).toBeEmptyDOMElement();
    });

    it("renders nothing when the request failed", () => {
        mockUseMonitorInsights.mockReturnValue({
            data: undefined,
            isLoading: false,
            error: new Error("boom"),
        });
        const { container } = render(<InsightsCard monitorId="m1" />);
        expect(container).toBeEmptyDOMElement();
    });

    // Findings are heuristics, and a weaker match says so rather than sounding certain.
    it("marks a lower-confidence finding", () => {
        mockUseMonitorInsights.mockReturnValue({
            data: [insight({ confidence: "medium" })],
            isLoading: false,
            error: null,
        });

        render(<InsightsCard monitorId="m1" />);
        expect(screen.getByText("worth a look")).toBeInTheDocument();
    });

    it("does not mark a high-confidence finding", () => {
        mockUseMonitorInsights.mockReturnValue({
            data: [insight({ confidence: "high" })],
            isLoading: false,
            error: null,
        });

        render(<InsightsCard monitorId="m1" />);
        expect(screen.queryByText("worth a look")).not.toBeInTheDocument();
    });
});
