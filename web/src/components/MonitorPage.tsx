import { useMemo } from "react";
import { useParams, useSearchParams, useNavigate } from "react-router-dom";
import { useMonitorsQuery } from "@/hooks/useMonitors";
import { useMonitorStore, SystemIncident } from "@/lib/store";
import { useFilteredSystemEvents } from "@/hooks/useSystemEvents";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { StatusBadge } from "@/components/ui/monitor-visuals";
import { Skeleton } from "@/components/ui/skeleton";
import { IncidentCard } from "@/components/IncidentCard";
import { Activity, Calendar, ExternalLink } from "lucide-react";

const fmtDateInput = (d: Date) => d.toISOString().slice(0, 10);

// MonitorPage shows incident rollups (outages) for a single monitor on a given UTC day.
// Each row is one downtime/degraded period with its duration; expand to see the enriched
// check events that fell inside that window. The page chrome (breadcrumb, sidebar group
// highlight) is handled by App.tsx so this component only owns the body.
export function MonitorPage() {
    const { id } = useParams<{ id: string }>();
    const [searchParams, setSearchParams] = useSearchParams();
    const navigate = useNavigate();
    const { groups } = useMonitorStore();
    const { isLoading: monitorsLoading } = useMonitorsQuery();

    const date = searchParams.get("date") || fmtDateInput(new Date());

    const monitor = useMemo(() => {
        for (const g of groups || []) {
            const m = g.monitors?.find(mm => mm.id === id);
            if (m) return m;
        }
        return null;
    }, [groups, id]);

    // Per-monitor + per-date filtered system events. The hook keeps the result local so
    // the global /api/events poll (no filters) can't overwrite us.
    const incidentsQuery = useFilteredSystemEvents({ monitorId: id, date });

    const handleDateChange = (newDate: string) => {
        const next = new URLSearchParams(searchParams);
        if (!newDate) {
            next.delete("date");
        } else {
            next.set("date", newDate);
        }
        setSearchParams(next, { replace: true });
    };

    if (!id) {
        return <div className="text-center py-12 text-muted-foreground">Monitor not found.</div>;
    }

    // Merge active + history + ssl warnings into one chronologically-ordered list. Active
    // outages are sorted to the top (rose pulse), then history newest first.
    const incidents: SystemIncident[] = [
        ...(incidentsQuery.data?.active ?? []),
        ...(incidentsQuery.data?.history ?? []),
    ];
    const sslWarnings = incidentsQuery.data?.sslWarnings ?? [];

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between gap-4">
                <div className="min-w-0">
                    <div className="text-2xl font-semibold truncate">
                        {monitorsLoading && !monitor ? <Skeleton className="h-7 w-48" /> : (monitor?.name ?? id)}
                    </div>
                    {monitor && (
                        <a
                            href={monitor.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-xs text-muted-foreground hover:text-foreground font-mono truncate inline-flex items-center gap-1 mt-0.5"
                        >
                            {monitor.url}
                            <ExternalLink className="w-3 h-3" />
                        </a>
                    )}
                </div>
                {monitor && <StatusBadge status={monitor.status} />}
            </div>

            <div className="flex items-end gap-3 border-b border-border pb-4">
                <div className="flex flex-col gap-1.5">
                    <Label htmlFor="date-picker" className="text-xs text-muted-foreground flex items-center gap-1.5">
                        <Calendar className="w-3 h-3" />
                        Incidents on date (UTC)
                    </Label>
                    <Input
                        id="date-picker"
                        type="date"
                        value={date}
                        onChange={(e) => handleDateChange(e.target.value)}
                        className="w-44"
                    />
                </div>
                <Button variant="outline" size="sm" onClick={() => handleDateChange(fmtDateInput(new Date()))}>
                    Today
                </Button>
                <Button variant="ghost" size="sm" onClick={() => navigate(`/incidents?monitorId=${id}`)}>
                    All incidents for this monitor
                </Button>
            </div>

            <section>
                <h2 className="text-sm font-medium text-foreground mb-3 flex items-center gap-2">
                    <Activity className="w-4 h-4 text-muted-foreground" />
                    Incidents ({incidents.length + sslWarnings.length})
                </h2>

                {incidentsQuery.isLoading ? (
                    <div className="space-y-2">
                        {[...Array(3)].map((_, i) => <Skeleton key={i} className="h-14 w-full" />)}
                    </div>
                ) : incidentsQuery.error ? (
                    <div className="text-sm text-rose-500 border border-rose-500/30 rounded-lg p-3 bg-rose-500/5">
                        Failed to load incidents: {(incidentsQuery.error as Error).message}
                    </div>
                ) : incidents.length === 0 && sslWarnings.length === 0 ? (
                    <div className="text-center py-12 text-muted-foreground text-sm border border-dashed border-border rounded-lg">
                        No incidents recorded on {date}.
                    </div>
                ) : (
                    <div className="space-y-2">
                        {incidents.map(inc => (
                            <IncidentCard
                                key={inc.id}
                                monitorId={inc.monitorId}
                                monitorName={inc.monitorName}
                                groupName={inc.groupName}
                                onMonitorPage
                                type={inc.type as "down" | "degraded"}
                                summary={inc.message}
                                startedAt={inc.startedAt}
                                endedAt={inc.resolvedAt}
                                duration={inc.duration ?? "0m"}
                            />
                        ))}
                        {sslWarnings.map(w => (
                            <IncidentCard
                                key={`ssl-${w.id}`}
                                monitorId={w.monitorId}
                                monitorName={w.monitorName}
                                groupName={w.groupName}
                                onMonitorPage
                                type="ssl_expiring"
                                summary={w.message}
                                startedAt={w.timestamp}
                                duration=""
                            />
                        ))}
                    </div>
                )}
            </section>
        </div>
    );
}
