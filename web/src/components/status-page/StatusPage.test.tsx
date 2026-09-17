import { describe, expect, it } from "vitest";
import { getOverallStatus } from "./statusPageStatus";

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
