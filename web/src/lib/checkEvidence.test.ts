import { describe, expect, it } from "vitest";
import { explainCheck, summarizeChecks, type CheckEvidence } from "./checkEvidence";

const recovered: CheckEvidence = { status: "up", latency: 3, timestamp: "2026-01-01T00:00:00Z", diagnostics: { attempts: [
    { status: "down", failurePhase: "tcp", totalMs: 10, hops: [] },
    { status: "up", totalMs: 3, hops: [] },
] } };

describe("check explanations", () => {
    it("keeps recovered failures visible", () => {
        expect(explainCheck(recovered).title).toBe("Recovered after 1 retry");
        expect(explainCheck(recovered).detail).toContain("earlier failure remains");
        expect(summarizeChecks([recovered])).toMatchObject({ recovered: 1, failed: 0, phases: [["tcp", 1]] });
    });
    it("does not claim a network or application cause from status alone", () => {
        const error = explainCheck({ status: "down", statusCode: 503, latency: 2, timestamp: "" });
        expect(error.detail).toContain("not proof of a bug");
        const unknown = explainCheck({ status: "down", latency: 2, timestamp: "" });
        expect(unknown.detail).toContain("not enough evidence");
    });
    it("does not count untraced checks as successful first attempts", () => {
        expect(summarizeChecks([recovered, { status: "up", latency: 1, timestamp: "" }])).toMatchObject({ total: 2, traced: 1, recovered: 1 });
    });
});
