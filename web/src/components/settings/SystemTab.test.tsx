import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { MonitorHealthCard } from "./SystemTab";

describe("MonitorHealthCard", () => {
    it("accounts for every monitor with mutually exclusive health states", () => {
        render(
            <MonitorHealthCard
                stats={{
                    totalMonitors: 62,
                    activeMonitors: 43,
                    downMonitors: 1,
                    degradedMonitors: 2,
                    totalGroups: 8,
                    dailyPingsEstimate: 59616,
                }}
            />,
        );

        const healthBar = screen.getByRole("img", { name: "40 healthy, 2 degraded, 1 down, 19 paused" });
        expect(healthBar).toBeInTheDocument();
        expect(Array.from(healthBar.children).reduce((sum, segment) => sum + Number.parseFloat((segment as HTMLElement).style.width), 0)).toBeCloseTo(100);
        expect(screen.getByText("Healthy").previousElementSibling).toHaveTextContent("40");
        expect(screen.getByText("Paused").previousElementSibling).toHaveTextContent("19");
    });
});
