import type { HTTPDiagnostics } from "@/hooks/useMonitorEvents";

export const phaseLabels: Record<string, string> = {
    dns: "Name resolution", tcp: "Connection", tls: "Secure connection",
    connection_acquire: "Connection", send: "Sending the request", response_wait: "Waiting for a response",
    http_status: "Unexpected HTTP response", request: "Request configuration", redirect: "Redirect limit", unknown: "Unclassified",
};

export interface CheckEvidence {
    id?: number;
    status: string;
    statusCode?: number;
    timestamp: string;
    latency: number;
    diagnostics?: HTTPDiagnostics;
}

export function recoveredByRetry(check: CheckEvidence) {
    return check.status === "up" && (check.diagnostics?.attempts?.length ?? 0) > 1;
}

export function explainCheck(check: CheckEvidence) {
    const attempts = check.diagnostics?.attempts ?? [];
    const attempt = recoveredByRetry(check) ? attempts[0] : attempts.at(-1);
    const hop = attempt?.hops?.at(-1);
    const phase = attempt?.failurePhase || hop?.failurePhase;
    const code = hop?.statusCode || check.statusCode;
    let title = check.status === "up" ? "Responded" : "Check failed";
    let detail = recoveredByRetry(check) ? "The original failure could not be classified." : check.status === "up" ? (check.diagnostics ? "The response matched this monitor’s rules." : "The check succeeded. Detailed attempt evidence is unavailable.") : "There is not enough evidence to identify the cause.";
    let action = "";
    if (phase === "dns") {
        title = "Could not resolve the address";
        detail = "The request failed during DNS, before reaching the service.";
        action = "Check the hostname and resolver. Similar failures on other destinations may indicate a shared dependency.";
    } else if (phase === "tcp" || phase === "connection_acquire") {
        title = "Could not establish a connection";
        detail = "Warden could not connect; either the path or the destination may be responsible.";
        action = "Compare other destinations and the second probe before attributing this to your Internet connection.";
    } else if (phase === "tls") {
        title = "Secure connection failed";
        detail = "TLS did not complete. Certificate configuration, the destination or the network may be involved.";
        action = "Inspect the certificate and the recorded error.";
    } else if (phase === "response_wait") {
        title = "No response arrived in time";
        detail = "The request was sent, but the first response byte did not arrive in time. This does not isolate backend processing time.";
        action = "Compare service logs and another probe to distinguish the service from its network path.";
    } else if (phase === "request" || phase === "redirect") {
        title = "Check configuration needs attention";
        detail = phase === "redirect" ? "The redirect limit was reached." : "Warden could not construct the request.";
        action = "Review the target and request configuration.";
    } else if (code && (check.status !== "up" || recoveredByRetry(check))) {
        title = `Received HTTP ${code}`;
        detail = code >= 500 ? "A server or intermediary returned an error. This is not proof of a bug in the application code." : "An HTTP response arrived but did not match this monitor's accepted status codes.";
        action = code === 401 || code === 403 ? "Review credentials and access rules." : code === 429 ? "Review request frequency and rate limits." : "Check the endpoint, accepted status codes and service logs.";
    }
    if (recoveredByRetry(check)) {
        title = `Recovered after ${attempts.length - 1} ${attempts.length === 2 ? "retry" : "retries"}`;
        detail = `The first attempt failed. ${detail} The final attempt succeeded; the earlier failure remains in the history.`;
        action = "No immediate action is needed. Watch whether these recoveries become frequent.";
    }
    return { title, detail, action };
}

export function summarizeChecks(checks: CheckEvidence[]) {
    const recovered = checks.filter(recoveredByRetry).length;
    const failed = checks.filter(check => check.status === "down").length;
    const traced = checks.filter(check => check.diagnostics).length;
    const phases = new Map<string, number>();
    for (const check of checks) {
        const seen = new Set<string>();
        for (const attempt of check.diagnostics?.attempts ?? []) {
            if (attempt.failurePhase) seen.add(attempt.failurePhase);
        }
        for (const phase of seen) phases.set(phase, (phases.get(phase) ?? 0) + 1);
    }
    return { total: checks.length, recovered, failed, traced, phases: [...phases.entries()].sort((a, b) => b[1] - a[1]) };
}
