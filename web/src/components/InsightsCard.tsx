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

    return (
        <section data-testid="monitor-insights">
            <h2 className="text-sm font-medium text-foreground mb-3 flex items-center gap-2">
                <Waves className="w-4 h-4 text-muted-foreground" />
                Patterns ({data.length})
            </h2>
            <div className="space-y-2">
                {data.map(insight => (
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
                            <p className="text-sm text-foreground/90">{insight.summary}</p>
                        </div>
                    </div>
                ))}
            </div>
            <p className="text-xs text-muted-foreground mt-2">
                Found by looking at the last 14 days, refreshed daily. Warden reports the shape; what
                causes it is still your call.
            </p>
        </section>
    );
}
