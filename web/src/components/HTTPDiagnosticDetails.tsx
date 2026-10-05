import type { HTTPDiagnostics } from "@/hooks/useMonitorEvents";
import { phaseLabels } from "@/lib/checkEvidence";

const labels: Record<string, string> = {
    ...phaseLabels, connection_acquire: "Connection setup", dns: "DNS", tcp: "TCP", tls: "TLS",
    send: "Send request", response_wait: "Response wait",
};

export function HTTPDiagnosticDetails({ diagnostics }: { diagnostics: HTTPDiagnostics }) {
    const attempts = diagnostics.attempts ?? [];
    const attemptTime = attempts.reduce((sum, attempt) => sum + attempt.totalMs, 0);
    const between = diagnostics.totalMs === undefined ? undefined : Math.max(0, diagnostics.totalMs - attemptTime);
    const decisions: Record<string, string> = {
        capacity_limited: "Retry skipped: Warden is busy. Checking again on schedule.",
        budget_exhausted: "Retry skipped: time budget exhausted. Checking again on schedule.",
        not_transient: "No retry: ineligible error or a delay requested by the service.",
        unsafe_request: "No retry: automatic mode requires GET/HEAD without a body.",
    };
    return <section className="space-y-4 min-w-0" aria-label="HTTP diagnostics">
        <div className="flex flex-wrap gap-x-5 gap-y-2 text-xs text-muted-foreground">
            <span>Requests <strong className="font-mono font-medium text-foreground">{attemptTime.toFixed(1)} ms</strong></span>
            {attempts.length > 1 && between !== undefined && <span>Between attempts <strong className="font-mono font-medium text-foreground">{between.toFixed(1)} ms</strong></span>}
            {diagnostics.totalMs !== undefined && <span>Total <strong className="font-mono font-medium text-foreground">{diagnostics.totalMs.toFixed(1)} ms</strong></span>}
        </div>
        {diagnostics.retryDecision && decisions[diagnostics.retryDecision] && <p className="text-xs text-muted-foreground">{decisions[diagnostics.retryDecision]}</p>}
        <div className="space-y-3">
            {attempts.map((attempt, i) => <div key={i} className="rounded-lg border border-border bg-card p-3 sm:p-4 space-y-3">
                <div className="flex flex-wrap items-center justify-between gap-2 text-sm">
                    <h3 className="font-medium">Attempt {i + 1}</h3>
                    <span className={`text-xs font-medium ${attempt.status === "down" || attempt.failurePhase ? "text-rose-700 dark:text-rose-400" : "text-muted-foreground"}`}>{attempt.failurePhase ? labels[attempt.failurePhase] ?? attempt.failurePhase : attempt.status === "up" ? "Passed" : "Recorded"} · {attempt.totalMs.toFixed(1)} ms</span>
                </div>
                {(attempt.hops ?? []).map((hop, j) => {
                    const phases = (hop.phases ?? []).filter(phase => phase.name !== "connection_acquire");
                    const scale = Math.max(hop.totalMs, ...phases.map(phase => phase.startMs + phase.durationMs), 0.001);
                    return <div key={j} className="space-y-3">
                        <div className="flex flex-wrap justify-between gap-2 text-xs text-muted-foreground"><span className="min-w-0 break-all">{j === 0 ? "Request" : `Redirect ${j}`} · {hop.host}</span>{hop.statusCode ? <span className="font-mono text-foreground">HTTP {hop.statusCode}</span> : <span>{labels[hop.failurePhase ?? ""] ?? "No response"}</span>}</div>
                        <dl className="space-y-2" aria-label={`Attempt ${i + 1}, ${j === 0 ? "request" : `redirect ${j}`} phases`}>
                            {phases.map((phase, k) => <div key={k} className="grid grid-cols-[7rem_minmax(0,1fr)_4rem] items-center gap-2 text-xs">
                                <dt className="text-muted-foreground">{labels[phase.name] ?? phase.name}{phase.exchange > 1 ? ` (${phase.exchange})` : ""}</dt>
                                <dd className="relative h-1.5 rounded bg-muted" aria-hidden="true"><span className={`absolute h-full min-w-px rounded ${phase.failed ? "bg-destructive" : "bg-primary"}`} style={{ left: `${Math.min(100, phase.startMs / scale * 100)}%`, width: `${Math.min(100 - phase.startMs / scale * 100, phase.durationMs / scale * 100)}%` }} /></dd>
                                <dd className="text-right font-mono tabular-nums">{phase.durationMs.toFixed(1)} ms</dd>
                            </div>)}
                        </dl>
                        {!phases.length && <p className="text-xs text-muted-foreground">No phase timings recorded.</p>}
                        {(attempt.truncated || hop.truncated) && <p className="text-xs text-muted-foreground">Trace truncated</p>}
                    </div>;
                })}
            </div>)}
        </div>
        <details className="rounded-lg border border-border">
            <summary className="min-h-11 cursor-pointer px-3 py-3 text-xs font-medium focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">Technical details</summary>
            <div className="space-y-4 px-3 pb-4 text-xs">
                <p className="text-muted-foreground">Response wait includes network and service, not backend execution time. Phase times can overlap; missing phases were not observed. Between attempts includes retry backoff and scheduling overhead.</p>
                {diagnostics.retryMode && <p>Retry mode: {diagnostics.retryMode}</p>}
                {attempts.map((attempt, i) => <div key={i} className="space-y-3">
                    <h4 className="font-medium">Attempt {i + 1} connections</h4>
                    {(attempt.hops ?? []).map((hop, j) => <div key={j} className="space-y-2">
                        <p className="break-all text-muted-foreground">{hop.remoteIp ?? "Remote IP unavailable"} · {hop.reused === undefined ? "Connection unavailable" : hop.reused ? "Reused connection" : "New connection"} · {hop.host}</p>
                        <div className="overflow-x-auto"><table className="w-full text-left text-xs"><thead className="text-muted-foreground"><tr><th className="pb-2 pr-3 font-normal">Phase</th><th className="pb-2 pr-3 font-normal">Exchange</th><th className="pb-2 pr-3 font-normal">Start</th><th className="pb-2 pr-3 font-normal">Duration</th><th className="pb-2 font-normal">Result</th></tr></thead><tbody>{(hop.phases ?? []).map((phase, k) => <tr key={k}><td className="py-1 pr-3">{labels[phase.name] ?? phase.name}</td><td className="pr-3">{phase.exchange}</td><td className="pr-3 whitespace-nowrap">{phase.startMs.toFixed(1)} ms</td><td className="pr-3 whitespace-nowrap">{phase.durationMs.toFixed(1)} ms</td><td>{phase.failed ? "Failed" : phase.complete ? "Completed" : "Incomplete"}</td></tr>)}</tbody></table></div>
                    </div>)}
                </div>)}
                {diagnostics.external && <div className="space-y-1 border-t border-border pt-3"><p>Second probe: {diagnostics.external.outcome}{diagnostics.external.statusCode ? ` · HTTP ${diagnostics.external.statusCode}` : ""}{diagnostics.external.failurePhase ? ` · ${labels[diagnostics.external.failurePhase] ?? diagnostics.external.failurePhase}` : ""}</p>{diagnostics.external.observedAt && <p>Observed: {diagnostics.external.observedAt}</p>}<p className="text-muted-foreground">A response confirms reachability, not application health. Probes on the same Internet connection cannot isolate an ISP failure.</p></div>}
            </div>
        </details>
    </section>;
}
