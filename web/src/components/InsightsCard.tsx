import { MonitorInsight, useMonitorInsights } from "@/hooks/useInsights";
import { Skeleton } from "@/components/ui/skeleton";
import { TrendingUp, TrendingDown, Clock, Repeat, Link2, Waves } from "lucide-react";

const KIND_LABEL: Record<MonitorInsight["kind"], string> = {
    latency_sawtooth: "Climbs and resets",
    periodic_reset: "On a schedule",
    time_of_day: "Time of day",
    repeat_offender: "Breaks often",
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

// InsightsCard shows what Warden noticed about the *shape* of a monitor's behaviour, as
// opposed to the incidents below it. It renders nothing at all when there is nothing to
// say — a panel that is permanently empty trains people to stop looking at it.
export function InsightsCard({ monitorId }: { monitorId: string }) {
    const { data, isLoading, error } = useMonitorInsights(monitorId);

    if (isLoading) {
        return <Skeleton className="h-20 w-full" />;
    }
    if (error || !data || data.length === 0) {
        return null;
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
