import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { vi, describe, it, expect, beforeEach } from "vitest";
import { IncidentCard } from "./IncidentCard";
import { EnrichedMonitorEvent } from "@/hooks/useMonitorEvents";

const mockUseIncidentEvents = vi.fn();

vi.mock("@/hooks/useMonitorEvents", () => ({
    useIncidentEvents: (...args: unknown[]) => mockUseIncidentEvents(...args),
}));

vi.mock("@/lib/store", () => ({
    useMonitorStore: () => ({ user: { timezone: "UTC" } }),
}));

function ev(over: Partial<EnrichedMonitorEvent> & { id: string; timestamp: string }): EnrichedMonitorEvent {
    return {
        type: "down",
        message: "Monitor is down (Status: 500)",
        statusCode: 500,
        responseBody: '{"error":"internal_error"}',
        ...over,
    };
}

// Newest-first, matching what the API returns.
const RUN_OF_THREE = [
    ev({ id: "3", timestamp: "2026-08-13T03:00:20Z", latency: 3 }),
    ev({ id: "2", timestamp: "2026-08-13T03:00:10Z", latency: 1 }),
    ev({ id: "1", timestamp: "2026-08-13T03:00:00Z", latency: 2 }),
];

function renderCard(props: Partial<React.ComponentProps<typeof IncidentCard>> = {}) {
    return render(
        <MemoryRouter>
            <IncidentCard
                monitorId="m-1"
                monitorName="Error 500"
                groupName="Demo Services"
                type="down"
                summary="Monitor is down (Status: 500)"
                startedAt="2026-08-13T03:00:00Z"
                duration="22m"
                {...props}
            />
        </MemoryRouter>,
    );
}

/** The drawer is lazy — open it before asserting on event content. */
async function expand() {
    await userEvent.click(screen.getAllByRole("button")[0]);
}

describe("IncidentCard", () => {
    beforeEach(() => {
        vi.clearAllMocks();
        mockUseIncidentEvents.mockReturnValue({ data: RUN_OF_THREE, isLoading: false, error: null });
    });

    describe("on the monitor's own page", () => {
        it("drops the monitor meta and the self-referential link", async () => {
            renderCard({ onMonitorPage: true });
            await expand();

            expect(screen.queryByRole("link", { name: /Open monitor page/ })).not.toBeInTheDocument();
            expect(screen.queryByRole("link", { name: "Error 500" })).not.toBeInTheDocument();
        });

        it("keeps both when rendered anywhere else", async () => {
            renderCard();
            await expand();

            expect(screen.getByRole("link", { name: /Open monitor page/ })).toBeInTheDocument();
            expect(screen.getByRole("link", { name: "Error 500" })).toBeInTheDocument();
        });
    });

    describe("a single run of repeated checks", () => {
        it("summarises the run on the drawer header instead of nesting a card", async () => {
            renderCard();
            await expand();

            expect(screen.getByText(/3 identical checks/)).toBeInTheDocument();
            expect(screen.getByText(/every 10s/)).toBeInTheDocument();
            // The generic label is replaced by the run summary — saying both is redundant.
            expect(screen.queryByText("Events during this incident")).not.toBeInTheDocument();
        });

        it("shows the first and last check, with the rest behind a toggle", async () => {
            renderCard();
            await expand();

            expect(screen.getByText("First check of the run")).toBeInTheDocument();
            expect(screen.getByText("Last check of the run")).toBeInTheDocument();

            await userEvent.click(screen.getByRole("button", { name: /Show all 3 checks/ }));
            expect(screen.getByText("All 3 checks, newest first")).toBeInTheDocument();
            expect(screen.queryByText("First check of the run")).not.toBeInTheDocument();
        });
    });

    describe("several runs", () => {
        it("keeps a card per run so the boundary between them is visible", async () => {
            mockUseIncidentEvents.mockReturnValue({
                data: [
                    ev({ id: "4", timestamp: "2026-08-13T03:00:30Z" }),
                    ev({ id: "3", timestamp: "2026-08-13T03:00:20Z", statusCode: 502, message: "Monitor is down (Status: 502)" }),
                    ev({ id: "2", timestamp: "2026-08-13T03:00:10Z" }),
                    ev({ id: "1", timestamp: "2026-08-13T03:00:00Z" }),
                ],
                isLoading: false,
                error: null,
            });

            renderCard();
            await expand();

            expect(screen.getByText("Events during this incident")).toBeInTheDocument();
            expect(screen.getByText("HTTP 502")).toBeInTheDocument();
            // The trailing pair of 500s is the only run long enough to be summarised.
            expect(screen.getByText(/2 identical checks/)).toBeInTheDocument();
        });
    });

    describe("event details", () => {
        it("labels the status code instead of showing a bare number", async () => {
            renderCard();
            await expand();

            expect(screen.getAllByText("HTTP 500").length).toBeGreaterThan(0);
        });

        it("reports a sub-millisecond check as <1ms rather than an empty gap", async () => {
            mockUseIncidentEvents.mockReturnValue({
                data: [
                    ev({ id: "2", timestamp: "2026-08-13T03:00:10Z" }),
                    ev({ id: "1", timestamp: "2026-08-13T03:00:00Z" }),
                ],
                isLoading: false,
                error: null,
            });

            renderCard();
            await expand();

            expect(screen.getAllByText("<1ms").length).toBeGreaterThan(0);
        });
    });

    describe("SSL warnings", () => {
        it("renders without an expand affordance — there are no checks behind it", () => {
            renderCard({ type: "ssl_expiring", summary: "SSL certificate expires in 14 days", duration: "" });

            const trigger = screen.getAllByRole("button")[0];
            expect(trigger).toBeDisabled();
            expect(within(trigger).queryByText(/ongoing/)).not.toBeInTheDocument();
        });
    });
});
