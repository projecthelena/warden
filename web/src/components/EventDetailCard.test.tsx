import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { vi, describe, it, expect } from "vitest";
import { EventDetailCard } from "./EventDetailCard";
import { EnrichedMonitorEvent } from "@/hooks/useMonitorEvents";

vi.mock("@/lib/store", () => ({
    useMonitorStore: () => ({ user: { timezone: "UTC" } }),
}));

const DOCS = "https://github.com/projecthelena/warden/blob/main/docs/monitor-types.md#ping-permissions";

function ev(over: Partial<EnrichedMonitorEvent> = {}): EnrichedMonitorEvent {
    return {
        id: "1",
        type: "down",
        timestamp: "2026-08-13T10:00:00Z",
        message: "Monitor is down",
        ...over,
    } as EnrichedMonitorEvent;
}

describe("EventDetailCard", () => {
    it("turns a documentation link in the error into something clickable", async () => {
        render(
            <EventDetailCard
                event={ev({
                    // Kept distinct from the error so the assertions below cannot match
                    // the collapsed header row instead of the expanded detail.
                    message: "Ping check could not run",
                    errorMessage: `Warden is not allowed to send ping (ICMP) packets on this host. How to fix: ${DOCS} (operation not permitted)`,
                })}
            />
        );

        await userEvent.click(screen.getByRole("button"));

        const link = screen.getByRole("link", { name: DOCS });
        expect(link).toHaveAttribute("href", DOCS);
        expect(link).toHaveAttribute("target", "_blank");
        // The surrounding words have to survive the split, or the reader loses the point.
        expect(screen.getByText(/not allowed to send ping/)).toBeInTheDocument();
        expect(screen.getByText(/operation not permitted/)).toBeInTheDocument();
    });

    it("only linkifies http and https, so a crafted error cannot become a javascript: link", async () => {
        render(
            <EventDetailCard
                event={ev({ errorMessage: "refused by javascript:alert(1) and by file:///etc/passwd" })}
            />
        );

        await userEvent.click(screen.getByRole("button"));

        expect(screen.queryByRole("link")).not.toBeInTheDocument();
    });

    it("leaves an error without a link alone", async () => {
        render(<EventDetailCard event={ev({ errorMessage: "dial tcp 127.0.0.1:1: connect: connection refused" })} />);

        await userEvent.click(screen.getByRole("button"));

        expect(screen.queryByRole("link")).not.toBeInTheDocument();
        expect(screen.getByText(/connection refused/)).toBeInTheDocument();
    });
});
