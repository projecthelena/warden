import userEvent from "@testing-library/user-event";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { HTTPDiagnosticDetails } from "./HTTPDiagnosticDetails";

import type { HTTPDiagnostics } from "@/hooks/useMonitorEvents";
import { explainCheck, recoveredByRetry } from "@/lib/checkEvidence";

describe("HTTP diagnostics", () => {
    it("separates attempts and redirects without calling response wait server time", async () => {
        render(<HTTPDiagnosticDetails diagnostics={{ attempts: [
            { totalMs: 12, hops: [{ host: "example.test", totalMs: 12, reused: false, failurePhase: "tcp", phases: [{ exchange: 1, name: "tcp", startMs: 0, durationMs: 12, complete: false }] }] },
            { totalMs: 2, hops: [{ host: "example.test", totalMs: 1, reused: true, phases: [] }, { host: "other.test", totalMs: 1, phases: [] }] },
        ], external: { outcome: "response", statusCode: 503 } }} />);
        expect(screen.getByRole("heading", { name: "Attempt 1", exact: true })).toBeInTheDocument();
        expect(screen.getByRole("heading", { name: "Attempt 2", exact: true })).toBeInTheDocument();
        expect(screen.getByText(/Redirect 1/)).toBeInTheDocument();
        await userEvent.click(screen.getByText("Technical details"));
        expect(screen.getByText("Incomplete")).toBeInTheDocument();
        expect(screen.getByText(/not backend execution time/)).toBeInTheDocument();
        expect(screen.getByText(/Reused connection/)).toBeInTheDocument();
        expect(screen.getByText(/not application health/)).toBeInTheDocument();
    });
});

it("separates request duration from retry waiting without adding overlapping phases", () => {
    render(<HTTPDiagnosticDetails diagnostics={{ totalMs: 1300, retryMode: "automatic", attempts: [
        { status: "down", failurePhase: "http_status", totalMs: 2, hops: [] },
        { status: "up", totalMs: 3, hops: [] },
    ] }} />);
    expect(screen.getByText("5.0 ms")).toBeVisible();
    expect(screen.getByText("1295.0 ms")).toBeVisible();
    expect(screen.getByText("1300.0 ms")).toBeVisible();
    expect(screen.getByText("Retry mode: automatic")).not.toBeVisible();
    expect(screen.queryByText("http_status")).not.toBeInTheDocument();
});

describe("nullable HTTP trace collections", () => {
    it.each([null, undefined])("renders an attempt without hops (%s)", hops => {
        const diagnostics = { attempts: [{ status: "down", failurePhase: "request", totalMs: 0, hops }] } as unknown as HTTPDiagnostics;
        expect(explainCheck({ status: "down", latency: 0, timestamp: "", diagnostics }).title).toBe("Check configuration needs attention");
        render(<HTTPDiagnosticDetails diagnostics={diagnostics} />);
        expect(screen.getByText("Attempt 1")).toBeVisible();
    });
    it("renders a hop without phase timings", () => {
        const diagnostics = { attempts: [{ totalMs: 2, hops: [{ host: "example.test", totalMs: 2, phases: null }] }] } as unknown as HTTPDiagnostics;
        render(<HTTPDiagnosticDetails diagnostics={diagnostics} />);
        expect(screen.getByText("No phase timings recorded.")).toBeVisible();
    });
    it("accepts an empty serialized attempts collection", () => {
        const diagnostics = { attempts: null } as unknown as HTTPDiagnostics;
        expect(recoveredByRetry({ status: "up", latency: 0, timestamp: "", diagnostics })).toBe(false);
        render(<HTTPDiagnosticDetails diagnostics={diagnostics} />);
        expect(screen.getByRole("region", { name: "HTTP diagnostics" })).toBeVisible();
    });
});
