/* Hallmark · genre: modern-minimal · macrostructure: Workbench · theme: Warden locked system · enrichment: none · nav: in-page tabs · footer: Ft2
 * audience: status-page visitors · use: understand impact and find a service · tone: technical, austere, utilitarian
 * pre-emit critique: P5 H5 E5 S5 R5 V4
 */
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Activity, AlertTriangle, ArrowDownCircle, CheckCircle2, ChevronDown, ChevronUp, Lock, Minus, RefreshCw, Rss, ShieldX, Wrench, XCircle } from "lucide-react";
import { useEffect, useMemo, useState, useCallback } from "react";
import { useParams } from "react-router-dom";
import { useMonitorStore, Group, Incident, Monitor, StatusPageConfig } from "@/lib/store";
import { cn, formatDate, hexToHSL, sanitizeImageUrl } from "@/lib/utils";
import { UptimeBar } from "./UptimeBar";
import { PastIncidentsSection } from "./PastIncidentsSection";
import { getOverallStatus } from "./statusPageStatus";
import { IncidentTimeline } from "@/components/incidents/IncidentTimeline";
import { type UptimeSummary } from "@/lib/uptime";
import { getMaintenanceState, isMaintenanceActive } from "@/lib/maintenance";
import { MarkdownContent } from "@/components/ui/markdown";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { buildStatusMonitorQuery } from "./statusPageMonitors";

// ---------- Types ----------

interface DayData {
    date: string;
    uptimePercent: number;
    totalChecks: number;
    upChecks: number;
}

interface StatusMonitor extends Monitor {
    uptimeDays?: DayData[];
    uptime?: UptimeSummary;
    overallUptime?: number;
    checkIntervalSeconds?: number;
}

interface StatusGroup extends Omit<Group, "monitors"> {
    monitors: StatusMonitor[];
    status?: Monitor["status"];
    monitorCount?: number;
    counts?: Record<string, number>;
}

interface GroupMonitorCounts {
    all: number;
    operational: number;
    issues: number;
    paused: number;
}

interface GroupPagination {
    page: number;
    pageSize: number;
    total: number;
    totalPages: number;
}

// ---------- Helpers ----------

const statusColorMap = {
    green: {
        banner: "bg-emerald-500/5 border-emerald-500/30",
        icon: "text-emerald-500",
        iconBg: "bg-emerald-500/10",
    },
    yellow: {
        banner: "bg-yellow-500/5 border-yellow-500/30",
        icon: "text-yellow-500",
        iconBg: "bg-yellow-500/10",
    },
    red: {
        banner: "bg-red-500/5 border-red-500/30",
        icon: "text-red-500",
        iconBg: "bg-red-500/10",
    },
    blue: {
        banner: "bg-blue-500/5 border-blue-500/30",
        icon: "text-blue-500",
        iconBg: "bg-blue-500/10",
    },
    gray: {
        banner: "bg-slate-500/5 border-slate-500/30",
        icon: "text-slate-400",
        iconBg: "bg-slate-500/10",
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
                "relative flex items-center gap-4 px-5 py-4 rounded-2xl border overflow-hidden",
                colors.banner
            )}
        >
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

function MaintenanceCard({ incident, timezone }: { incident: Incident; timezone: string }) {
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
                                className="bg-blue-500/10 text-blue-500 border-0 rounded-sm px-1.5 py-0 text-[10px] font-bold uppercase tracking-wider h-5 shrink-0"
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
                    {incident.description && <MarkdownContent className="mt-0.5 line-clamp-2 text-xs text-muted-foreground">{incident.description}</MarkdownContent>}
                </div>
            </div>
            <div className="text-[11px] text-muted-foreground tabular-nums font-mono whitespace-nowrap hidden sm:block">
                {formatDate(incident.startTime, timezone)}
                {incident.endTime && (
                    <> &mdash; {formatDate(incident.endTime, timezone)}</>
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
    uptimeDaysRange,
}: {
    monitor: StatusMonitor;
    isMaintenance?: boolean;
    showUptimeBars?: boolean;
    showUptimePercentage?: boolean;
    uptimeDaysRange: number;
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
    const uptime = monitor.uptime ?? {
        percent: monitor.overallUptime ?? 100,
        totalChecks: uptimeDays.reduce((sum, day) => sum + day.totalChecks, 0),
        downChecks: 0,
        downtimeSeconds: 0,
    };

    return (
        <div className="group px-4 py-3 border-b border-border/40 last:border-b-0 transition-colors hover:bg-accent/30">
            {/* Top row: status icon + name + status label */}
            <div className="flex items-center justify-between gap-3 mb-1">
                <div className="flex items-center gap-2 min-w-0">
                    <div className="relative flex items-center justify-center shrink-0" role="img" aria-label={statusLabel}>
                        <StatusIcon className={cn("relative w-3 h-3", statusColor)} />
                    </div>
                    <span className="font-medium text-sm text-foreground truncate" title={monitor.name}>{monitor.name}</span>
                </div>
                <span className="text-xs text-muted-foreground hidden sm:inline shrink-0">{statusLabel}</span>
            </div>

            {/* Uptime bar (full width, below name) */}
            {showUptimeBars && uptimeDays.length > 0 && (
                <UptimeBar
                    days={uptimeDays}
                    summary={uptime}
                    rangeDays={uptimeDaysRange}
                    intervalSeconds={monitor.checkIntervalSeconds ?? monitor.interval ?? 60}
                    showPercentage={showUptimePercentage}
                />
            )}
        </div>
    );
}

function GroupSection({
    group,
    incidents,
    slug,
    showUptimeBars = true,
    showUptimePercentage = true,
    uptimeDaysRange,
}: {
    group: StatusGroup;
    incidents: Incident[];
    slug: string;
    showUptimeBars?: boolean;
    showUptimePercentage?: boolean;
    uptimeDaysRange: number;
}) {
    const [expanded, setExpanded] = useState(false);
    const [monitors, setMonitors] = useState<StatusMonitor[]>([]);
    const [loading, setLoading] = useState(false);
    const [loadError, setLoadError] = useState<string | null>(null);
    const [searchInput, setSearchInput] = useState("");
    const [search, setSearch] = useState("");
    const [statusFilter, setStatusFilter] = useState("all");
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(25);
    const [retryKey, setRetryKey] = useState(0);
    const total = group.monitorCount ?? 0;
    const [counts, setCounts] = useState<GroupMonitorCounts>({
        all: total,
        operational: group.counts?.up ?? 0,
        issues: (group.counts?.down ?? 0) + (group.counts?.degraded ?? 0),
        paused: group.counts?.paused ?? 0,
    });
    const [pagination, setPagination] = useState<GroupPagination>({ page: 1, pageSize: 25, total, totalPages: total ? Math.ceil(total / 25) : 0 });
    const now = new Date();
    const isGroupMaintenance =
        incidents &&
        incidents.some(
            (i) =>
                i.affectedGroups?.includes(group.id) &&
                isMaintenanceActive(i, now)
        );

    useEffect(() => {
        const timer = window.setTimeout(() => {
            const nextSearch = searchInput.trim();
            if (nextSearch !== search) {
                setPage(1);
                setSearch(nextSearch);
            }
        }, 250);
        return () => window.clearTimeout(timer);
    }, [search, searchInput]);

    useEffect(() => {
        if (!expanded) return;
        const controller = new AbortController();
        const loadMonitors = async () => {
            setLoading(true);
            setLoadError(null);
            try {
                const query = buildStatusMonitorQuery({
                    group: group.id,
                    page,
                    pageSize,
                    status: statusFilter,
                    search,
                });
                const response = await fetch(`/api/s/${encodeURIComponent(slug)}?${query}`, {
                    credentials: "include",
                    signal: controller.signal,
                });
                if (!response.ok) throw new Error("Unable to load services");
                const payload = await response.json();
                const nextPagination = payload.pagination as GroupPagination;
                if (nextPagination.totalPages > 0 && page > nextPagination.totalPages) {
                    setPage(nextPagination.totalPages);
                    return;
                }
                setMonitors((payload.groups?.[0]?.monitors || []) as StatusMonitor[]);
                setCounts(payload.counts as GroupMonitorCounts);
                setPagination(nextPagination);
            } catch (error) {
                if ((error as Error).name !== "AbortError") {
                    setLoadError("Services could not be loaded. Try again.");
                }
            } finally {
                if (!controller.signal.aborted) setLoading(false);
            }
        };
        void loadMonitors();
        return () => controller.abort();
    }, [expanded, group.id, page, pageSize, retryKey, search, slug, statusFilter]);

    const toggleExpanded = () => {
        const next = !expanded;
        setExpanded(next);
    };

    const showControls = total > 10 || searchInput !== "" || statusFilter !== "all";
    const firstVisible = pagination.total === 0 ? 0 : (pagination.page - 1) * pagination.pageSize + 1;
    const lastVisible = Math.min(pagination.page * pagination.pageSize, pagination.total);

    const statusLabel = group.status === "down" ? "Unavailable" : group.status === "degraded" ? "Degraded" : group.status === "paused" ? "Paused" : "Operational";
    const statusClass = group.status === "down" ? "text-red-500" : group.status === "degraded" ? "text-yellow-500" : group.status === "paused" ? "text-muted-foreground" : "text-emerald-500";

    return (
        <div className="overflow-hidden rounded-xl border border-border bg-card [content-visibility:auto] [contain-intrinsic-size:72px]">
            <button
                type="button"
                onClick={toggleExpanded}
                aria-expanded={expanded}
                aria-controls={`status-group-${group.id}`}
                className="flex min-h-14 w-full items-center justify-between gap-4 px-4 py-3 text-left hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-inset"
            >
                <div className="min-w-0">
                    <h3 className="truncate text-sm font-semibold text-foreground">{group.name}</h3>
                    <p className="mt-0.5 text-xs text-muted-foreground tabular-nums">{total} {total === 1 ? "service" : "services"}</p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                    <span className={cn("text-xs font-medium", statusClass)}>{statusLabel}</span>
                    <ChevronDown className={cn("h-4 w-4 text-muted-foreground transition-transform", expanded && "rotate-180")} />
                </div>
            </button>
            {expanded && (
                <div id={`status-group-${group.id}`} className="border-t border-border/60">
                {showControls && (
                    <div className="space-y-3 border-b border-border/60 bg-muted/10 p-3 sm:p-4">
                        <div className="flex flex-col gap-2 sm:flex-row">
                            <Label htmlFor={`monitor-search-${group.id}`} className="sr-only">Search services in {group.name}</Label>
                            <Input
                                id={`monitor-search-${group.id}`}
                                value={searchInput}
                                onChange={(event) => setSearchInput(event.target.value)}
                                placeholder="Search services"
                                className="h-9 flex-1"
                            />
                            <Select value={String(pageSize)} onValueChange={(value) => { setPageSize(Number(value)); setPage(1); }}>
                                <SelectTrigger aria-label="Services per page" className="h-9 w-full sm:w-[112px]">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="25">25 / page</SelectItem>
                                    <SelectItem value="50">50 / page</SelectItem>
                                    <SelectItem value="100">100 / page</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="flex flex-wrap gap-1.5" aria-label="Filter services by status">
                            {([
                                ["all", "All", counts.all],
                                ["operational", "Operational", counts.operational],
                                ["issues", "Issues", counts.issues],
                                ["paused", "Paused", counts.paused],
                            ] as const).map(([value, label, count]) => (
                                <Button
                                    key={value}
                                    type="button"
                                    size="sm"
                                    variant={statusFilter === value ? "secondary" : "ghost"}
                                    aria-pressed={statusFilter === value}
                                    className="h-8 whitespace-nowrap px-2.5 text-xs"
                                    onClick={() => { setStatusFilter(value); setPage(1); }}
                                >
                                    {label} <span className="ml-1 tabular-nums text-muted-foreground">{count}</span>
                                </Button>
                            ))}
                        </div>
                    </div>
                )}
                {monitors.map((m) => (
                    <MonitorRow
                        key={m.id}
                        monitor={m}
                        isMaintenance={isGroupMaintenance}
                        showUptimeBars={showUptimeBars}
                        showUptimePercentage={showUptimePercentage}
                        uptimeDaysRange={uptimeDaysRange}
                    />
                ))}
                {loading && <div role="status" aria-live="polite" className="px-5 py-5 text-sm text-muted-foreground">Loading services…</div>}
                {loadError && (
                    <div className="flex items-center justify-between gap-3 px-5 py-4 text-sm text-destructive">
                        <span>{loadError}</span>
                        <Button variant="outline" size="sm" onClick={() => setRetryKey((value) => value + 1)}>Retry</Button>
                    </div>
                )}
                {!loading && !loadError && monitors.length === 0 && (
                    <div className="px-5 py-8 text-center text-sm text-muted-foreground">
                        {search || statusFilter !== "all" ? "No services match these filters" : "No services configured"}
                    </div>
                )}
                {!loading && !loadError && pagination.totalPages > 1 && (
                    <div className="flex flex-col items-center justify-between gap-3 border-t border-border/40 px-4 py-3 sm:flex-row">
                        <p className="text-xs tabular-nums text-muted-foreground">{firstVisible}–{lastVisible} of {pagination.total}</p>
                        <div className="flex items-center gap-2">
                            <Button variant="outline" size="sm" disabled={page <= 1} onClick={() => setPage((value) => value - 1)}>Previous</Button>
                            <span className="min-w-20 text-center text-xs tabular-nums text-muted-foreground">Page {pagination.page} of {pagination.totalPages}</span>
                            <Button variant="outline" size="sm" disabled={page >= pagination.totalPages} onClick={() => setPage((value) => value + 1)}>Next</Button>
                        </div>
                    </div>
                )}
                </div>
            )}
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

// ---------- Login for Private Pages ----------

function StatusPageLogin({ slug, onSuccess }: { slug?: string; onSuccess: () => void }) {
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [loginError, setLoginError] = useState("");
    const [isLoading, setIsLoading] = useState(false);

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);
        setLoginError("");
        try {
            const res = await fetch("/api/auth/login", {
                method: "POST",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ username, password }),
                credentials: "include",
            });
            if (res.ok) {
                onSuccess();
            } else {
                const data = await res.json().catch(() => ({ error: "Login failed" }));
                setLoginError(data.error || "Invalid credentials");
            }
        } catch {
            setLoginError("Login failed. Please try again.");
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="min-h-screen bg-background flex items-center justify-center text-foreground">
            <div className="w-full max-w-sm space-y-6 px-4">
                <div className="text-center space-y-2">
                    <Lock className="w-12 h-12 text-muted-foreground mx-auto" />
                    <h1 className="text-2xl font-bold">Private Status Page</h1>
                    <p className="text-sm text-muted-foreground">
                        Sign in to view {slug ? `"${slug}"` : "this status page"}.
                    </p>
                </div>
                <form onSubmit={handleLogin} className="space-y-4">
                    <div className="space-y-2">
                        <Label htmlFor="sp-username">Username</Label>
                        <Input
                            id="sp-username"
                            value={username}
                            onChange={(e) => setUsername(e.target.value)}
                            required
                            autoFocus
                        />
                    </div>
                    <div className="space-y-2">
                        <Label htmlFor="sp-password">Password</Label>
                        <Input
                            id="sp-password"
                            type="password"
                            value={password}
                            onChange={(e) => setPassword(e.target.value)}
                            required
                        />
                    </div>
                    {loginError && (
                        <p className="text-sm text-destructive">{loginError}</p>
                    )}
                    <Button type="submit" className="w-full" disabled={isLoading}>
                        {isLoading ? "Signing in..." : "Sign In"}
                    </Button>
                </form>
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
    const [retryKey, setRetryKey] = useState(0);
    const [data, setData] = useState<{
        title: string;
        groups: StatusGroup[];
        incidents: Incident[];
        pastIncidents?: Incident[];
        config?: StatusPageConfig;
    } | null>(null);
    const [secondsToUpdate, setSecondsToUpdate] = useState(60);
    const [serviceQuery, setServiceQuery] = useState("");

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
                if (result?._error === "auth_required") {
                    setError("auth_required");
                    setData(null);
                } else if (result?._error === "forbidden") {
                    setError("forbidden");
                    setData(null);
                } else if (result) {
                    setData(result);
                    setError(null);
                    applyTheme(result.config);
                    applyAccentColor(result.config);
                    applyFavicon(result.config, result.title);
                } else {
                    setError("Status page not found.");
                }
                setLoading(false);
                if (result && !result._error) setSecondsToUpdate(60);
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
    }, [slug, fetchPublicStatusBySlug, applyTheme, applyAccentColor, applyFavicon, retryKey]);

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
        const statusGroups = groups.map((group) => group.monitors?.length ? group : {
            ...group,
            monitors: group.monitorCount ? [{ id: `${group.id}-summary`, status: group.status || "up" }] : [],
        });
        const status = getOverallStatus(statusGroups, incidents, maintenanceGroupIds);

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

    if (error === "auth_required") {
        return <StatusPageLogin slug={slug} onSuccess={() => { setError(null); setRetryKey((k) => k + 1); }} />;
    }

    if (error === "forbidden") {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center text-foreground">
                <div className="text-center space-y-4">
                    <ShieldX className="w-16 h-16 text-muted-foreground mx-auto" />
                    <h1 className="text-2xl font-bold">Access Denied</h1>
                    <p className="text-muted-foreground">You do not have permission to view this status page.</p>
                    <div className="flex gap-2 justify-center pt-2">
                        <Button variant="outline" onClick={() => window.history.back()}>Go Back</Button>
                        <Button variant="outline" onClick={() => { window.location.href = "/my-pages"; }}>My Pages</Button>
                    </div>
                </div>
            </div>
        );
    }

    if (error || !data || !computed) {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center text-foreground">
                <div className="text-center space-y-4">
                    <Activity className="w-16 h-16 text-muted-foreground mx-auto" />
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
    const uptimeDaysRange = config?.uptimeDaysRange ?? 90;
    const timezone = config?.timezone || 'UTC';
    const visibleGroups = groups.filter((group) => group.name.toLowerCase().includes(serviceQuery.trim().toLowerCase()));
    const totalMonitors = groups.reduce((sum, group) => sum + (group.monitorCount ?? group.monitors.length), 0);

    return (
        <div className="min-h-screen bg-background text-foreground font-sans flex flex-col">
            <main className="max-w-3xl mx-auto px-4 sm:px-6 pb-16 w-full flex-1">
                {/* Header */}
                {(() => {
                    const content = config?.headerContent || 'logo-title';
                    const alignment = config?.headerAlignment || 'center';
                    const arrangement = config?.headerArrangement || 'inline';
                    const hasLogo = !!config?.logoUrl;
                    const showLogo = content !== 'title-only';
                    const showTitle = content !== 'logo-only';
                    const isInline = content === 'logo-title' && arrangement === 'inline';

                    const alignItems = alignment === 'left' ? 'items-start' : alignment === 'right' ? 'items-end' : 'items-center';
                    const textAlign = alignment === 'left' ? 'text-left' : alignment === 'right' ? 'text-right' : 'text-center';

                    const logoElement = showLogo && (
                        <div>
                            {hasLogo ? (
                                <img
                                    src={sanitizeImageUrl(config.logoUrl)}
                                    alt="Logo"
                                    className={cn("relative object-contain", content === 'logo-only' ? "w-16 h-16 sm:w-20 sm:h-20" : "w-10 h-10")}
                                    onError={(e) => {
                                        e.currentTarget.style.display = 'none';
                                        e.currentTarget.parentElement?.querySelector('.fallback-icon')?.classList.remove('hidden');
                                    }}
                                />
                            ) : null}
                            <Activity className={cn("relative w-8 h-8 text-primary fallback-icon", hasLogo && "hidden")} />
                        </div>
                    );

                    const titleElement = showTitle && (
                        <h1 className={cn(
                            "text-2xl sm:text-3xl font-bold tracking-tight text-foreground",
                            textAlign
                        )}>{data.title}</h1>
                    );

                    const descriptionElement = config?.description && showTitle && (
                        <p className={cn(
                            "text-sm text-muted-foreground mt-2 max-w-md leading-relaxed",
                            textAlign
                        )}>
                            {config.description}
                        </p>
                    );

                    if (isInline) {
                        return (
                            <div className={cn("pt-16 sm:pt-20 pb-10 sm:pb-14 flex flex-col", alignItems)}>
                                <div className="flex items-center gap-4">
                                    {logoElement}
                                    <div className="min-w-0">
                                        {titleElement}
                                        {descriptionElement}
                                    </div>
                                </div>
                            </div>
                        );
                    }

                    return (
                        <div className={cn("flex flex-col pt-16 sm:pt-20 pb-10 sm:pb-14", alignItems)}>
                            {logoElement && <div className="mb-4">{logoElement}</div>}
                            {titleElement}
                            {descriptionElement}
                        </div>
                    );
                })()}

                <div className="mb-6">
                    <StatusBanner status={status} secondsToUpdate={secondsToUpdate} />
                </div>

                <Tabs defaultValue="overview" className="w-full">
                    <TabsList className="mb-8 grid h-11 w-full grid-cols-3 sm:inline-grid sm:w-auto">
                        <TabsTrigger value="overview">Overview</TabsTrigger>
                        <TabsTrigger value="services">Services</TabsTrigger>
                        <TabsTrigger value="history">History</TabsTrigger>
                    </TabsList>

                    <TabsContent value="overview" className="mt-0 space-y-8">
                        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                            <div className="rounded-lg border border-border bg-card p-4"><p className="text-xs text-muted-foreground">Services</p><p className="mt-1 font-mono text-2xl font-medium tabular-nums">{totalMonitors}</p></div>
                            <div className="rounded-lg border border-border bg-card p-4"><p className="text-xs text-muted-foreground">Operational</p><p className="mt-1 font-mono text-2xl font-medium text-emerald-500 tabular-nums">{groups.reduce((sum, group) => sum + (group.counts?.up || 0), 0)}</p></div>
                            <div className="rounded-lg border border-border bg-card p-4"><p className="text-xs text-muted-foreground">Degraded</p><p className="mt-1 font-mono text-2xl font-medium text-yellow-500 tabular-nums">{groups.reduce((sum, group) => sum + (group.counts?.degraded || 0), 0)}</p></div>
                            <div className="rounded-lg border border-border bg-card p-4"><p className="text-xs text-muted-foreground">Unavailable</p><p className="mt-1 font-mono text-2xl font-medium text-red-500 tabular-nums">{groups.reduce((sum, group) => sum + (group.counts?.down || 0), 0)}</p></div>
                        </div>

                {(maintenanceIncidents.length > 0 || incidentItems.length > 0) ? (
                    <div className="space-y-6">
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
                                        <MaintenanceCard key={i.id} incident={i} timezone={timezone} />
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
                ) : <p className="text-sm text-muted-foreground">No active incidents or maintenance.</p>}
                    </TabsContent>

                    <TabsContent value="services" className="mt-0">
                        <div className="mb-4 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                            <div><h2 className="text-lg font-semibold">Services &amp; groups</h2><p className="text-sm text-muted-foreground">Expand a group to load its services and uptime history.</p></div>
                            <Input value={serviceQuery} onChange={(event) => setServiceQuery(event.target.value)} placeholder="Search groups" aria-label="Search groups" className="sm:max-w-xs" />
                        </div>
                        <div className="space-y-3">
                    {visibleGroups.map((group) => (
                        <GroupSection
                            key={group.id}
                            group={group}
                            incidents={incidents}
                            slug={slug || "all"}
                            showUptimeBars={showUptimeBars}
                            showUptimePercentage={showUptimePercentage}
                            uptimeDaysRange={uptimeDaysRange}
                        />
                    ))}
                        {visibleGroups.length === 0 && <div className="rounded-lg border border-dashed border-border p-8 text-center text-sm text-muted-foreground">No groups match your search.</div>}
                        </div>
                    </TabsContent>

                    <TabsContent value="history" className="mt-0">
                {showIncidentHistory && pastIncidents && pastIncidents.length > 0 ? (
                        <PastIncidentsSection incidents={pastIncidents} timezone={timezone} />
                ) : <div className="rounded-lg border border-border bg-card p-8 text-center"><h2 className="font-semibold">No recent incidents</h2><p className="mt-1 text-sm text-muted-foreground">Resolved incidents from the last 14 days will appear here.</p></div>}
                    </TabsContent>
                </Tabs>
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
                            className="font-semibold text-foreground/60 hover:text-foreground hover:underline underline-offset-4 transition-colors"
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
