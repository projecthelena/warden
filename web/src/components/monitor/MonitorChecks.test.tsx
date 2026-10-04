import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MonitorChecks } from "./MonitorChecks";

afterEach(() => vi.unstubAllGlobals());

describe("monitor check history", () => {
  it("shows full-day counts separately and pages through retained checks", async () => {
    const fetchMock = vi.fn(
      async (url: string) =>
        new Response(
          JSON.stringify(
            url.endsWith("/summary")
              ? {
                  total: 50,
                  failed: 5,
                  recovered: 3,
                  traced: 50,
                  failurePhases: { tcp: 8 },
                }
              : url.includes("beforeId=")
                ? []
                : Array.from({ length: 20 }, (_, i) => ({
                    id: 100 - i,
                    status: "up",
                    timestamp: "2026-01-01T00:00:00Z",
                    latency: 2,
                  })),
          ),
          { status: 200 },
        ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MonitorChecks monitorId="m-test" />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Last 24 hours")).toBeInTheDocument();
    expect(screen.getByLabelText("24-hour totals")).toHaveTextContent(
      "Checks50Failed5Recovered3",
    );
    await userEvent.click(screen.getByRole("button", { name: "Older" }));
    await waitFor(() =>
      expect(
        fetchMock.mock.calls.some(([url]) => url.includes("beforeId=81")),
      ).toBe(true),
    );
    expect(await screen.findByText("No checks recorded.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Newer" })).toBeEnabled();
    client.clear();
  });
});

it("keeps individual checks available when the summary fails", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (url: string) =>
      url.endsWith("/summary")
        ? new Response("unavailable", { status: 503 })
        : new Response(
            JSON.stringify([
              {
                id: 1,
                status: "down",
                timestamp: "2026-01-01T00:00:00Z",
                latency: 0,
              },
            ]),
          ),
    ),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MonitorChecks monitorId="m-old" />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "24-hour summary is unavailable",
  );

  await userEvent.click(screen.getByLabelText(/Check failed/));
  expect(
    screen.getByText("No HTTP trace was recorded for this check."),
  ).toBeVisible();
  expect(screen.getByRole("button", { name: "Older" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "Newer" })).toBeDisabled();
  client.clear();
});

it("reports a history failure instead of presenting empty history", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () => new Response("unavailable", { status: 503 })),
  );
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MonitorChecks monitorId="m-error" />
    </QueryClientProvider>,
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Could not load recent checks.",
  );
  expect(screen.queryByText("No checks recorded.")).not.toBeInTheDocument();
  client.clear();
});

it.each([true, false])(
  "opens the exact evidence check with trace availability %s",
  async (traced) => {
    const diagnostics = traced
      ? { totalMs: 9, attempts: [{ status: "up", totalMs: 9, hops: [] }] }
      : undefined;
    const fetchMock = vi.fn(
      async () =>
        new Response(
          JSON.stringify([
            {
              id: 42,
              monitorId: "m-test",
              status: "up",
              timestamp: "2026-01-01T00:00:00Z",
              latency: 9,
              diagnostics,
            },
          ]),
        ),
    );
    vi.stubGlobal("fetch", fetchMock);
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={client}>
        <MonitorChecks monitorId="m-test" checkId="42" />
      </QueryClientProvider>,
    );
    const selected = await screen.findByTestId("selected-check");
    expect(selected).toHaveAttribute("open");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/monitors/m-test/checks?checkId=42",
      { credentials: "include" },
    );
    expect(screen.queryByText("Last 24 hours")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Older" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "All check history" }),
    ).toHaveAttribute("href", "/monitors/m-test?tab=checks");
    if (!traced)
      expect(
        screen.getByText("No HTTP trace was recorded for this check."),
      ).toBeVisible();
    client.clear();
  },
);

it("explains expired evidence without substituting recent checks", async () => {
  const fetchMock = vi.fn(async (_url: string) => new Response("[]"));
  vi.stubGlobal("fetch", fetchMock);
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <MonitorChecks monitorId="m-test" checkId="42" />
    </QueryClientProvider>,
  );
  expect(
    await screen.findByText(/This check is no longer available/),
  ).toBeInTheDocument();
  expect(fetchMock).toHaveBeenCalledTimes(1);
  expect(fetchMock.mock.calls[0][0]).toContain("checkId=42");
  client.clear();
});

it.each(["-1", "0", "abc", "9007199254740992", ""])(
  "rejects invalid evidence reference %s without fetching",
  async (checkId) => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const client = new QueryClient();
    render(
      <QueryClientProvider client={client}>
        <MonitorChecks monitorId="m-test" checkId={checkId} />
      </QueryClientProvider>,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(
      "Invalid check reference.",
    );
    expect(fetchMock).not.toHaveBeenCalled();
    client.clear();
  },
);
