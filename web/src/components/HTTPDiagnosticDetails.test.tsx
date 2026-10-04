import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { HTTPDiagnosticDetails } from "./HTTPDiagnosticDetails";

describe("HTTP diagnostics", () => {
    it("separates attempts and redirects without calling response wait server time", () => {
        render(<HTTPDiagnosticDetails diagnostics={{ attempts: [
            { totalMs: 12, hops: [{ host: "example.test", totalMs: 12, reused: false, failurePhase: "tcp", phases: [{ exchange: 1, name: "tcp", startMs: 0, durationMs: 12, complete: false }] }] },
            { totalMs: 2, hops: [{ host: "example.test", totalMs: 1, reused: true, phases: [] }, { host: "other.test", totalMs: 1, phases: [] }] },
        ], external: { outcome: "response", statusCode: 503 } }} />);
        expect(screen.getByText(/Attempt 1/)).toBeInTheDocument();
        expect(screen.getByText(/Attempt 2/)).toBeInTheDocument();
        expect(screen.getByText(/Redirect 1/)).toBeInTheDocument();
        expect(screen.getByText("Incomplete")).toBeInTheDocument();
        expect(screen.getByText(/not backend execution time/)).toBeInTheDocument();
        expect(screen.getByText(/Reused connection/)).toBeInTheDocument();
        expect(screen.getByText(/not application health/)).toBeInTheDocument();
    });
});
