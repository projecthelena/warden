import { useEffect, useState } from "react";
import { Monitor, Group, Incident, useMonitorStore } from "@/lib/store";
import { CheckCircle2, AlertTriangle, XCircle, Activity, ExternalLink, RefreshCw } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { useParams } from "react-router-dom";
import { cn } from "@/lib/utils";

function StatusHeader({ groups, incidents, title, secondsToUpdate }: { groups: Group[], incidents: Incident[], title: string, secondsToUpdate: number }) {
    const hasActiveIncidents = incidents.some(i => i.status !== 'resolved' && i.status !== 'completed' && i.status !== 'scheduled');
    const hasDown = groups.some(g => g.monitors.some(m => m.status === 'down'));
    const hasDegraded = groups.some(g => g.monitors.some(m => m.status === 'degraded'));

    let statusConfig;

    if (hasActiveIncidents || hasDown) {
        statusConfig = {
            icon: XCircle,
            color: "bg-destructive/15 border-destructive/50 text-destructive",
            message: "System Outage",
            border: "border-destructive/50"
        };
    } else if (hasDegraded) {
        statusConfig = {
            icon: AlertTriangle,
            color: "bg-yellow-500/15 border-yellow-500/50 text-yellow-600 dark:text-yellow-500",
            message: "Partially Degraded Service",
            border: "border-yellow-500/50"
        };
    } else {
        // Operational default
        statusConfig = {
            icon: CheckCircle2,
            color: "bg-emerald-500/15 border-emerald-500/20 text-emerald-600 dark:text-emerald-500",
            message: "All Systems Operational",
            border: "border-emerald-500/20"
        };
    }

    const Icon = statusConfig.icon;

    return (
        <div className="space-y-8 mb-12">
            <div className="flex items-center gap-3 justify-center pt-8">
                <Activity className="w-6 h-6 text-primary" />
                <h1 className="text-2xl font-bold text-foreground">{title}</h1>
            </div>

            <div className={cn(
                "w-full rounded-xl py-4 px-6 flex items-center justify-between border",
                statusConfig.color,
                statusConfig.border
            )}>
                <div className="flex items-center gap-3">
                    <Icon className="w-6 h-6" />
                    <span className="text-xl font-semibold tracking-tight">
                        {statusConfig.message}
                    </span>
                </div>
                <div className="hidden sm:block text-sm opacity-80 font-medium tabular-nums">
                    Refreshing in {secondsToUpdate}s
                </div>
            </div>
        </div>
    )
}

function PublicMonitor({ monitor }: { monitor: Monitor }) {
    const statusColor =
        monitor.status === 'up' ? 'bg-emerald-500' :
            monitor.status === 'degraded' ? 'bg-yellow-500' : 'bg-destructive';

    const statusText =
        monitor.status === 'up' ? 'Operational' :
            monitor.status === 'degraded' ? 'Degraded' : 'Down';

    const statusTextColor =
        monitor.status === 'up' ? 'text-emerald-600 dark:text-emerald-500' :
            monitor.status === 'degraded' ? 'text-yellow-600 dark:text-yellow-500' : 'text-destructive';

    return (
        <div className="group relative flex flex-col sm:flex-row items-center justify-between p-4 rounded-xl border border-border bg-card hover:bg-accent/50 transition-all duration-300 gap-4 overflow-hidden">
            {/* Hover Glow - Simplified to border accent or subtle bg */}

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
                    {monitor.status !== 'up' && (
                        <span className={`absolute inline-flex h-full w-full rounded-full ${statusColor} opacity-75 animate-ping`} />
                    )}
                    <span className={`relative inline-flex rounded-full h-2.5 w-2.5 ${statusColor}`} />
                </div>
            </div>
        </div>
    )
}

function IncidentItem({ incident }: { incident: Incident }) {
    const isMaintenance = incident.type === 'maintenance';

    return (
        <div className="relative pl-8 pb-8 last:pb-0">
            {/* Timeline Line */}
            <div className="absolute left-[11px] top-2 bottom-0 w-px bg-border last:hidden" />

            {/* Timeline Dot */}
            <div className={cn(
                "absolute left-0 top-1.5 w-6 h-6 rounded-full border-4 border-background flex items-center justify-center z-10",
                isMaintenance ? "bg-blue-500" : "bg-destructive"
            )}>
                {isMaintenance ? (
                    <RefreshCw className="w-3 h-3 text-white" />
                ) : (
                    <AlertTriangle className="w-3 h-3 text-white" />
                )}
            </div>

            <div className="bg-card border border-border rounded-lg p-5">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 mb-3">
                    <h3 className="text-lg font-semibold text-foreground">{incident.title}</h3>
                    <Badge variant={isMaintenance ? 'secondary' : 'destructive'}
                        className="w-fit uppercase text-[10px] tracking-wider font-bold">
                        {incident.status.replace('_', ' ')}
                    </Badge>
                </div>

                <div className="prose prose-sm max-w-none text-muted-foreground mb-4">
                    {incident.description}
                </div>

                <div className="text-xs text-muted-foreground font-mono">
                    Updated {new Date(incident.startTime).toLocaleString(undefined, {
                        dateStyle: 'medium',
                        timeStyle: 'short'
                    })}
                </div>
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

    const { groups, incidents, title } = data;

    return (
        <div className="min-h-screen bg-background text-foreground font-sans">
            <main className="relative max-w-4xl mx-auto px-6 pb-20">
                <StatusHeader groups={groups} incidents={incidents} title={title} secondsToUpdate={secondsToUpdate} />

                {/* Incidents Section */}
                {incidents && incidents.length > 0 && (
                    <div className="mb-16 animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards">
                        <div className="flex items-center gap-3 mb-6">
                            <AlertTriangle className="w-5 h-5 text-muted-foreground" />
                            <h2 className="text-xl font-semibold text-foreground">Active Incidents</h2>
                        </div>
                        <div className="pl-2">
                            {incidents.map(i => <IncidentItem key={i.id} incident={i} />)}
                        </div>
                    </div>
                )}

                {/* Groups & Monitors */}
                <div className="space-y-10">
                    {groups && groups.map((group, idx) => (
                        <div key={group.id}
                            className="space-y-4 animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards"
                            style={{ animationDelay: `${idx * 100}ms` }}
                        >
                            <div className="flex items-center justify-between px-2">
                                <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">{group.name}</h3>
                            </div>
                            <div className="space-y-3">
                                {group.monitors.map(m => <PublicMonitor key={m.id} monitor={m} />)}
                            </div>
                        </div>
                    ))}
                </div>
            </main>

            <footer className="relative border-t border-border mt-20 py-10 text-center">
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
