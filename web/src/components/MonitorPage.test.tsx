import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes, useLocation } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { MonitorPage } from "./MonitorPage";

const state = vi.hoisted(() => ({ canEdit: true }));
vi.mock("@/hooks/useRole", () => ({ useRole: () => ({ canEdit: state.canEdit }) }));
vi.mock("@/hooks/useMonitors", () => ({ useMonitorsQuery: () => ({ isLoading: false }) }));
vi.mock("@/hooks/useSystemEvents", () => ({ useFilteredSystemEvents: () => ({ data: {} }) }));
vi.mock("@/lib/store", () => ({
    useMonitorStore: () => ({ groups: [], user: { timezone: "UTC" } }),
    findMonitorWithGroup: () => ({
        monitor: { id: "m1", name: "Test monitor", type: "http", url: "https://example.com", interval: 60, status: "up" },
        group: { name: "Test group" },
    }),
}));
vi.mock("@/components/monitor/MonitorOverview", () => ({ MonitorOverview: () => <div>Overview content</div> }));
vi.mock("@/components/monitor/MonitorChecks", () => ({ MonitorChecks: () => <div>Checks content</div> }));
vi.mock("@/components/monitor/MonitorSettings", () => ({ MonitorSettings: () => <div>Settings content</div> }));
vi.mock("@/components/InsightsCard", () => ({ InsightsCard: ({ monitorId }: { monitorId: string }) => <div>Patterns for {monitorId}</div> }));

function Location() { return <output data-testid="location">{useLocation().search}</output>; }
function open(query = "") {
    return render(<MemoryRouter initialEntries={[`/monitors/m1${query}`]}><Routes>
        <Route path="/monitors/:id" element={<><MonitorPage /><Location /></>} />
    </Routes></MemoryRouter>);
}
describe("MonitorPage tabs", () => {
    beforeEach(() => { state.canEdit = true; });
    it("opens patterns on demand and preserves filters when switching back", async () => {
        const user = userEvent.setup();
        open("?date=2026-09-01");
        expect(screen.getByText("Overview content")).toBeInTheDocument();
        expect(screen.queryByText("Patterns for m1")).not.toBeInTheDocument();
        await user.click(screen.getByRole("tab", { name: "Patterns" }));
        expect(await screen.findByText("Patterns for m1")).toBeInTheDocument();
        expect(screen.queryByText("Overview content")).not.toBeInTheDocument();
        expect(screen.getByTestId("location")).toHaveTextContent("?date=2026-09-01&tab=patterns");
        await user.click(screen.getByRole("tab", { name: "Overview" }));
        expect(screen.getByTestId("location")).toHaveTextContent("?date=2026-09-01");
        expect(screen.queryByText("Patterns for m1")).not.toBeInTheDocument();
    });
    it("supports keyboard navigation to patterns", async () => {
        const user = userEvent.setup();
        open("?tab=incidents");
        screen.getByRole("tab", { name: "Incidents" }).focus();
        await user.keyboard("{ArrowRight}");
        expect(screen.getByRole("tab", { name: "Patterns" })).toHaveFocus();
        expect(await screen.findByText("Patterns for m1")).toBeInTheDocument();
    });
    it.each([true, false])("supports direct links with edit permission %s", async canEdit => {
        state.canEdit = canEdit;
        open("?tab=patterns");
        expect(screen.getByRole("tab", { name: "Patterns" })).toHaveAttribute("aria-selected", "true");
        expect(await screen.findByText("Patterns for m1")).toBeInTheDocument();
        expect(screen.queryByRole("tab", { name: "Settings" }) !== null).toBe(canEdit);
    });
    it("preserves the checks tab and its direct link", async () => {
        const user = userEvent.setup();
        open("?tab=checks");
        expect(screen.getByText("Checks content")).toBeInTheDocument();
        await user.click(screen.getByRole("tab", { name: "Patterns" }));
        expect(await screen.findByText("Patterns for m1")).toBeInTheDocument();
        expect(screen.queryByText("Checks content")).not.toBeInTheDocument();
    });
    it.each(["unknown", "settings"])("falls back to overview for a viewer requesting %s", tab => {
        state.canEdit = false;
        open(`?tab=${tab}`);
        expect(screen.getByRole("tab", { name: "Overview" })).toHaveAttribute("aria-selected", "true");
    });
});
