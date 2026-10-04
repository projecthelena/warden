import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MonitorChecks } from "./MonitorChecks";

afterEach(() => vi.unstubAllGlobals());

describe("monitor check history", () => {
    it("shows full-day counts separately and pages through retained checks", async () => {
        const fetchMock = vi.fn(async (url: string) => new Response(JSON.stringify(
            url.endsWith("/summary") ? { total: 50, failed: 5, recovered: 3, traced: 50, failurePhases: { tcp: 8 } } :
            url.includes("beforeId=") ? [] : Array.from({ length: 20 }, (_, i) => ({ id: 100 - i, status: "up", timestamp: "2026-01-01T00:00:00Z", latency: 2 }))), { status: 200 }));
        vi.stubGlobal("fetch", fetchMock);
        const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
        render(<QueryClientProvider client={client}><MonitorChecks monitorId="m-test" /></QueryClientProvider>);
        expect(await screen.findByText("Last 24 hours")).toBeInTheDocument();
        expect(screen.getByText("50 checks · 5 failed · 3 recovered after retry")).toBeInTheDocument();
        await userEvent.click(screen.getByRole("button", { name: "Older" }));
        await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => url.includes("beforeId=81"))).toBe(true));
        expect(await screen.findByText("No checks recorded.")).toBeInTheDocument();
        expect(screen.getByRole("button", { name: "Newer" })).toBeEnabled();
        client.clear();
    });
});
