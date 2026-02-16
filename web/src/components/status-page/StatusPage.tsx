import { Badge } from "@/components/ui/badge";
import { Activity, AlertTriangle, ArrowDownCircle, CheckCircle2, ExternalLink, RefreshCw, XCircle } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { useMonitorStore, Group, Incident, Monitor } from "@/lib/store";
import { cn, formatDate } from "@/lib/utils";
import { UptimeBar } from "./UptimeBar";

// ---------- Types ----------

interface DayData {
    date: string;
    uptimePercent: number;
    totalChecks: number;
}

interface StatusMonitor extends Monitor {
    uptimeDays?: DayData[];
    overallUptime?: number;
}

interface StatusGroup extends Omit<Group, "monitors"> {
    monitors: StatusMonitor[];
}

// ---------- Helpers ----------

function getMaintenanceState(incidents: Incident[]) {
    const now = new Date();
    const maintenanceIncidents = (incidents || []).filter(
        (i) => i.type === "maintenance" && i.status !== "completed" && i.status !== "resolved"
    );
    const activeMaintenance = maintenanceIncidents.filter(
        (i) => new Date(i.startTime) <= now && (!i.endTime || new Date(i.endTime) > now)
    );
    const maintenanceGroupIds = new Set<string>();
    activeMaintenance.forEach((i) => {
        i.affectedGroups?.forEach((gId) => maintenanceGroupIds.add(gId));
    });
    return { maintenanceIncidents, activeMaintenance, maintenanceGroupIds };
}

function getOverallStatus(groups: StatusGroup[], incidents: Incident[], maintenanceGroupIds: Set<string>) {
    const effectiveIncidents = (incidents || []).filter((i) => {
        if (i.type !== "incident" || i.status === "resolved") return false;
        if (!i.affectedGroups || i.affectedGroups.length === 0) return true;
        return !i.affectedGroups.some((gId) => maintenanceGroupIds.has(gId));
    });

    const hasActiveOutage = effectiveIncidents.length > 0;
    const hasDown = groups.some(
        (g) => !maintenanceGroupIds.has(g.id) && g.monitors.some((m) => m.status === "down")
    );
    const hasDegraded = groups.some(
        (g) => !maintenanceGroupIds.has(g.id) && g.monitors.some((m) => m.status === "degraded")
    );
    const isUnderMaintenance = maintenanceGroupIds.size > 0;

    if (isUnderMaintenance && !hasActiveOutage && !hasDown) {
        return {
            icon: RefreshCw,
            label: "System Under Maintenance",
            description: "Scheduled maintenance is currently in progress.",
            color: "blue" as const,
        };
    }
    if (hasActiveOutage || hasDown) {
        return {
            icon: XCircle,
            label: "System Outage",
            description: "Some systems are experiencing issues.",
            color: "red" as const,
        };
    }
    if (hasDegraded) {
        return {
            icon: AlertTriangle,
            label: "Partially Degraded Service",
            description: "Some monitors are reporting degraded performance.",
            color: "yellow" as const,
        };
    }
    return {
        icon: CheckCircle2,
        label: "All Systems Operational",
        description: "All monitors are running normally.",
        color: "green" as const,
    };
}

const statusColorMap = {
    green: {
        banner: "border-emerald-500/30 bg-emerald-500/5",
        icon: "text-emerald-500",
        dot: "bg-emerald-500",
    },
    yellow: {
        banner: "border-yellow-500/30 bg-yellow-500/5",
        icon: "text-yellow-500",
        dot: "bg-yellow-500",
    },
    red: {
        banner: "border-red-500/30 bg-red-500/5",
        icon: "text-red-500",
        dot: "bg-red-500",
    },
    blue: {
        banner: "border-blue-500/30 bg-blue-500/5",
        icon: "text-blue-500",
        dot: "bg-blue-500",
    },
};

// ---------- Sub-Components ----------

function StatusBanner({
    status,
    secondsToUpdate,
}: {
    status: ReturnType<typeof getOverallStatus>;
    secondsToUpdate: number;
}) {
    const colors = statusColorMap[status.color];
    const Icon = status.icon;
    return (
        <div
            className={cn(
                "flex items-center gap-3 px-5 py-4 rounded-xl border transition-all duration-500",
                colors.banner
            )}
        >
            <Icon className={cn("h-5 w-5 shrink-0", colors.icon)} />
            <div className="flex-1 min-w-0">
                <p className={cn("text-sm font-semibold", colors.icon)}>{status.label}</p>
                <p className="text-xs text-muted-foreground">{status.description}</p>
            </div>
            <span className="text-[11px] text-muted-foreground/60 tabular-nums whitespace-nowrap">
                {secondsToUpdate}s
            </span>
        </div>
    );
}

function MaintenanceCard({ incident }: { incident: Incident }) {
    const start = new Date(incident.startTime);
    const end = incident.endTime ? new Date(incident.endTime) : null;
    const now = new Date();
    const isOngoing = now >= start && (!end || now < end);

    return (
        <div className="flex items-center justify-between py-3 px-4 rounded-lg border border-blue-500/20 bg-blue-500/5 gap-4">
            <div className="flex items-center gap-3 min-w-0">
                <div className="flex items-center justify-center w-8 h-8 rounded-full bg-background border border-blue-500/30 text-blue-500 shrink-0">
                    <RefreshCw className="w-4 h-4 animate-spin-slow" />
                </div>
                <div className="min-w-0">
                    <div className="flex items-center gap-2">
                        <span className="font-medium text-sm text-foreground truncate">{incident.title}</span>
                        {isOngoing ? (
                            <Badge
                                variant="secondary"
                                className="bg-blue-500/10 text-blue-500 border-0 rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider h-5 animate-pulse shrink-0"
                            >
                                Ongoing
                            </Badge>
                        ) : (
                            <Badge
                                variant="outline"
                                className="text-blue-500 border-blue-500/50 rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider h-5 shrink-0"
                            >
                                Scheduled
                            </Badge>
                        )}
                    </div>
                    {incident.description && (
                        <p className="text-xs text-muted-foreground truncate mt-0.5">{incident.description}</p>
                    )}
                </div>
            </div>
            <div className="text-[11px] text-muted-foreground tabular-nums font-mono whitespace-nowrap hidden sm:block">
                {formatDate(incident.startTime)}
                {incident.endTime && (
                    <> &mdash; {formatDate(incident.endTime)}</>
                )}
            </div>
        </div>
    );
}

function IncidentCard({ incident }: { incident: Incident }) {
    const title = incident.title.replace("Service Disruption: ", "");
    const isDown = incident.severity === "critical";
    const colorClass = isDown ? "text-red-500" : "text-yellow-500";
    const bgBadge = isDown
        ? "bg-red-500/10 text-red-500"
        : "bg-yellow-500/10 text-yellow-500";
    const borderClass = isDown
        ? "border-red-500/20 bg-red-500/5"
        : "border-yellow-500/20 bg-yellow-500/5";

    const start = new Date(incident.startTime).getTime();
    const now = new Date().getTime();
    const diffMins = Math.floor((now - start) / 60000);
    let durationStr = diffMins < 1 ? "Just now" : `${diffMins}m`;
    if (diffMins >= 60) {
        const h = Math.floor(diffMins / 60);
        const m = diffMins % 60;
        durationStr = `${h}h ${m}m`;
    }

    return (
        <div className={cn("flex items-center justify-between py-3 px-4 rounded-lg border gap-4", borderClass)}>
            <div className="flex items-center gap-3 min-w-0">
                <div className="flex items-center justify-center w-8 h-8 rounded-full bg-background border border-border/50 shrink-0">
                    {isDown ? (
                        <ArrowDownCircle className={cn("w-4 h-4", colorClass)} />
                    ) : (
                        <AlertTriangle className={cn("w-4 h-4", colorClass)} />
                    )}
                </div>
                <div className="min-w-0">
                    <div className="flex items-center gap-2">
                        <span className="font-medium text-sm text-foreground truncate">{title}</span>
                        <Badge
                            variant="secondary"
                            className={cn(
                                "rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider border-0 h-5 shrink-0",
                                bgBadge
                            )}
                        >
                            {isDown ? "Unavailable" : "Issue"}
                        </Badge>
                    </div>
                    {incident.description && (
                        <p className="text-xs text-muted-foreground truncate mt-0.5">{incident.description}</p>
                    )}
                </div>
            </div>
            <span className={cn("text-xs font-medium whitespace-nowrap hidden sm:block", colorClass)}>
                {durationStr}
            </span>
        </div>
    );
}

function MonitorRow({
    monitor,
    isMaintenance,
}: {
    monitor: StatusMonitor;
    isMaintenance?: boolean;
}) {
    let statusColor = "bg-emerald-500";
    let statusLabel = "Operational";
    if (isMaintenance) {
        statusColor = "bg-blue-500";
        statusLabel = "Maintenance";
    } else if (monitor.status === "degraded") {
        statusColor = "bg-yellow-500";
        statusLabel = "Degraded";
    } else if (monitor.status === "down") {
        statusColor = "bg-red-500";
        statusLabel = "Down";
    } else if (monitor.status === "paused") {
        statusColor = "bg-muted-foreground/50";
        statusLabel = "Paused";
    }

    const uptimeDays = monitor.uptimeDays || [];
    const overallUptime = monitor.overallUptime ?? 100;

    return (
        <div className="group px-4 py-3.5 border-b border-border/50 last:border-b-0 transition-colors hover:bg-accent/30">
            {/* Top row: name + status badge */}
            <div className="flex items-center justify-between gap-3 mb-2">
                <div className="flex items-center gap-2 min-w-0">
                    <span className="font-medium text-sm text-foreground truncate">{monitor.name}</span>
                    {monitor.url && (
                        <a
                            href={monitor.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="opacity-0 group-hover:opacity-50 transition-opacity hover:!opacity-100 shrink-0"
                        >
                            <ExternalLink className="w-3 h-3 text-muted-foreground" />
                        </a>
                    )}
                </div>
                <div className="flex items-center gap-1.5 shrink-0">
                    <span className="text-xs text-muted-foreground hidden sm:inline">{statusLabel}</span>
                    <div className="relative flex items-center justify-center">
                        {monitor.status === "down" && !isMaintenance && (
                            <span
                                className={cn(
                                    "absolute inline-flex h-full w-full rounded-full opacity-75 animate-ping",
                                    statusColor
                                )}
                            />
                        )}
                        <span className={cn("relative inline-flex rounded-full h-2 w-2", statusColor)} />
                    </div>
                </div>
            </div>

            {/* Uptime bar */}
            {uptimeDays.length > 0 && (
                <UptimeBar days={uptimeDays} overallUptime={overallUptime} />
            )}
        </div>
    );
}

function GroupSection({
    group,
    incidents,
    index,
}: {
    group: StatusGroup;
    incidents: Incident[];
    index: number;
}) {
    const now = new Date();
    const isGroupMaintenance =
        incidents &&
        incidents.some(
            (i) =>
                i.type === "maintenance" &&
                i.status !== "completed" &&
                i.affectedGroups?.includes(group.id) &&
                new Date(i.startTime) <= now &&
                (!i.endTime || new Date(i.endTime) > now)
        );

    return (
        <div
            className="animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards"
            style={{ animationDelay: `${index * 80}ms` }}
        >
            <div className="mb-2 px-1">
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    {group.name}
                </h3>
            </div>
            <div className="rounded-xl border border-border bg-card overflow-hidden">
                {group.monitors.map((m) => (
                    <MonitorRow key={m.id} monitor={m} isMaintenance={isGroupMaintenance} />
                ))}
                {group.monitors.length === 0 && (
                    <div className="px-4 py-6 text-center text-sm text-muted-foreground">
                        No monitors configured
                    </div>
                )}
            </div>
        </div>
    );
}

function StatusSkeleton() {
    return (
        <div className="min-h-screen bg-background flex flex-col items-center pt-24 px-4">
            <div className="w-12 h-12 rounded-full bg-muted animate-pulse mb-4" />
            <div className="h-6 w-40 bg-muted rounded animate-pulse mb-10" />
            <div className="w-full max-w-3xl space-y-3">
                <div className="h-16 w-full bg-muted/50 rounded-xl animate-pulse" />
                <div className="h-48 w-full bg-muted/30 rounded-xl animate-pulse" />
                <div className="h-48 w-full bg-muted/30 rounded-xl animate-pulse" />
            </div>
        </div>
    );
}

// ---------- Main Component ----------

export function StatusPage() {
    const { slug } = useParams();
    const { fetchPublicStatusBySlug } = useMonitorStore();
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [data, setData] = useState<{
        title: string;
        groups: StatusGroup[];
        incidents: Incident[];
    } | null>(null);
    const [secondsToUpdate, setSecondsToUpdate] = useState(60);

    useEffect(() => {
        let isMounted = true;

        const load = async (isBackground = false) => {
            if (!isBackground) setLoading(true);
            const result = await fetchPublicStatusBySlug(slug || "all");

            if (isMounted) {
                if (result) {
                    setData(result);
                    setError(null);
                } else {
                    setError("Status page not found or private.");
                }
                setLoading(false);
                if (result) setSecondsToUpdate(60);
            }
        };

        load();

        const pollInterval = setInterval(() => load(true), 60000);
        const timerInterval = setInterval(() => {
            setSecondsToUpdate((prev) => Math.max(0, prev - 1));
        }, 1000);

        return () => {
            isMounted = false;
            clearInterval(pollInterval);
            clearInterval(timerInterval);
        };
    }, [slug, fetchPublicStatusBySlug]);

    // Computed state
    const computed = useMemo(() => {
        if (!data) return null;
        const { groups, incidents = [] } = data;
        const { maintenanceIncidents, maintenanceGroupIds } = getMaintenanceState(incidents);
        const status = getOverallStatus(groups, incidents, maintenanceGroupIds);

        const incidentItems = (incidents || []).filter((i) => {
            if (i.type !== "incident" || i.status === "resolved") return false;
            if (i.affectedGroups && i.affectedGroups.length > 0) {
                return !i.affectedGroups.some((gId) => maintenanceGroupIds.has(gId));
            }
            return true;
        });

        return { groups, incidents, maintenanceIncidents, incidentItems, status };
    }, [data]);

    if (loading && !data) return <StatusSkeleton />;

    if (error || !data || !computed) {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center text-foreground">
                <div className="text-center space-y-4">
                    <div className="relative inline-block">
                        <div className="absolute inset-0 bg-destructive/20 blur-xl rounded-full" />
                        <Activity className="relative w-16 h-16 text-muted-foreground mx-auto" />
                    </div>
                    <h1 className="text-2xl font-bold">Status Page Unavailable</h1>
                    <p className="text-muted-foreground">{error || "Could not load status information."}</p>
                </div>
            </div>
        );
    }

    const { groups, incidents, maintenanceIncidents, incidentItems, status } = computed;

    return (
        <div className="min-h-screen bg-background text-foreground font-sans flex flex-col">
            <main className="max-w-3xl mx-auto px-4 sm:px-6 pb-16 w-full flex-1">
                {/* Header */}
                <div className="flex items-center gap-3 justify-center pt-10 pb-8">
                    <Activity className="w-6 h-6 text-primary" />
                    <h1 className="text-xl font-bold text-foreground">{data.title}</h1>
                </div>

                {/* Status Banner */}
                <div className="mb-6 animate-in fade-in duration-500">
                    <StatusBanner status={status} secondsToUpdate={secondsToUpdate} />
                </div>

                {/* Alerts: Maintenance & Incidents */}
                {(maintenanceIncidents.length > 0 || incidentItems.length > 0) && (
                    <div className="mb-8 space-y-6 animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards">
                        {maintenanceIncidents.length > 0 && (
                            <div>
                                <h2 className="text-xs font-semibold text-blue-500 uppercase tracking-widest mb-2 px-1">
                                    Scheduled Maintenance
                                </h2>
                                <div className="space-y-2">
                                    {maintenanceIncidents.map((i) => (
                                        <MaintenanceCard key={i.id} incident={i} />
                                    ))}
                                </div>
                            </div>
                        )}

                        {incidentItems.length > 0 && (
                            <div>
                                <h2 className="text-xs font-semibold text-red-500 uppercase tracking-widest mb-2 px-1">
                                    Active Incidents
                                </h2>
                                <div className="space-y-2">
                                    {incidentItems.map((i) => (
                                        <IncidentCard key={i.id} incident={i} />
                                    ))}
                                </div>
                            </div>
                        )}
                    </div>
                )}

                {/* Monitor Groups */}
                <div className="space-y-8">
                    {groups.map((group, idx) => (
                        <GroupSection key={group.id} group={group} incidents={incidents} index={idx} />
                    ))}
                </div>
            </main>

            {/* Footer */}
            <footer className="border-t border-border mt-auto py-6 text-center bg-card/30">
                <div className="text-muted-foreground text-xs flex items-center justify-center gap-1.5">
                    <span>Powered by</span>
                    <a
                        href="https://projecthelena.com/"
                        target="_blank"
                        rel="noopener noreferrer"
                        className="font-semibold text-foreground/80 hover:text-foreground hover:underline underline-offset-4 transition-all"
                    >
                        Warden
                    </a>
                </div>
            </footer>
        </div>
    );
}
