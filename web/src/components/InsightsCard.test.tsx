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

    it.each([[], undefined])("shows an empty state for %s", data => {
        mockUseMonitorInsights.mockReturnValue({ data, isLoading: false, error: null });
        render(<InsightsCard monitorId="m1" />);
        expect(screen.getByText("No patterns detected yet.")).toBeInTheDocument();
        expect(screen.getByText(/refreshed daily/)).toBeInTheDocument();
    });

    it("distinguishes request errors from no findings", () => {
        mockUseMonitorInsights.mockReturnValue({ data: undefined, isLoading: false, error: new Error("boom") });
        render(<InsightsCard monitorId="m1" />);
        expect(screen.getByRole("alert")).toHaveTextContent("Could not load patterns");
        expect(screen.queryByText("No patterns detected yet.")).not.toBeInTheDocument();
    });

    it("announces loading without showing an empty state", () => {
        mockUseMonitorInsights.mockReturnValue({ isLoading: true, error: null });
        render(<InsightsCard monitorId="m1" />);
        expect(screen.getByRole("status", { name: "Loading patterns" })).toBeInTheDocument();
        expect(screen.queryByText("No patterns detected yet.")).not.toBeInTheDocument();
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
    it("groups related monitors without claiming they share a cause", () => {
        mockUseMonitorInsights.mockReturnValue({
            data: [insight(), ...[2, 3, 4].map(id => insight({ id, kind: "co_failure", summary: "Three coincident outages.", detail: { withMonitorId: `m${id}`, withMonitorName: `Service ${id}` } }))],
            isLoading: false, error: null,
        });
        render(<InsightsCard monitorId="m1" />);
        expect(screen.getByText("Patterns (2)")).toBeInTheDocument();
        expect(screen.getByText("Overlapping outages with 3 monitors")).toBeInTheDocument();
        expect(screen.getByText(/A shared cause is not confirmed/)).toBeInTheDocument();
        expect(screen.getByRole("link", { name: "Service 2" })).toHaveAttribute("href", "/monitors/m2?tab=checks");
        expect(screen.getByRole("link", { name: "Review checks and HTTP traces" })).toHaveAttribute("href", "/monitors/m1?tab=checks");
    });

    it("shows missing trace coverage without inventing measurements", () => {
        mockUseMonitorInsights.mockReturnValue({ data: [insight({ detail: { checkEvidence: { total: 100, traced: 0, recovered: 0 } } })], isLoading: false, error: null });
        render(<InsightsCard monitorId="m1" />);
        expect(screen.getByText(/No HTTP trace evidence is available/)).toBeInTheDocument();
    });

});
