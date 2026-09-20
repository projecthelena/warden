import { describe, expect, it } from "vitest";
import { buildGroupMonitorsURL } from "./useMonitors";

describe("buildGroupMonitorsURL", () => {
    it("serializes bounded paging and trims search text", () => {
        const url = buildGroupMonitorsURL("group/one", { page: 3, pageSize: 50, search: "  checkout api  ", status: "issues" });
        expect(url).toBe("/api/groups/group%2Fone/monitors?page=3&page_size=50&status=issues&search=checkout+api");
    });

    it("omits an empty search", () => {
        expect(buildGroupMonitorsURL("g-1", { page: 1, pageSize: 25, search: " ", status: "all" }))
            .toBe("/api/groups/g-1/monitors?page=1&page_size=25&status=all");
    });
});
