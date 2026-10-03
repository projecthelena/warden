import { describe, expect, it } from "vitest";
import { getOverallStatus } from "./statusPageStatus";
import { getMaintenanceState } from "@/lib/maintenance";
import { buildStatusMonitorQuery, mergeStatusMonitors } from "./statusPageMonitors";

describe("mergeStatusMonitors", () => {
    it("keeps stable order while replacing duplicates", () => {
        expect(mergeStatusMonitors(
            [{ id: "a", status: "up" }, { id: "b", status: "down" }],
            [{ id: "b", status: "up" }, { id: "c", status: "up" }],
        )).toEqual([
            { id: "a", status: "up" },
            { id: "b", status: "up" },
            { id: "c", status: "up" },
        ]);
    });
});

describe("buildStatusMonitorQuery", () => {
    it("uses bounded server pagination and omits an empty search", () => {
        expect(buildStatusMonitorQuery({ group: "core", page: 2, pageSize: 50, status: "issues", search: "  " }).toString())
            .toBe("group=core&page=2&page_size=50&status=issues");
    });

    it("trims and includes a monitor search", () => {
        expect(buildStatusMonitorQuery({ group: "core", page: 1, pageSize: 25, status: "all", search: " API " }).get("search"))
            .toBe("API");
    });
});

function groupsWithStatuses(...statuses: Array<"up" | "down" | "degraded" | "paused">) {
    return [{
        id: "g-1",
        name: "Services",
        monitors: statuses.map((status, index) => ({ id: `m-${index}`, status })),
    }] as Parameters<typeof getOverallStatus>[0];
}

describe("getOverallStatus", () => {
    it("reports monitoring paused when every monitor is paused", () => {
        const status = getOverallStatus(groupsWithStatuses("paused", "paused"), [], new Set());

        expect(status.label).toBe("Monitoring Paused");
        expect(status.description).toBe("Health checks are temporarily paused.");
        expect(status.color).toBe("gray");
    });

    it("remains operational when a healthy monitor is still running", () => {
        const status = getOverallStatus(groupsWithStatuses("up", "paused"), [], new Set());

        expect(status.label).toBe("All Systems Operational");
    });

    it("does not call an empty status page paused", () => {
        const status = getOverallStatus([], [], new Set());

        expect(status.label).toBe("All Systems Operational");
    });

    it("keeps an active incident above the paused state", () => {
        const status = getOverallStatus(
            groupsWithStatuses("paused"),
            [{ type: "incident", status: "investigating", affectedGroups: ["g-1"] }] as Parameters<typeof getOverallStatus>[1],
            new Set(),
        );

        expect(status.label).toBe("System Outage");
    });
});

describe("status page maintenance lifecycle", () => {
    const window = {
        id: "maintenance-1",
        title: "Router restart",
        description: "",
        type: "maintenance",
        severity: "minor",
        status: "scheduled",
        startTime: "2026-09-15T22:13:00Z",
        endTime: "2026-09-15T22:18:00Z",
        affectedGroups: ["g-1"],
    } as Parameters<typeof getMaintenanceState>[0][number];

    it("marks affected groups only while maintenance is active", () => {
        const state = getMaintenanceState([window], new Date("2026-09-15T22:15:00Z"));

        expect(state.maintenanceGroupIds).toEqual(new Set(["g-1"]));
        expect(state.maintenanceIncidents).toHaveLength(1);
    });

    it("removes expired windows from current status", () => {
        const state = getMaintenanceState([window], new Date("2026-09-15T22:19:00Z"));

        expect(state.maintenanceGroupIds).toEqual(new Set());
        expect(state.maintenanceIncidents).toHaveLength(0);
    });
});
