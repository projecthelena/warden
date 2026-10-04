import { render, screen } from "@testing-library/react";
import { vi, describe, it, expect, beforeEach } from "vitest";
import userEvent from "@testing-library/user-event";
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
        insight({
          id: 2,
          kind: "time_of_day",
          summary: "prod-3 misbehaves in the evening.",
        }),
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

  it.each([[], undefined])("shows an empty state for %s", (data) => {
    mockUseMonitorInsights.mockReturnValue({
      data,
      isLoading: false,
      error: null,
    });
    render(<InsightsCard monitorId="m1" />);
    expect(screen.getByText("No patterns detected yet.")).toBeInTheDocument();
    expect(screen.getByText(/refreshed daily/)).toBeInTheDocument();
  });

  it("distinguishes request errors from no findings", () => {
    mockUseMonitorInsights.mockReturnValue({
      data: undefined,
      isLoading: false,
      error: new Error("boom"),
    });
    render(<InsightsCard monitorId="m1" />);
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Could not load patterns",
    );
    expect(
      screen.queryByText("No patterns detected yet."),
    ).not.toBeInTheDocument();
  });

  it("announces loading without showing an empty state", () => {
    mockUseMonitorInsights.mockReturnValue({ isLoading: true, error: null });
    render(<InsightsCard monitorId="m1" />);
    expect(
      screen.getByRole("status", { name: "Loading patterns" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("No patterns detected yet."),
    ).not.toBeInTheDocument();
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
      data: [
        insight(),
        ...[2, 3, 4].map((id) =>
          insight({
            id,
            kind: "co_failure",
            summary: "Three coincident outages.",
            detail: {
              withMonitorId: `m${id}`,
              withMonitorName: `Service ${id}`,
              withCheck: { id: id + 100 },
            },
          }),
        ),
      ],
      isLoading: false,
      error: null,
    });
    render(<InsightsCard monitorId="m1" />);
    expect(screen.getByText("Patterns (2)")).toBeInTheDocument();
    expect(
      screen.getByText("Overlapping outages with 3 monitors"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Timing matches do not confirm a shared cause/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "View check for Service 2" }),
    ).toHaveAttribute("href", "/monitors/m2?tab=checks&checkId=102");
    expect(
      screen.getByRole("link", { name: "All check history" }),
    ).toHaveAttribute("href", "/monitors/m1?tab=checks");
  });

  it("shows missing trace coverage without inventing measurements", () => {
    mockUseMonitorInsights.mockReturnValue({
      data: [
        insight({
          detail: { checkEvidence: { total: 100, traced: 0, recovered: 0 } },
        }),
      ],
      isLoading: false,
      error: null,
    });
    render(<InsightsCard monitorId="m1" />);
    expect(
      screen.getByText(/No traces were recorded in this window/),
    ).toBeInTheDocument();
  });

  it("keeps forty relationships to five searchable rows per page", async () => {
    mockUseMonitorInsights.mockReturnValue({
      data: Array.from({ length: 40 }, (_, i) =>
        insight({
          id: i + 1,
          kind: "co_failure",
          detail: {
            withMonitorId: `m${i}`,
            withMonitorName: `Service ${String(i + 1).padStart(2, "0")}`,
            matchedOutages: 3,
            overlap: 100,
            withCheck: { id: i + 100 },
          },
        }),
      ),
      isLoading: false,
    });
    const user = userEvent.setup();
    render(<InsightsCard monitorId="m1" />);
    await user.click(screen.getByText("View related monitors"));
    expect(screen.getAllByTestId("related-monitor-row")).toHaveLength(5);
    expect(
      screen.getByText("3 shared outages per match · 100% downtime overlap"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/3 distinct outages started/),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Previous" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Next" }));
    expect(screen.getByText("6–10 of 40")).toBeInTheDocument();
    expect(screen.getAllByTestId("related-monitor-row")).toHaveLength(5);
    await user.type(screen.getByRole("searchbox"), "Service 40");
    expect(screen.getByText("1–1 of 1")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "View check for Service 40" }),
    ).toHaveAttribute("href", "/monitors/m39?tab=checks&checkId=139");
    await user.clear(screen.getByRole("searchbox"));
    await user.type(screen.getByRole("searchbox"), "missing");
    expect(screen.getByText("No matching monitors.")).toBeInTheDocument();
  });

  it.each([
    [
      "enabled",
      100,
      "HTTP trace capture is on.",
      "All 100 checks in this window have traces.",
    ],
    [
      "disabled",
      0,
      "HTTP trace capture is off.",
      "No traces were recorded in this window",
    ],
    [
      "enabled",
      30,
      "HTTP trace capture is on.",
      "Partial history: 30 of 100 checks have traces.",
    ],
    [
      "disabled",
      30,
      "HTTP trace capture is off.",
      "Partial history: 30 of 100 checks have traces.",
    ],
    [
      "enabled",
      0,
      "HTTP trace capture is on.",
      "No traces were recorded in this window",
    ],
    [
      undefined,
      0,
      "Current HTTP trace capture state is unavailable.",
      "No traces were recorded in this window",
    ],
  ] as const)(
    "separates current capture %s from historical coverage %s",
    (traceCapture, traced, policy, coverage) => {
      mockUseMonitorInsights.mockReturnValue({
        data: [
          insight({
            traceCapture,
            detail: { checkEvidence: { total: 100, traced } },
          }),
        ],
        isLoading: false,
      });
      render(<InsightsCard monitorId="m1" />);
      const status = screen.getByText(
        (_, element) =>
          element?.tagName === "P" &&
          element.textContent?.includes(policy) === true,
      );
      expect(status).toHaveTextContent(coverage);
      expect(
        screen.queryByText(/Compare.*HTTP timings/),
      ).not.toBeInTheDocument();
    },
  );

  it("does not treat missing metadata as disabled capture", () => {
    mockUseMonitorInsights.mockReturnValue({
      data: [insight()],
      isLoading: false,
    });
    render(<InsightsCard monitorId="m1" />);
    expect(
      screen.getByText(/Trace coverage was not recorded/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/capture is off/)).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "No retained example check is available for this finding.",
      ),
    ).toBeInTheDocument();
  });

  it("does not offer HTTP traces for non-HTTP monitors", () => {
    mockUseMonitorInsights.mockReturnValue({
      data: [insight({ traceCapture: "not_applicable" })],
      isLoading: false,
    });
    render(<InsightsCard monitorId="m1" />);
    expect(
      screen.getByText("HTTP traces do not apply to this monitor."),
    ).toBeInTheDocument();
  });

  it("links both measured periods to specific checks", () => {
    mockUseMonitorInsights.mockReturnValue({
      data: [
        insight({
          kind: "latency_drift",
          detail: { evidenceCheck: { id: 42 }, comparisonCheck: { id: 21 } },
        }),
      ],
      isLoading: false,
    });
    render(<InsightsCard monitorId="m1" />);
    expect(
      screen.getByRole("link", { name: "View example check" }),
    ).toHaveAttribute("href", "/monitors/m1?tab=checks&checkId=42");
    expect(
      screen.getByRole("link", { name: "View previous-period check" }),
    ).toHaveAttribute("href", "/monitors/m1?tab=checks&checkId=21");
  });
});
