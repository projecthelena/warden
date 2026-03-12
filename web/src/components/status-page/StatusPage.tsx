import { Badge } from "@/components/ui/badge";
import { Activity, AlertTriangle, ArrowDownCircle, CheckCircle2, ChevronDown, ChevronUp, Minus, RefreshCw, Rss, Wrench, XCircle } from "lucide-react";
import { useEffect, useMemo, useState, useCallback } from "react";
import { useParams } from "react-router-dom";
import { useMonitorStore, Group, Incident, Monitor, StatusPageConfig } from "@/lib/store";
import { cn, formatDate, hexToHSL, sanitizeImageUrl } from "@/lib/utils";
import { UptimeBar } from "./UptimeBar";
import { PastIncidentsSection } from "./PastIncidentsSection";
import { IncidentTimeline } from "@/components/incidents/IncidentTimeline";

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
        banner: "bg-gradient-to-r from-emerald-500/10 via-emerald-500/5 to-transparent border-emerald-500/30",
        icon: "text-emerald-500",
        iconBg: "bg-emerald-500/10",
        dot: "bg-emerald-500",
    },
    yellow: {
        banner: "bg-gradient-to-r from-yellow-500/10 via-yellow-500/5 to-transparent border-yellow-500/30",
        icon: "text-yellow-500",
        iconBg: "bg-yellow-500/10",
        dot: "bg-yellow-500",
    },
    red: {
        banner: "bg-gradient-to-r from-red-500/10 via-red-500/5 to-transparent border-red-500/30",
        icon: "text-red-500",
        iconBg: "bg-red-500/10",
        dot: "bg-red-500",
    },
    blue: {
        banner: "bg-gradient-to-r from-blue-500/10 via-blue-500/5 to-transparent border-blue-500/30",
        icon: "text-blue-500",
        iconBg: "bg-blue-500/10",
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
                "relative flex items-center gap-4 px-5 py-4 rounded-2xl border transition-all duration-700 overflow-hidden",
                colors.banner
            )}
        >
            {/* Decorative blur orb */}
            <div className={cn("absolute -right-8 -top-8 w-32 h-32 rounded-full blur-3xl opacity-20", colors.dot)} />

            <div className={cn("relative flex items-center justify-center w-11 h-11 rounded-xl shrink-0", colors.iconBg)}>
                <Icon className={cn("h-5 w-5", colors.icon)} />
            </div>
            <div className="relative flex-1 min-w-0">
                <p className={cn("text-base sm:text-lg font-semibold", colors.icon)}>{status.label}</p>
                <p className="text-xs text-muted-foreground">{status.description}</p>
            </div>
            <div className="relative flex items-center gap-1.5 text-[11px] text-muted-foreground/60 tabular-nums whitespace-nowrap">
                <RefreshCw className="w-3 h-3" />
                {secondsToUpdate}s
            </div>
        </div>
    );
}

function MaintenanceCard({ incident }: { incident: Incident }) {
    const start = new Date(incident.startTime);
    const end = incident.endTime ? new Date(incident.endTime) : null;
    const now = new Date();
    const isOngoing = now >= start && (!end || now < end);

    return (
        <div className="flex items-center justify-between py-3 px-4 rounded-xl border border-blue-500/20 bg-blue-500/5 border-l-2 border-l-blue-500 gap-4">
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
    const [expanded, setExpanded] = useState(false);
    const title = incident.title.replace("Service Disruption: ", "");
    const isDown = incident.severity === "critical";
    const colorClass = isDown ? "text-red-500" : "text-yellow-500";
    const bgBadge = isDown
        ? "bg-red-500/10 text-red-500"
        : "bg-yellow-500/10 text-yellow-500";
    const borderClass = isDown
        ? "border-red-500/20 bg-red-500/5 border-l-2 border-l-red-500"
        : "border-yellow-500/20 bg-yellow-500/5 border-l-2 border-l-yellow-500";

    const start = new Date(incident.startTime).getTime();
    const now = new Date().getTime();
    const diffMins = Math.floor((now - start) / 60000);
    let durationStr = diffMins < 1 ? "Just now" : `${diffMins}m`;
    if (diffMins >= 60) {
        const h = Math.floor(diffMins / 60);
        const m = diffMins % 60;
        durationStr = `${h}h ${m}m`;
    }

    const hasUpdates = incident.updates && incident.updates.length > 0;

    return (
        <div className={cn("rounded-xl border overflow-hidden", borderClass)}>
            <div
                className={cn(
                    "flex items-center justify-between py-3 px-4 gap-4",
                    hasUpdates && "cursor-pointer hover:bg-accent/20 transition-colors"
                )}
                onClick={() => hasUpdates && setExpanded(!expanded)}
            >
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
                <div className="flex items-center gap-2 shrink-0">
                    <span className={cn("text-xs font-medium whitespace-nowrap hidden sm:block", colorClass)}>
                        {durationStr}
                    </span>
                    {hasUpdates && (
                        expanded ? (
                            <ChevronUp className="w-4 h-4 text-muted-foreground" />
                        ) : (
                            <ChevronDown className="w-4 h-4 text-muted-foreground" />
                        )
                    )}
                </div>
            </div>
            {expanded && hasUpdates && (
                <div className="px-4 pb-4 pt-2 border-t border-border/30 bg-background/50">
                    <IncidentTimeline updates={incident.updates!} readonly />
                </div>
            )}
        </div>
    );
}

function MonitorRow({
    monitor,
    isMaintenance,
    showUptimeBars = true,
    showUptimePercentage = true,
}: {
    monitor: StatusMonitor;
    isMaintenance?: boolean;
    showUptimeBars?: boolean;
    showUptimePercentage?: boolean;
}) {
    let statusColor = "text-emerald-500";
    let statusLabel = "Operational";
    let StatusIcon = CheckCircle2;
    if (isMaintenance) {
        statusColor = "text-blue-500";
        statusLabel = "Maintenance";
        StatusIcon = Wrench;
    } else if (monitor.status === "degraded") {
        statusColor = "text-yellow-500";
        statusLabel = "Degraded";
        StatusIcon = AlertTriangle;
    } else if (monitor.status === "down") {
        statusColor = "text-red-500";
        statusLabel = "Down";
        StatusIcon = XCircle;
    } else if (monitor.status === "paused") {
        statusColor = "text-muted-foreground/50";
        statusLabel = "Paused";
        StatusIcon = Minus;
    }

    const uptimeDays = monitor.uptimeDays || [];
    const overallUptime = monitor.overallUptime ?? 100;

    return (
        <div className="group px-4 py-3 border-b border-border/40 last:border-b-0 transition-colors hover:bg-accent/30">
            {/* Top row: status icon + name + status label */}
            <div className="flex items-center justify-between gap-3 mb-1">
                <div className="flex items-center gap-2 min-w-0">
                    <div className="relative flex items-center justify-center shrink-0" role="img" aria-label={statusLabel}>
                        {monitor.status === "down" && !isMaintenance && (
                            <span
                                className="absolute inline-flex h-full w-full rounded-full opacity-75 animate-ping bg-red-500"
                            />
                        )}
                        <StatusIcon className={cn("relative w-3 h-3", statusColor)} />
                    </div>
                    <span className="font-medium text-sm text-foreground truncate" title={monitor.name}>{monitor.name}</span>
                </div>
                <span className="text-xs text-muted-foreground hidden sm:inline shrink-0">{statusLabel}</span>
            </div>

            {/* Uptime bar (full width, below name) */}
            {showUptimeBars && uptimeDays.length > 0 && (
                <UptimeBar days={uptimeDays} overallUptime={overallUptime} showPercentage={showUptimePercentage} />
            )}
        </div>
    );
}

function GroupSection({
    group,
    incidents,
    index,
    showUptimeBars = true,
    showUptimePercentage = true,
}: {
    group: StatusGroup;
    incidents: Incident[];
    index: number;
    showUptimeBars?: boolean;
    showUptimePercentage?: boolean;
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
            className="animate-in slide-in-from-bottom-3 duration-500 fade-in fill-mode-backwards"
            style={{ animationDelay: `${index * 100}ms` }}
        >
            <div className="mb-2 px-1">
                <h3 className="text-sm font-semibold text-foreground">
                    {group.name}
                </h3>
            </div>
            <div className="rounded-2xl border border-border bg-card shadow-sm overflow-hidden">
                {group.monitors.map((m) => (
                    <MonitorRow
                        key={m.id}
                        monitor={m}
                        isMaintenance={isGroupMaintenance}
                        showUptimeBars={showUptimeBars}
                        showUptimePercentage={showUptimePercentage}
                    />
                ))}
                {group.monitors.length === 0 && (
                    <div className="px-5 py-6 text-center text-sm text-muted-foreground">
                        No monitors configured
                    </div>
                )}
            </div>
        </div>
    );
}

function StatusSkeleton() {
    return (
        <div className="min-h-screen bg-background flex flex-col items-center pt-16 sm:pt-20 px-4">
            <div className="w-14 h-14 rounded-full bg-muted animate-pulse mb-5" />
            <div className="h-7 w-48 bg-muted rounded animate-pulse mb-3" />
            <div className="h-4 w-32 bg-muted/50 rounded animate-pulse mb-14" />
            <div className="w-full max-w-3xl space-y-4">
                <div className="h-[72px] w-full bg-muted/50 rounded-2xl animate-pulse" />
                <div className="h-52 w-full bg-muted/30 rounded-2xl animate-pulse" style={{ animationDelay: "100ms" }} />
                <div className="h-52 w-full bg-muted/30 rounded-2xl animate-pulse" style={{ animationDelay: "200ms" }} />
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
        pastIncidents?: Incident[];
        config?: StatusPageConfig;
    } | null>(null);
    const [secondsToUpdate, setSecondsToUpdate] = useState(60);

    // Apply theme based on config
    const applyTheme = useCallback((config?: StatusPageConfig) => {
        const theme = config?.theme || 'system';
        const root = document.documentElement;

        // Remove existing theme classes
        root.classList.remove('light', 'dark');

        if (theme === 'system') {
            // Use system preference
            const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            root.classList.add(prefersDark ? 'dark' : 'light');
        } else {
            root.classList.add(theme);
        }
    }, []);

    // Apply accent color via CSS variable
    const applyAccentColor = useCallback((config?: StatusPageConfig) => {
        const root = document.documentElement;
        const accentColor = config?.accentColor;

        if (accentColor) {
            const hsl = hexToHSL(accentColor);
            if (hsl) {
                root.style.setProperty('--primary', `${hsl.h} ${hsl.s}% ${hsl.l}%`);
            }
        } else {
            // Reset to default
            root.style.removeProperty('--primary');
        }
    }, []);

    // Apply custom favicon
    const applyFavicon = useCallback((config?: StatusPageConfig, pageTitle?: string) => {
        const faviconUrl = config?.faviconUrl;

        // Update page title
        if (pageTitle) {
            document.title = `${pageTitle} - Status`;
        }

        // Find or create favicon link element
        let faviconLink = document.querySelector<HTMLLinkElement>('link[rel="icon"]');

        if (faviconUrl) {
            if (!faviconLink) {
                faviconLink = document.createElement('link');
                faviconLink.rel = 'icon';
                document.head.appendChild(faviconLink);
            }
            faviconLink.href = sanitizeImageUrl(faviconUrl);
        } else if (faviconLink) {
            // Reset to default favicon
            faviconLink.href = '/favicon.ico';
        }
    }, []);

    useEffect(() => {
        let isMounted = true;

        const load = async (isBackground = false) => {
            if (!isBackground) setLoading(true);
            const result = await fetchPublicStatusBySlug(slug || "all");

            if (isMounted) {
                if (result) {
                    setData(result);
                    setError(null);
                    applyTheme(result.config);
                    applyAccentColor(result.config);
                    applyFavicon(result.config, result.title);
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
            // Cleanup: restore user's theme preference (not hardcoded dark)
            const storedTheme = (localStorage.getItem('warden-theme') as 'dark' | 'light' | 'system') || 'dark';
            const root = document.documentElement;
            root.classList.remove('light', 'dark');
            root.style.removeProperty('--primary');
            if (storedTheme === 'system') {
                const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
                root.classList.add(prefersDark ? 'dark' : 'light');
            } else {
                root.classList.add(storedTheme);
            }
            // Reset favicon and title
            const faviconLink = document.querySelector<HTMLLinkElement>('link[rel="icon"]');
            if (faviconLink) faviconLink.href = '/favicon.ico';
            document.title = 'Warden';
        };
    }, [slug, fetchPublicStatusBySlug, applyTheme, applyAccentColor, applyFavicon]);

    // Listen for system theme changes when using 'system' theme
    useEffect(() => {
        const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        const handleChange = () => {
            if (data?.config?.theme === 'system') {
                applyTheme(data.config);
            }
        };
        mediaQuery.addEventListener('change', handleChange);
        return () => mediaQuery.removeEventListener('change', handleChange);
    }, [data?.config, applyTheme]);

    // Computed state
    const computed = useMemo(() => {
        if (!data) return null;
        const { groups, incidents = [], pastIncidents = [], config } = data;
        const { maintenanceIncidents, maintenanceGroupIds } = getMaintenanceState(incidents);
        const status = getOverallStatus(groups, incidents, maintenanceGroupIds);

        const incidentItems = (incidents || []).filter((i) => {
            if (i.type !== "incident" || i.status === "resolved") return false;
            if (i.affectedGroups && i.affectedGroups.length > 0) {
                return !i.affectedGroups.some((gId) => maintenanceGroupIds.has(gId));
            }
            return true;
        });

        return { groups, incidents, pastIncidents, maintenanceIncidents, incidentItems, status, config };
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

    const { groups, incidents, pastIncidents, maintenanceIncidents, incidentItems, status, config } = computed;
    const showUptimeBars = config?.showUptimeBars ?? true;
    const showUptimePercentage = config?.showUptimePercentage ?? true;
    const showIncidentHistory = config?.showIncidentHistory ?? true;

    return (
        <div className="min-h-screen bg-background text-foreground font-sans flex flex-col">
            <main className="max-w-3xl mx-auto px-4 sm:px-6 pb-16 w-full flex-1">
                {/* Header */}
                <div className="flex flex-col items-center pt-16 sm:pt-20 pb-10 sm:pb-14">
                    <div className="relative mb-4">
                        {/* Glow halo behind logo */}
                        <div className="absolute inset-0 bg-primary/15 blur-2xl rounded-full scale-150" />
                        {config?.logoUrl ? (
                            <img
                                src={sanitizeImageUrl(config.logoUrl)}
                                alt="Logo"
                                className="relative w-10 h-10 object-contain"
                                onError={(e) => {
                                    e.currentTarget.style.display = 'none';
                                    e.currentTarget.parentElement?.querySelector('.fallback-icon')?.classList.remove('hidden');
                                }}
                            />
                        ) : null}
                        <Activity className={cn("relative w-8 h-8 text-primary fallback-icon", config?.logoUrl && "hidden")} />
                    </div>
                    <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-foreground">{data.title}</h1>
                    {config?.description && (
                        <p className="text-sm text-muted-foreground mt-2 text-center max-w-md leading-relaxed">
                            {config.description}
                        </p>
                    )}
                </div>

                {/* Status Banner */}
                <div className="mb-6 animate-in fade-in duration-500">
                    <StatusBanner status={status} secondsToUpdate={secondsToUpdate} />
                </div>

                {/* Alerts: Maintenance & Incidents */}
                {(maintenanceIncidents.length > 0 || incidentItems.length > 0) && (
                    <div className="mb-8 space-y-6 animate-in slide-in-from-bottom-3 duration-500 fade-in fill-mode-backwards">
                        {maintenanceIncidents.length > 0 && (
                            <div>
                                <h2 className="text-sm font-semibold text-foreground mb-2 px-1 flex items-center gap-2">
                                    <span className="inline-block w-2 h-2 rounded-full bg-blue-500" />
                                    Scheduled Maintenance
                                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0 h-4">
                                        {maintenanceIncidents.length}
                                    </Badge>
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
                                <h2 className="text-sm font-semibold text-foreground mb-2 px-1 flex items-center gap-2">
                                    <span className="inline-block w-2 h-2 rounded-full bg-red-500" />
                                    Active Incidents
                                    <Badge variant="secondary" className="text-[10px] px-1.5 py-0 h-4">
                                        {incidentItems.length}
                                    </Badge>
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
                <div className="space-y-6">
                    {groups.map((group, idx) => (
                        <GroupSection
                            key={group.id}
                            group={group}
                            incidents={incidents}
                            index={idx}
                            showUptimeBars={showUptimeBars}
                            showUptimePercentage={showUptimePercentage}
                        />
                    ))}
                </div>

                {/* Past Incidents */}
                {showIncidentHistory && pastIncidents && pastIncidents.length > 0 && (
                    <div className="mt-10">
                        <PastIncidentsSection incidents={pastIncidents} />
                    </div>
                )}
            </main>

            {/* Footer */}
            <footer className="border-t border-border mt-auto py-8">
                <div className="max-w-3xl mx-auto px-4 sm:px-6 flex items-center justify-between text-muted-foreground/60 text-xs">
                    <div className="flex items-center gap-1.5">
                        <span>Powered by</span>
                        <a
                            href="https://projecthelena.com/"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="font-semibold text-foreground/60 hover:text-foreground hover:underline underline-offset-4 transition-all"
                        >
                            Warden
                        </a>
                    </div>
                    <a
                        href={`/api/s/${slug}/rss`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1 hover:text-foreground transition-colors"
                        title="Subscribe via RSS"
                    >
                        <Rss className="w-3 h-3" />
                        <span>RSS</span>
                    </a>
                </div>
            </footer>
        </div>
    );
}
