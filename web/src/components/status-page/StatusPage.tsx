import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { Activity, AlertTriangle, ArrowDownCircle, CheckCircle2, Clock, ExternalLink, RefreshCw, XCircle } from "lucide-react";
import { useEffect, useState } from "react";
import { useParams } from "react-router-dom";
import { useMonitorStore, Group, Incident, Monitor } from "@/lib/store";
import { cn, formatDate } from "@/lib/utils";

function StatusHeader({ groups, incidents, title, secondsToUpdate }: { groups: Group[], incidents: Incident[], title: string, secondsToUpdate: number }) {
    // Separate maintenance from other incidents
    const safeIncidents = incidents || [];
    const maintenanceIncidents = safeIncidents.filter(i => i.type === 'maintenance' && i.status !== 'completed' && i.status !== 'resolved');
    const problemIncidents = safeIncidents.filter(i => i.type === 'incident' && i.status !== 'resolved');

    // Check if maintenance is actively happening (started and not ended)
    const now = new Date();
    const activeMaintenance = maintenanceIncidents.filter(i =>
        new Date(i.startTime) <= now && (!i.endTime || new Date(i.endTime) > now)
    );
    const isUnderMaintenance = activeMaintenance.length > 0;

    // Collect IDs of groups under maintenance
    const maintenanceGroupIds = new Set<string>();
    activeMaintenance.forEach(i => {
        if (i.affectedGroups) {
            i.affectedGroups.forEach(gId => maintenanceGroupIds.add(gId));
        }
    });

    // Filter out incidents that belong to maintenance groups
    // If an incident has NO affected groups (global?), we might keep it.
    // If it has affected groups, check if ALL of them are in maintenance.
    const effectiveProblemIncidents = problemIncidents.filter(i => {
        if (!i.affectedGroups || i.affectedGroups.length === 0) return true;
        // If overlap between incident groups and maintenance groups?
        // User says: "si hay un maincnate en el grupo no deberiamos poner evetnos de critital"
        // So if relevant group is in maintenance, hide incident.
        return !i.affectedGroups.some(gId => maintenanceGroupIds.has(gId));
    });

    const hasActiveOutage = effectiveProblemIncidents.length > 0;

    // Check for down monitors, excluding those in maintenance groups
    const hasDown = groups.some(g =>
        !maintenanceGroupIds.has(g.id) && g.monitors.some(m => m.status === 'down')
    );
    const hasDegraded = groups.some(g =>
        !maintenanceGroupIds.has(g.id) && g.monitors.some(m => m.status === 'degraded')
    );

    let statusConfig;

    if (isUnderMaintenance) {
        statusConfig = {
            variant: "default" as const, // Alert variant
            icon: RefreshCw,
            className: "border-blue-500/50 bg-blue-500/5 [&>svg]:text-blue-500",
            message: "System Under Maintenance",
            description: "Scheduled maintenance is currently in progress."
        };
    } else if (hasActiveOutage || hasDown) {
        statusConfig = {
            variant: "default" as const,
            icon: XCircle,
            className: "border-destructive/50 bg-destructive/5 [&>svg]:text-destructive",
            message: "System Outage",
            description: "Some systems are experiencing issues."
        };
    } else if (hasDegraded) {
        statusConfig = {
            variant: "default" as const, // Use default but style as warning? Shadcn usually doesn't have warning variant by default.
            icon: AlertTriangle,
            className: "border-yellow-500/50 bg-yellow-500/5 [&>svg]:text-yellow-500",
            message: "Partially Degraded Service",
            description: "Some monitors are reporting degraded performance."
        };
    } else {
        // Operational default
        statusConfig = {
            variant: "default" as const,
            icon: CheckCircle2,
            className: "border-emerald-500/50 bg-emerald-500/5 [&>svg]:text-emerald-500",
            message: "All Systems Operational",
            description: "All monitors are running normally."
        };
    }

    const Icon = statusConfig.icon;

    return (
        <div className="space-y-8 mb-12">
            <div className="flex items-center gap-3 justify-center pt-8">
                <Activity className="w-6 h-6 text-primary" />
                <h1 className="text-2xl font-bold text-foreground">{title}</h1>
            </div>

            <Alert variant={statusConfig.variant} className={cn("transition-all duration-500", statusConfig.className)}>
                <Icon className="h-4 w-4" />
                <AlertTitle className="text-lg font-semibold flex items-center justify-between">
                    {statusConfig.message}
                    <span className="text-xs font-normal opacity-70 tabular-nums">Refreshing in {secondsToUpdate}s</span>
                </AlertTitle>
                <AlertDescription className="text-sm opacity-90 font-medium">
                    {statusConfig.description}
                </AlertDescription>
            </Alert>
        </div>
    )
}

function PublicMonitor({ monitor, isMaintenance }: { monitor: Monitor, isMaintenance?: boolean }) {
    let statusColor =
        monitor.status === 'up' ? 'bg-emerald-500' :
            monitor.status === 'degraded' ? 'bg-yellow-500' : 'bg-destructive';

    let statusText =
        monitor.status === 'up' ? 'Operational' :
            monitor.status === 'degraded' ? 'Degraded' : 'Down';

    let statusTextColor =
        monitor.status === 'up' ? 'text-emerald-600 dark:text-emerald-500' :
            monitor.status === 'degraded' ? 'text-yellow-600 dark:text-yellow-500' : 'text-destructive';

    if (isMaintenance) {
        statusColor = 'bg-blue-500';
        statusText = 'Maintenance';
        statusTextColor = 'text-blue-600 dark:text-blue-500';
    }

    return (
        <div className="group relative flex flex-col sm:flex-row items-center justify-between p-4 rounded-xl border border-border bg-card hover:bg-accent/50 transition-all duration-300 gap-4 overflow-hidden">
            <div className="flex items-center justify-between w-full sm:w-auto gap-4 pl-1">
                <div className="space-y-1">
                    <div className="font-medium text-foreground flex items-center gap-2">
                        {monitor.name}
                        {monitor.url && (
                            <a href={monitor.url} target="_blank" rel="noopener noreferrer" className="opacity-0 group-hover:opacity-50 transition-opacity hover:!opacity-100">
                                <ExternalLink className="w-3 h-3 text-muted-foreground" />
                            </a>
                        )}
                    </div>
                </div>
                <div className="flex items-center gap-2 sm:hidden">
                    <span className={`w-2 h-2 rounded-full ${statusColor}`} />
                    <span className={`text-sm font-medium ${statusTextColor}`}>
                        {statusText}
                    </span>
                </div>
            </div>

            <div className="hidden sm:flex items-center gap-2.5 min-w-[140px] justify-end">
                <div className={`text-sm font-medium ${statusTextColor} transition-colors`}>
                    {statusText}
                </div>
                <div className="relative flex items-center justify-center">
                    {monitor.status !== 'up' && !isMaintenance && (
                        <span className={`absolute inline-flex h-full w-full rounded-full ${statusColor} opacity-75 animate-ping`} />
                    )}
                    <span className={`relative inline-flex rounded-full h-2.5 w-2.5 ${statusColor}`} />
                </div>
            </div>
        </div>
    )
}

function MaintenanceItem({ incident }: { incident: Incident }) {
    const start = new Date(incident.startTime);
    const end = incident.endTime ? new Date(incident.endTime) : null;
    const now = new Date();
    const isOngoing = now >= start && (!end || now < end);

    // Apply similar grouping style as IncidentItem but with blue theme
    const bgRow = "bg-blue-500/5 border-blue-500/20";
    const bgBadge = "bg-blue-500/10 text-blue-500 border-0";
    const textBase = "text-blue-600 dark:text-blue-400";

    return (
        <div className={cn("group flex items-center justify-between py-4 px-6 rounded-xl border transition-colors", bgRow)}>
            <div className="flex items-center gap-4">
                {/* Icon */}
                <div className="flex items-center justify-center w-10 h-10 rounded-full bg-background border border-blue-500/30 shadow-sm text-blue-500">
                    <RefreshCw className="w-5 h-5 animate-spin-slow" />
                </div>

                <div className="space-y-0.5">
                    <div className="flex items-center gap-2 text-sm">
                        <span className="font-semibold text-foreground text-base">
                            {incident.title}
                        </span>
                        {isOngoing ? (
                            <Badge variant="secondary" className={cn("ml-2 rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider h-5 animate-pulse", bgBadge)}>
                                Ongoing
                            </Badge>
                        ) : (
                            <Badge variant="outline" className="ml-2 rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider h-5 text-blue-500 border-blue-500/50">
                                Scheduled
                            </Badge>
                        )}
                    </div>
                    <div className="text-sm text-muted-foreground">
                        {incident.description}
                    </div>
                </div>
            </div>

            <div className="flex items-center gap-4 text-xs text-muted-foreground tabular-nums font-mono opacity-80">
                <span>{formatDate(incident.startTime)}</span>
                {incident.endTime && (
                    <>
                        <span>-</span>
                        <span>{formatDate(incident.endTime)}</span>
                    </>
                )}
            </div>
        </div>
    )
}

function IncidentItem({ incident }: { incident: Incident }) {
    // Attempt to parse group from Title often formatted as "Service Disruption: [Monitor]"
    let title = incident.title.replace("Service Disruption: ", "");
    let group = ""; // Might need access to Groups map to find group name properly, or rely on synthesized ID?

    // For manual incidents, title is just Title.
    // We can try to guess status.

    const isDown = incident.severity === 'critical';
    const colorClass = isDown ? "text-red-500" : "text-yellow-500";
    const bgBadge = isDown ? "bg-red-500/10 text-red-500 hover:bg-red-500/20" : "bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20";
    const bgRow = isDown ? "bg-destructive/5 border-destructive/20 hover:bg-destructive/10" : "bg-yellow-500/5 border-yellow-500/20 hover:bg-yellow-500/10";

    // Calculate duration
    const start = new Date(incident.startTime).getTime();
    const now = new Date().getTime();
    const diffMins = Math.floor((now - start) / 60000);
    let durationStr = diffMins < 1 ? "Just now" : `${diffMins}m ongoing`;
    if (diffMins >= 60) {
        const h = Math.floor(diffMins / 60);
        const m = diffMins % 60;
        durationStr = `${h}h ${m}m ongoing`;
    }

    return (
        <div className={cn("group flex items-center justify-between py-4 px-6 rounded-xl border transition-colors", bgRow)}>
            <div className="flex items-center gap-4">
                {/* Icon */}
                <div className={cn("flex items-center justify-center w-10 h-10 rounded-full bg-background border border-border/50 shadow-sm")}>
                    {isDown ? (
                        <ArrowDownCircle className={cn("w-5 h-5", colorClass)} />
                    ) : (
                        <AlertTriangle className={cn("w-5 h-5", colorClass)} />
                    )}
                </div>

                <div className="space-y-0.5">
                    <div className="flex items-center gap-2 text-sm">
                        <span className="font-semibold text-foreground text-base">
                            {title}
                        </span>
                        <Badge variant="secondary" className={cn("ml-2 rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider border-0 h-5", bgBadge)}>
                            {isDown ? 'UNAVAILABLE' : 'ISSUE'}
                        </Badge>
                    </div>
                    <div className="text-sm text-muted-foreground">
                        {incident.description}
                    </div>
                </div>
            </div>

            <div className="flex items-center gap-4 text-xs text-muted-foreground tabular-nums">
                <span className={cn("font-medium text-sm hidden sm:block", colorClass)}>
                    {durationStr}
                </span>
            </div>
        </div>
    )
}

function StatusSkeleton() {
    return (
        <div className="min-h-screen bg-background flex flex-col items-center pt-32 px-4">
            <div className="w-16 h-16 rounded-full bg-muted animate-pulse mb-8" />
            <div className="h-8 w-48 bg-muted rounded animate-pulse mb-12" />
            <div className="w-full max-w-3xl space-y-4">
                {[1, 2, 3].map(i => (
                    <div key={i} className="h-20 w-full bg-muted/50 rounded-xl animate-pulse" />
                ))}
            </div>
        </div>
    )
}

export function StatusPage() {
    const { slug } = useParams();
    const { fetchPublicStatusBySlug } = useMonitorStore();
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [data, setData] = useState<{ title: string, groups: Group[], incidents: Incident[] } | null>(null);
    const [secondsToUpdate, setSecondsToUpdate] = useState(60);

    useEffect(() => {
        let isMounted = true;

        const load = async (isBackground = false) => {
            if (!isBackground) setLoading(true);
            const result = await fetchPublicStatusBySlug(slug || 'all');

            if (isMounted) {
                if (result) {
                    setData(result);
                    setError(null);
                } else {
                    setError("Status page not found or private.");
                }
                setLoading(false);
                if (result) setSecondsToUpdate(60); // Reset timer only on successful fetch
            }
        };

        load();

        const pollInterval = setInterval(() => {
            load(true);
        }, 60000);

        const timerInterval = setInterval(() => {
            setSecondsToUpdate(prev => Math.max(0, prev - 1));
        }, 1000);

        return () => {
            isMounted = false;
            clearInterval(pollInterval);
            clearInterval(timerInterval);
        };
    }, [slug, fetchPublicStatusBySlug]);

    if (loading && !data) return <StatusSkeleton />;

    if (error || !data) {
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
        )
    }

    const { groups, incidents = [], title } = data;

    // Calculate active maintenance to filter out incidents
    const now = new Date();
    const maintenanceIncidents = (incidents || []).filter(i => i.type === 'maintenance' && i.status !== 'completed' && i.status !== 'resolved');
    const activeMaintenance = maintenanceIncidents.filter(i =>
        new Date(i.startTime) <= now && (!i.endTime || new Date(i.endTime) > now)
    );
    const maintenanceGroupIds = new Set<string>();
    activeMaintenance.forEach(i => {
        if (i.affectedGroups) {
            i.affectedGroups.forEach(gId => maintenanceGroupIds.add(gId));
        }
    });

    const incidentItems = (incidents || []).filter(i => {
        if (i.type !== 'incident' || i.status === 'resolved') return false;
        // Filter out if it belongs to a maintenance group
        if (i.affectedGroups && i.affectedGroups.length > 0) {
            // If overlap
            return !i.affectedGroups.some(gId => maintenanceGroupIds.has(gId));
        }
        return true;
    });

    return (
        <div className="min-h-screen bg-background text-foreground font-sans flex flex-col">
            <main className="relative max-w-4xl mx-auto px-6 pb-20 w-full flex-1">
                <StatusHeader groups={groups} incidents={incidents || []} title={title} secondsToUpdate={secondsToUpdate} />

                {/* Maintenance Section: Show ALL maintenance (future + active) */}
                {maintenanceIncidents.length > 0 && (
                    <div className="mb-12 animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards p-1">
                        <div className="flex items-center gap-2 mb-3 px-1">
                            {/* <RefreshCw className="w-4 h-4 text-blue-500" /> */}
                            <h2 className="text-xs font-semibold text-blue-500 uppercase tracking-widest pl-1">Scheduled Maintenance</h2>
                        </div>
                        <div className="space-y-4">
                            {maintenanceIncidents.map(i => <MaintenanceItem key={i.id} incident={i} />)}
                        </div>
                    </div>
                )}

                {/* Active Incidents Section */}
                {incidentItems.length > 0 && (
                    <div className="mb-12 animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards p-1">
                        {/* Critical Outages Headers */}
                        <div className="flex items-center gap-2 mb-3 px-1">
                            <h2 className="text-xs font-semibold text-red-500 uppercase tracking-widest pl-1">Critical Outages</h2>
                        </div>
                        <div className="space-y-3">
                            {incidentItems.map(i => <IncidentItem key={i.id} incident={i} />)}
                        </div>
                    </div>
                )}

                {/* Groups & Monitors */}
                <div className="space-y-10">
                    {groups && groups.map((group, idx) => {
                        const now = new Date();
                        const isGroupMaintenance = incidents && incidents.some(i =>
                            i.type === 'maintenance' &&
                            i.status !== 'completed' &&
                            i.affectedGroups.includes(group.id) &&
                            new Date(i.startTime) <= now && (!i.endTime || new Date(i.endTime) > now)
                        );

                        return (
                            <div key={group.id}
                                className="space-y-4 animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards"
                                style={{ animationDelay: `${idx * 100}ms` }}
                            >
                                <div className="flex items-center justify-between px-2">
                                    <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{group.name}</h3>
                                </div>
                                <div className="space-y-3">
                                    {group.monitors.map(m => <PublicMonitor key={m.id} monitor={m} isMaintenance={isGroupMaintenance} />)}
                                </div>
                            </div>
                        )
                    })}
                </div>
            </main>

            <footer className="relative border-t border-border mt-auto py-8 text-center bg-card/30">
                <div className="text-muted-foreground text-sm flex items-center justify-center gap-2 hover:text-foreground transition-colors">
                    <span>Powered by</span>
                    <a href="https://clusteruptime.com/" target="_blank" rel="noopener noreferrer" className="font-semibold text-foreground/80 hover:text-foreground hover:underline underline-offset-4 transition-all">
                        ClusterUptime
                    </a>
                </div>
            </footer>
        </div>
    )
}
