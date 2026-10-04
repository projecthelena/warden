import { MonitorInsight, useMonitorInsights } from "@/hooks/useInsights";
import { Skeleton } from "@/components/ui/skeleton";
import { TrendingUp, TrendingDown, Clock, Repeat, Link2, Waves } from "lucide-react";

const KIND_LABEL: Record<MonitorInsight["kind"], string> = {
    latency_sawtooth: "Climbs and resets",
    periodic_reset: "On a schedule",
    time_of_day: "Time of day",
    co_failure: "Fails with another monitor",
    latency_drift: "Getting slower",
    latency_improved: "Getting faster",
};

const NEXT_STEP: Record<MonitorInsight["kind"], string> = {
    latency_sawtooth: "Compare slow checks with HTTP timings and deployment or restart history.",
    periodic_reset: "Check scheduled jobs and restart history against this cadence.",
    time_of_day: "Compare traffic and scheduled jobs during this time window.",
    co_failure: "Compare failed checks at the same time. Check the monitoring host and shared network before blaming the services.",
    latency_drift: "Compare HTTP timings in slow and normal checks to locate the slowdown.",
    latency_improved: "Check whether the improvement follows a deployment or configuration change.",
};

function observation(insight: MonitorInsight): string {
    const d = insight.detail;
    if (!d) return insight.summary;
    if (insight.kind === "latency_sawtooth" && typeof d.ramps === "number" && typeof d.baselineMs === "number" && typeof d.worstPeakMs === "number") {
        return `${d.ramps} rises followed by a return to normal. Baseline ${d.baselineMs} ms; highest hourly average ${d.worstPeakMs} ms.`;
    }
    if ((insight.kind === "latency_drift" || insight.kind === "latency_improved") && typeof d.previousMedianMs === "number" && typeof d.recentMedianMs === "number") {
        return `Typical hourly latency changed from ${d.previousMedianMs} ms to ${d.recentMedianMs} ms between last week and this week.`;
    }
    if (insight.kind === "periodic_reset" && typeof d.periodHours === "number") {
        return `Latency returns to normal roughly every ${d.periodHours} hours. The cause is not confirmed.`;
    }
    if (insight.kind === "time_of_day" && typeof d.events === "number" && typeof d.share === "number" && typeof d.startHourUTC === "number" && typeof d.endHourUTC === "number") {
        return `${Math.round(d.share * 100)}% of ${d.events} problems occurred between ${String(d.startHourUTC).padStart(2, "0")}:00 and ${String(d.endHourUTC).padStart(2, "0")}:00 UTC.`;
    }
    if (insight.kind === "co_failure" && typeof d.overlap === "number") {
        const repeated = typeof d.matchedOutages === "number" ? `${d.matchedOutages} distinct outages started within 5 minutes of each other. ` : "Simultaneous starts were not recorded for this finding. ";
        return `${repeated}${d.overlap}% of this monitor's downtime overlapped${typeof d.reverseOverlap === "number" ? `; ${d.reverseOverlap}% in the other direction` : ""}.`;
    }
    return insight.summary;
}

function CheckEvidence({ insight }: { insight: MonitorInsight }) {
    const evidence = insight.detail?.checkEvidence;
    if (!evidence || typeof evidence !== "object") return null;
    const e = evidence as Record<string, unknown>;
    if (typeof e.total !== "number" || typeof e.traced !== "number") return null;
    const phases = e.failurePhases && typeof e.failurePhases === "object"
        ? Object.entries(e.failurePhases).filter(([, count]) => typeof count === "number" && count > 0)
        : [];
    return <div className="space-y-2 text-xs text-muted-foreground"><p>
        Window evidence: {e.total.toLocaleString()} checks · {e.traced.toLocaleString()} with HTTP traces.
        {e.traced === 0 && " No HTTP trace evidence is available for this window."}
        {typeof e.recovered === "number" && e.recovered > 0 && ` ${e.recovered} checks recovered on retry.`}
    </p>
        {phases.length > 0 && <p>First-attempt failure phases: {phases.map(([phase, count]) => `${phase.replaceAll("_", " ")}: ${count}`).join(" · ")}. These describe where requests failed, not the root cause.</p>}
    </div>;
}

function KindIcon({ kind }: { kind: MonitorInsight["kind"] }) {
    const cls = "w-4 h-4 text-muted-foreground shrink-0 mt-0.5";
    switch (kind) {
        case "latency_sawtooth":
            return <Waves className={cls} />;
        case "periodic_reset":
            return <Repeat className={cls} />;
        case "time_of_day":
            return <Clock className={cls} />;
        case "co_failure":
            return <Link2 className={cls} />;
        case "latency_improved":
            return <TrendingDown className={cls} />;
        default:
            return <TrendingUp className={cls} />;
    }
}

export function InsightsCard({ monitorId }: { monitorId: string }) {
    const { data, isLoading, error } = useMonitorInsights(monitorId);

    if (isLoading) {
        return <div role="status" aria-label="Loading patterns"><Skeleton className="h-20 w-full" /></div>;
    }
    if (error) {
        return <div role="alert" className="rounded-lg border border-rose-500/30 bg-rose-500/5 p-4 text-sm text-rose-400">Could not load patterns. Please try again later.</div>;
    }
    if (!data || data.length === 0) {
        return <div className="grid min-h-44 place-content-center gap-2 rounded-xl border border-dashed border-border bg-card/30 px-4 text-center text-sm text-muted-foreground">
            <p>No patterns detected yet.</p>
            <p>Patterns are based on the last 14 days of checks and refreshed daily.</p>
        </div>;
    }

    const related = data.filter(insight => insight.kind === "co_failure");
    const individual = data.filter(insight => insight.kind !== "co_failure");
    const patternCount = individual.length + (related.length > 0 ? 1 : 0);

    return (
        <section data-testid="monitor-insights">
            <h2 className="text-sm font-medium text-foreground mb-3 flex items-center gap-2">
                <Waves className="w-4 h-4 text-muted-foreground" />
                Patterns ({patternCount})
            </h2>
            <div className="space-y-2">
                {individual.map(insight => (
                    <div
                        key={insight.id}
                        className="border border-border rounded-lg p-3 bg-muted/20 flex gap-3"
                    >
                        <KindIcon kind={insight.kind} />
                        <div className="min-w-0 space-y-1">
                            <div className="flex items-center gap-2 flex-wrap">
                                <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                                    {KIND_LABEL[insight.kind] ?? insight.kind}
                                </span>
                                {insight.confidence === "medium" && (
                                    <span className="text-[10px] uppercase tracking-wide text-muted-foreground border border-border rounded px-1.5 py-0.5">
                                        worth a look
                                    </span>
                                )}
                            </div>
                            <p className="text-sm text-foreground/90">{observation(insight)}</p>
                            <p className="text-sm text-muted-foreground">{NEXT_STEP[insight.kind]}</p>
                            <details className="text-xs text-muted-foreground">
                                <summary className="cursor-pointer rounded py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">View evidence</summary>
                                <div className="space-y-2 py-2">
                                    {observation(insight) !== insight.summary && <p>{insight.summary}</p>}
                                    <CheckEvidence insight={insight} />
                                    <p>Last analyzed: {new Date(insight.detectedAt).toLocaleString()}</p>
                                </div>
                            </details>
                        </div>
                    </div>
                ))}
                {related.length > 0 && <div className="rounded-lg border border-border bg-muted/20 p-4 space-y-3">
                    <h3 className="text-sm font-medium">Overlapping outages with {related.length} {related.length === 1 ? "monitor" : "monitors"}</h3>
                    <p className="text-sm text-muted-foreground">These monitors overlap with this monitor. A shared cause is not confirmed.</p>
                    <p className="text-sm text-foreground/90">{NEXT_STEP.co_failure}</p>
                    <details>
                        <summary className="cursor-pointer rounded py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">View related monitors</summary>
                        <ul className="divide-y divide-border">
                            {related.map(insight => {
                                const otherId = insight.detail?.withMonitorId;
                                const otherName = insight.detail?.withMonitorName;
                                return <li key={insight.id} className="space-y-2 py-3 text-sm break-words">
                                    {typeof otherId === "string" && <a className="inline-block font-medium text-primary underline underline-offset-4 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring" href={`/monitors/${encodeURIComponent(otherId)}?tab=checks`}>
                                        {typeof otherName === "string" ? otherName : otherId}
                                    </a>}
                                    <p>{observation(insight)}</p>
                                </li>;
                            })}
                        </ul>
                        <CheckEvidence insight={related[0]} />
                        <p className="py-2 text-xs text-muted-foreground">Last analyzed: {new Date(related[0].detectedAt).toLocaleString()}</p>
                    </details>
                </div>}
            </div>
            <a href={`/monitors/${encodeURIComponent(monitorId)}?tab=checks`} className="inline-block rounded py-3 text-sm font-medium text-primary underline underline-offset-4 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">Review checks and HTTP traces</a>
            <p className="text-xs text-muted-foreground mt-2">
                Found by looking at the last 14 days, refreshed daily. Warden reports the shape; what
                causes it is still your call.
            </p>
        </section>
    );
}
