/* Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4
 * genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app
 * contrast: pass (40–41) · slop: pass (42–45) · responsive: pass (34, 49, 50–57)
 */
import { useMemo } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { Activity, Bell, BellOff, Calendar, ExternalLink, Pause, Play, Settings2 } from "lucide-react";
import { useMonitorsQuery } from "@/hooks/useMonitors";
import { useFilteredSystemEvents } from "@/hooks/useSystemEvents";
import { useRole } from "@/hooks/useRole";
import { findMonitorWithGroup, SSLWarning, SystemIncident, useMonitorStore } from "@/lib/store";
import { formatDate } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";
import { StatusBadge } from "@/components/ui/monitor-visuals";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { IncidentCard } from "@/components/IncidentCard";
import { MonitorOverview } from "@/components/monitor/MonitorOverview";
import { MonitorSettings } from "@/components/monitor/MonitorSettings";

const today = () => new Date().toISOString().slice(0, 10);

export function MonitorPage() {
    const { id } = useParams<{ id: string }>();
    const navigate = useNavigate();
    const [searchParams, setSearchParams] = useSearchParams();
    const { groups, user, pauseMonitor, resumeMonitor, setMonitorAlertsMuted } = useMonitorStore();
    const { canEdit } = useRole();
    const { isLoading } = useMonitorsQuery();
    const context = useMemo(() => id ? findMonitorWithGroup(groups, id) : null, [groups, id]);
    const monitor = context?.monitor;
    const group = context?.group;
    const requestedTab = searchParams.get("tab");
    const activeTab = requestedTab === "incidents" || (requestedTab === "settings" && canEdit) ? requestedTab : "overview";
    const date = searchParams.get("date") || today();
    const incidentsQuery = useFilteredSystemEvents({ monitorId: id, date });
    const incidents: SystemIncident[] = [...(incidentsQuery.data?.active ?? []), ...(incidentsQuery.data?.history ?? [])];
    const sslWarnings = incidentsQuery.data?.sslWarnings ?? [];

    const updateQuery = (updates: Record<string, string | null>) => {
        const next = new URLSearchParams(searchParams);
        Object.entries(updates).forEach(([key, value]) => value ? next.set(key, value) : next.delete(key));
        setSearchParams(next, { replace: true });
    };

    if (isLoading && !monitor) return <MonitorPageSkeleton />;
    if (!id || !monitor || !group) return <div className="grid min-h-72 place-items-center rounded-xl border border-dashed border-border text-sm text-muted-foreground">Monitor not found.</div>;
    const paused = monitor.status === "paused";

    return (
        <div className="space-y-6" data-testid="monitor-page">
            <header className="rounded-xl border border-border bg-card p-4 shadow-none sm:p-6">
                <div className="flex flex-col gap-5 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0 space-y-3">
                        <div className="flex flex-wrap items-center gap-3">
                            <h1 className="min-w-0 break-words text-2xl font-semibold tracking-tight sm:text-3xl">{monitor.name}</h1>
                            <StatusBadge status={monitor.status} />
                            {monitor.alertsMuted && <span className="inline-flex items-center gap-1 rounded-md border border-amber-500/30 bg-amber-500/10 px-2 py-1 text-xs text-amber-300"><BellOff className="h-3 w-3" />Alerts muted</span>}
                        </div>
                        <a href={monitor.url} target="_blank" rel="noopener noreferrer" className="inline-flex max-w-full items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                            <span className="truncate">{monitor.url}</span><ExternalLink className="h-3.5 w-3.5 shrink-0" />
                        </a>
                        <dl className="flex flex-wrap gap-x-6 gap-y-2 text-xs text-muted-foreground">
                            <Meta label="Type" value={monitor.type.toUpperCase()} /><Meta label="Group" value={group.name} />
                            <Meta label="Frequency" value={monitor.interval < 60 ? `${monitor.interval}s` : `${monitor.interval / 60}m`} />
                            <Meta label="Last check" value={monitor.lastCheck && monitor.lastCheck !== "Never" ? formatDate(monitor.lastCheck, user?.timezone) : "Never"} />
                        </dl>
                    </div>
                    {canEdit && <div className="flex flex-wrap gap-2 lg:justify-end">
                        <Button variant="outline" className="whitespace-nowrap" data-testid="monitor-mute-alerts-btn" onClick={() => setMonitorAlertsMuted(monitor.id, !monitor.alertsMuted)}>{monitor.alertsMuted ? <Bell className="mr-2 h-4 w-4" /> : <BellOff className="mr-2 h-4 w-4" />}{monitor.alertsMuted ? "Unmute Alerts" : "Mute Alerts"}</Button>
                        <Button variant={paused ? "default" : "secondary"} className="whitespace-nowrap" onClick={() => paused ? resumeMonitor(monitor.id) : pauseMonitor(monitor.id)}>{paused ? <Play className="mr-2 h-4 w-4" /> : <Pause className="mr-2 h-4 w-4" />}{paused ? "Resume Monitor" : "Pause Monitor"}</Button>
                    </div>}
                </div>
            </header>

            <Tabs value={activeTab} onValueChange={tab => updateQuery({ tab: tab === "overview" ? null : tab })}>
                <TabsList className={`grid h-11 w-full sm:w-auto ${canEdit ? "grid-cols-3" : "grid-cols-2"}`}>
                    <TabsTrigger value="overview" className="min-w-24"><Activity className="mr-2 h-4 w-4" />Overview</TabsTrigger>
                    <TabsTrigger value="incidents" className="min-w-24"><Calendar className="mr-2 h-4 w-4" />Incidents</TabsTrigger>
                    {canEdit && <TabsTrigger value="settings" className="min-w-24" data-testid="monitor-settings-tab"><Settings2 className="mr-2 h-4 w-4" />Settings</TabsTrigger>}
                </TabsList>
                <TabsContent value="overview" className="mt-5"><MonitorOverview monitor={monitor} timezone={user?.timezone} /></TabsContent>
                <TabsContent value="incidents" className="mt-5"><IncidentsPanel id={id} date={date} incidents={incidents} sslWarnings={sslWarnings} query={incidentsQuery} onDateChange={value => updateQuery({ date: value === today() ? null : value })} onAll={() => navigate(`/incidents?monitorId=${id}`)} /></TabsContent>
                {canEdit && <TabsContent value="settings" className="mt-5"><MonitorSettings monitor={monitor} groupId={group.id} /></TabsContent>}
            </Tabs>
        </div>
    );
}

function Meta({ label, value }: { label: string; value: string }) { return <div className="flex gap-1.5"><dt>{label}</dt><dd className="font-medium text-foreground">{value}</dd></div>; }

function IncidentsPanel({ date, incidents, sslWarnings, query, onDateChange, onAll }: { date: string; incidents: SystemIncident[]; sslWarnings: SSLWarning[]; query: ReturnType<typeof useFilteredSystemEvents>; onDateChange: (value: string) => void; onAll: () => void; id: string }) {
    return <div className="space-y-5">
        <div className="flex flex-col gap-3 rounded-xl border border-border bg-card p-4 sm:flex-row sm:items-end sm:justify-between"><div className="flex flex-wrap items-end gap-2"><div className="grid gap-1.5"><Label htmlFor="monitor-incident-date" className="text-xs text-muted-foreground">Date (UTC)</Label><Input id="monitor-incident-date" type="date" value={date} onChange={event => onDateChange(event.target.value)} className="w-44" /></div><Button variant="outline" onClick={() => onDateChange(today())}>Today</Button></div><Button variant="ghost" className="self-start whitespace-nowrap sm:self-auto" onClick={onAll}>All monitor incidents</Button></div>
        <section aria-labelledby="incidents-title"><h2 id="incidents-title" className="mb-3 text-sm font-medium">Incidents ({incidents.length + sslWarnings.length})</h2>
            {query.isLoading ? <div className="space-y-2">{[...Array(3)].map((_, index) => <Skeleton key={index} className="h-16 w-full" />)}</div> : query.error ? <div className="rounded-lg border border-rose-500/30 bg-rose-500/5 p-4 text-sm text-rose-400">Could not load incidents for this date.</div> : incidents.length === 0 && sslWarnings.length === 0 ? <div className="grid min-h-44 place-items-center rounded-xl border border-dashed border-border bg-card/30 px-4 text-center text-sm text-muted-foreground">No incidents recorded on {date}.</div> : <div className="space-y-2">{incidents.map(incident => <IncidentCard key={incident.id} monitorId={incident.monitorId} monitorName={incident.monitorName} groupName={incident.groupName} onMonitorPage type={incident.type as "down" | "degraded"} summary={incident.message} startedAt={incident.startedAt} endedAt={incident.resolvedAt} duration={incident.duration ?? "0m"} />)}{sslWarnings.map(warning => <IncidentCard key={`ssl-${warning.id}`} monitorId={warning.monitorId} monitorName={warning.monitorName} groupName={warning.groupName} onMonitorPage type="ssl_expiring" summary={warning.message} startedAt={warning.timestamp} duration="" />)}</div>}
        </section>
    </div>;
}

function MonitorPageSkeleton() { return <div className="space-y-5"><Skeleton className="h-44 w-full rounded-xl" /><Skeleton className="h-11 w-80" /><div className="grid grid-cols-2 gap-3 lg:grid-cols-4">{[...Array(4)].map((_, index) => <Skeleton key={index} className="h-28 rounded-xl" />)}</div><Skeleton className="h-80 w-full rounded-xl" /></div>; }
