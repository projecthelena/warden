import type { HTTPDiagnostics } from "@/hooks/useMonitorEvents";

const labels: Record<string, string> = {
    connection_acquire: "Acquire connection (includes DNS/TCP/TLS)",
    dns: "DNS", tcp: "TCP", tls: "TLS", send: "Send request",
    response_wait: "Wait for first response byte",
};

export function HTTPDiagnosticDetails({ diagnostics }: { diagnostics: HTTPDiagnostics }) {
    return <section className="space-y-3 text-xs" aria-label="HTTP diagnostics">
        <p className="text-muted-foreground">Times can overlap and must not be added. Response wait includes the network and service; it is not backend execution time. Missing phases were not observed.</p>
        {diagnostics.retryMode && <p>Retry mode: {diagnostics.retryMode}{diagnostics.totalMs !== undefined ? ` · Total including retries: ${diagnostics.totalMs.toFixed(1)} ms` : ""}</p>}
        {diagnostics.retryDecision === "capacity_limited" && <p>No immediate retry: Warden reserved capacity for normal checks. The next scheduled check will try again.</p>}
        {diagnostics.retryDecision === "budget_exhausted" && <p>No immediate retry: the check's time budget was exhausted. The next scheduled check will try again.</p>}
        {diagnostics.retryDecision === "not_transient" && <p>No immediate retry: this error is not eligible for automatic retry, or the service requested a delay.</p>}
        {diagnostics.retryDecision === "unsafe_request" && <p>Automatic retry is limited to GET/HEAD requests without a body.</p>}
        {diagnostics.attempts.map((attempt, i) => <div key={i} className="space-y-2">
            <p className="font-medium">Attempt {i + 1} · {attempt.totalMs.toFixed(1)} ms{attempt.truncated && " · Trace truncated"}</p>
            {attempt.failurePhase && <p>Attempt failed: {labels[attempt.failurePhase] ?? attempt.failurePhase}</p>}
            {attempt.hops.map((hop, j) => <div key={j} className="border border-border rounded p-2 space-y-1 overflow-x-auto">
                <p className="break-all">{j === 0 ? "Request" : `Redirect ${j}`} · {hop.host} · {hop.totalMs.toFixed(1)} ms</p>
                <p>{hop.remoteIp ?? "Remote IP unavailable"} · {hop.reused === undefined ? "Connection unavailable" : hop.reused ? "Reused connection" : "New connection"}{hop.statusCode ? ` · HTTP ${hop.statusCode}` : ""}</p>
                {hop.failurePhase && <p className="text-rose-500">Failed during: {labels[hop.failurePhase] ?? hop.failurePhase}</p>}
                <table className="w-full text-left"><thead><tr><th>Phase</th><th>Transport attempt</th><th>Start</th><th>Duration</th><th>Result</th></tr></thead>
                    <tbody>{hop.phases.map((phase, k) => <tr key={k}>
                        <td>{labels[phase.name] ?? phase.name}</td><td>{phase.exchange}</td><td>{phase.startMs.toFixed(1)} ms</td><td>{phase.durationMs.toFixed(1)} ms</td>
                        <td>{phase.failed ? "Failed" : phase.complete ? "Completed" : "Incomplete"}</td>
                    </tr>)}</tbody>
                </table>
                {hop.truncated && <p>Trace truncated</p>}
            </div>)}
        </div>)}
        {diagnostics.external && <div className="border border-border rounded p-2">
            <p>Second probe: {diagnostics.external.outcome}{diagnostics.external.statusCode ? ` · HTTP ${diagnostics.external.statusCode}` : ""}{diagnostics.external.failurePhase ? ` · ${diagnostics.external.failurePhase}` : ""}</p>
            {diagnostics.external.observedAt && <p>Observed: {diagnostics.external.observedAt}</p>}
            <p className="text-muted-foreground">A response confirms reachability, not application health. Two probes on the same Internet connection cannot isolate an ISP failure.</p>
        </div>}
    </section>;
}
