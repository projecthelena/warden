
import { useEffect, useState } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { useMonitorStore, Incident, SystemIncident } from "@/lib/store";
import { useFilteredSystemEvents } from "@/hooks/useSystemEvents";
import { IncidentCard as OutageCard } from "@/components/IncidentCard";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Calendar, CheckCircle2, AlertTriangle, Megaphone, Eye, EyeOff, ChevronDown, ChevronUp } from "lucide-react";
import { useSearchParams } from "react-router-dom";
import { cn, formatDate } from "@/lib/utils";
import { PromoteOutageDialog } from "./PromoteOutageDialog";
import { IncidentTimeline } from "./IncidentTimeline";
import { useRole } from "@/hooks/useRole";

function ManualIncidentCard({ incident, timezone, onAddUpdate, onToggleVisibility }: {
    incident: Incident;
    timezone?: string;
    onAddUpdate?: (status: string, message: string) => Promise<void>;
    onToggleVisibility?: () => void;
}) {
    const [isExpanded, setIsExpanded] = useState(false);
    const isMaintenance = incident.type === 'maintenance';
    const isActive = incident.status !== 'resolved' && incident.status !== 'completed';
    const hasUpdates = incident.updates && incident.updates.length > 0;

    return (
        <div className="rounded-xl border border-border/40 bg-card/30 overflow-hidden">
            <div
                className="flex items-center justify-between p-4 hover:bg-card/50 transition-all duration-200 cursor-pointer"
                onClick={() => setIsExpanded(!isExpanded)}
            >
                <div className="space-y-1 flex-1 min-w-0">
                    <div className="flex items-center gap-3 flex-wrap">
                        {isMaintenance ? <Calendar className="w-4 h-4 text-blue-400 shrink-0" /> : <AlertTriangle className="w-4 h-4 text-red-500 shrink-0" />}
                        <span className="font-medium text-foreground truncate">{incident.title}</span>
                        <Badge variant="outline" className={cn(
                            "text-[10px] uppercase tracking-wider font-mono px-1.5 py-0 h-auto border-0 shrink-0",
                            isMaintenance ? "bg-blue-500/10 text-blue-400" : "bg-red-500/10 text-red-500"
                        )}>
                            {incident.status.replace('_', ' ')}
                        </Badge>
                        {incident.public && (
                            <Badge variant="outline" className="text-[10px] uppercase tracking-wider font-mono px-1.5 py-0 h-auto border-0 bg-emerald-500/10 text-emerald-500 shrink-0">
                                Public
                            </Badge>
                        )}
                        {incident.source === 'auto' && (
                            <Badge variant="outline" className="text-[10px] uppercase tracking-wider font-mono px-1.5 py-0 h-auto border-0 bg-purple-500/10 text-purple-500 shrink-0">
                                Auto
                            </Badge>
                        )}
                    </div>
                    {incident.description && (
                        <div className="text-sm text-muted-foreground pl-7 truncate">
                            {incident.description}
                        </div>
                    )}
                </div>
                <div className="flex items-center gap-3 shrink-0">
                    <span className="text-xs text-muted-foreground tabular-nums">
                        {formatDate(incident.startTime, timezone)}
                    </span>
                    {(hasUpdates || isActive) && (
                        isExpanded ? <ChevronUp className="w-4 h-4 text-muted-foreground" /> : <ChevronDown className="w-4 h-4 text-muted-foreground" />
                    )}
                </div>
            </div>

            {isExpanded && (
                <div className="px-4 pb-4 border-t border-border/30 pt-4 space-y-4">
                    {/* Visibility Toggle */}
                    {onToggleVisibility && !isMaintenance && (
                        <div className="flex items-center justify-between">
                            <span className="text-xs text-muted-foreground">Status Page Visibility</span>
                            <Button
                                variant="outline"
                                size="sm"
                                onClick={(e) => { e.stopPropagation(); onToggleVisibility(); }}
                                className="h-7 text-xs"
                            >
                                {incident.public ? (
                                    <><EyeOff className="w-3 h-3 mr-1.5" /> Make Private</>
                                ) : (
                                    <><Eye className="w-3 h-3 mr-1.5" /> Make Public</>
                                )}
                            </Button>
                        </div>
                    )}

                    {/* Timeline */}
                    <IncidentTimeline
                        updates={incident.updates || []}
                        onAddUpdate={isActive && onAddUpdate ? onAddUpdate : undefined}
                        readonly={!isActive}
                        timezone={timezone}
                    />
                </div>
            )}
        </div>
    )
}

export function IncidentsView() {
    const { incidents, systemEvents, fetchSystemEvents, fetchIncidents, user, promoteOutage, addIncidentUpdate, setIncidentVisibility, getIncidentWithUpdates, groups } = useMonitorStore();
    const { canEdit } = useRole();
    const timezone = user?.timezone;
    const [searchParams, setSearchParams] = useSearchParams();

    // Promote dialog state
    const [promoteDialogOpen, setPromoteDialogOpen] = useState(false);
    const [selectedOutage, setSelectedOutage] = useState<SystemIncident | null>(null);

    // Expanded incidents with updates loaded
    const [incidentUpdates, setIncidentUpdates] = useState<Record<string, Incident>>({});

    // Three optional filters, all combinable, all URL-synced. Slack digest deep-links use
    // ?date=; the MonitorPage cross-link uses ?monitorId=; the Group dropdown sets ?groupId=.
    const dateFilter = searchParams.get('date') || '';
    const isValidDate = /^\d{4}-\d{2}-\d{2}$/.test(dateFilter);
    const monitorFilter = searchParams.get('monitorId') || '';
    const groupFilter = searchParams.get('groupId') || '';
    const hasAnyFilter = isValidDate || !!monitorFilter || !!groupFilter;

    // When any filter is active, fetch into a local snapshot so the global App-level poll
    // (which hits /api/events with no filters and writes to Zustand) can't clobber it.
    const scoped = useFilteredSystemEvents({
        date: isValidDate ? dateFilter : undefined,
        monitorId: monitorFilter || undefined,
        groupId: groupFilter || undefined,
    });

    useEffect(() => {
        // Only refresh the global store on the fully-unfiltered view; filtered mode reads
        // from `scoped` instead.
        if (!hasAnyFilter) {
            fetchSystemEvents();
        }
        fetchIncidents();
    }, [fetchSystemEvents, fetchIncidents, hasAnyFilter]);

    // Load updates for all incidents
    useEffect(() => {
        const loadUpdates = async () => {
            const updates: Record<string, Incident> = {};
            for (const inc of incidents) {
                if (inc.type === 'incident') {
                    const full = await getIncidentWithUpdates(inc.id);
                    if (full) {
                        updates[inc.id] = full;
                    }
                }
            }
            setIncidentUpdates(updates);
        };
        if (incidents.length > 0) {
            loadUpdates();
        }
    }, [incidents, getIncidentWithUpdates]);

    // When a date filter is present, default to History tab (active outages don't have a date).
    const currentTab = searchParams.get('tab') || (isValidDate ? 'history' : 'active');

    const handleTabChange = (value: string) => {
        const next = new URLSearchParams(searchParams);
        if (value === 'active') {
            next.delete('tab');
        } else {
            next.set('tab', value);
        }
        setSearchParams(next, { replace: true });
    };

    const updateFilter = (key: 'date' | 'monitorId' | 'groupId', value: string) => {
        const next = new URLSearchParams(searchParams);
        if (!value) {
            next.delete(key);
        } else {
            next.set(key, value);
        }
        // When the group changes, drop the monitor filter if it no longer belongs to that group.
        if (key === 'groupId') {
            const m = next.get('monitorId');
            if (m && value) {
                const stillBelongs = groups?.some(g => g.id === value && g.monitors?.some(mm => mm.id === m));
                if (!stillBelongs) next.delete('monitorId');
            }
        }
        setSearchParams(next, { replace: true });
    };

    const clearAllFilters = () => {
        const next = new URLSearchParams(searchParams);
        next.delete('date');
        next.delete('monitorId');
        next.delete('groupId');
        setSearchParams(next, { replace: true });
    };

    // Monitors available in the dropdown: all by default, or only those in the selected group.
    const visibleMonitors = (() => {
        if (!groups) return [];
        if (groupFilter) {
            const g = groups.find(gg => gg.id === groupFilter);
            return g?.monitors ?? [];
        }
        return groups.flatMap(g => g.monitors ?? []);
    })();

    const handlePromoteOutage = (event: SystemIncident) => {
        setSelectedOutage(event);
        setPromoteDialogOpen(true);
    };

    const handlePromoteSubmit = async (outageId: string, data: { title: string; description: string; severity: string; affectedGroups: string[] }) => {
        // outageId from /api/events is already the numeric monitor_outages.id
        await promoteOutage(outageId, data);
    };

    const handleAddUpdate = (incidentId: string) => async (status: string, message: string) => {
        await addIncidentUpdate(incidentId, status, message);
        // Refresh the incident with updates
        const full = await getIncidentWithUpdates(incidentId);
        if (full) {
            setIncidentUpdates(prev => ({ ...prev, [incidentId]: full }));
        }
    };

    const handleToggleVisibility = (incidentId: string, currentPublic: boolean) => async () => {
        await setIncidentVisibility(incidentId, !currentPublic);
    };

    const activeIncidents = incidents.filter(i => i.type === 'incident' && i.status !== 'resolved' && i.status !== 'completed');
    let history = incidents.filter(i => i.status === 'resolved' || i.status === 'completed');

    // Prefer the local scoped fetch when any filter is active, so the global App-level
    // poll over /api/events can't overwrite the filtered list mid-view.
    const sourceSystemEvents = hasAnyFilter
        ? (scoped.data ?? { active: [], history: [], sslWarnings: [] })
        : (systemEvents ?? { active: [], history: [], sslWarnings: [] });
    const activeSystemEvents = sourceSystemEvents.active || [];
    const historySystemEvents = sourceSystemEvents.history || [];
    const sslWarnings = sourceSystemEvents.sslWarnings || [];

    // Apply filters to manual incidents client-side (we don't have a server-side filter
    // for them yet, but volume is low). Date scopes to that UTC day; monitor/group match
    // against the affectedGroups list — manual incidents are group-scoped, not per-monitor,
    // so monitorFilter narrows to incidents whose affectedGroups include the monitor's group.
    if (isValidDate) {
        const dayStart = new Date(`${dateFilter}T00:00:00Z`).getTime();
        const dayEnd = dayStart + 24 * 60 * 60 * 1000;
        history = history.filter(i => {
            const start = new Date(i.startTime).getTime();
            return start >= dayStart && start < dayEnd;
        });
    }
    if (groupFilter) {
        history = history.filter(i => i.affectedGroups?.includes(groupFilter));
    }
    if (monitorFilter) {
        // Resolve monitor → group to keep this consistent with how manual incidents target groups.
        const owningGroupId = groups?.find(g => g.monitors?.some(m => m.id === monitorFilter))?.id;
        if (owningGroupId) {
            history = history.filter(i => i.affectedGroups?.includes(owningGroupId));
        } else {
            history = [];
        }
    }

    // Split Active Events
    const downtimeEvents = activeSystemEvents.filter(e => e.type === 'down');
    const degradedEvents = activeSystemEvents.filter(e => e.type === 'degraded');

    const totalActive = activeIncidents.length + activeSystemEvents.length + sslWarnings.length;

    return (
        <div className="space-y-8 max-w-5xl mx-auto">
            <div className="flex items-center justify-between border-b border-border/40 pb-6">
                <div>
                    <h2 className="text-xl font-semibold tracking-tight text-foreground">Monitor Events</h2>
                    <p className="text-sm text-muted-foreground mt-1">Track active downtimes and review historical monitor events.</p>
                </div>
            </div>

            {/* Filter row — Group / Monitor / Date. All optional and combinable. URL syncs
                via useSearchParams so links from Slack, MonitorPage, and dashboard can deep-link
                into any combination. */}
            <div className="flex flex-wrap items-end gap-3">
                <div className="grid gap-1.5">
                    <span className="text-xs text-muted-foreground">Group</span>
                    <Select
                        value={groupFilter || "__all__"}
                        onValueChange={v => updateFilter('groupId', v === '__all__' ? '' : v)}
                    >
                        <SelectTrigger className="w-48"><SelectValue placeholder="All groups" /></SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__all__">All groups</SelectItem>
                            {(groups ?? []).map(g => (
                                <SelectItem key={g.id} value={g.id}>{g.name}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
                <div className="grid gap-1.5">
                    <span className="text-xs text-muted-foreground">Monitor</span>
                    <Select
                        value={monitorFilter || "__all__"}
                        onValueChange={v => updateFilter('monitorId', v === '__all__' ? '' : v)}
                    >
                        <SelectTrigger className="w-56"><SelectValue placeholder="All monitors" /></SelectTrigger>
                        <SelectContent>
                            <SelectItem value="__all__">All monitors</SelectItem>
                            {visibleMonitors.map(m => (
                                <SelectItem key={m.id} value={m.id}>{m.name}</SelectItem>
                            ))}
                        </SelectContent>
                    </Select>
                </div>
                <div className="grid gap-1.5">
                    <span className="text-xs text-muted-foreground flex items-center gap-1.5">
                        <Calendar className="w-3 h-3" />Date (UTC)
                    </span>
                    <input
                        type="date"
                        value={dateFilter}
                        onChange={e => updateFilter('date', e.target.value)}
                        className="h-9 w-44 rounded-md border border-input bg-background px-3 text-sm"
                    />
                </div>
                {hasAnyFilter && (
                    <Button variant="ghost" size="sm" onClick={clearAllFilters}>
                        Clear all
                    </Button>
                )}
            </div>

            <Tabs value={currentTab} onValueChange={handleTabChange} className="w-full">
                <TabsList className="bg-transparent border-b border-border/40 w-full justify-start h-auto p-0 space-x-6 rounded-none">
                    <TabsTrigger
                        value="active"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground data-[state=active]:text-foreground transition-all"
                    >
                        Active Issues
                        {totalActive > 0 && <span className="ml-2 bg-red-500/10 text-red-500 text-[10px] px-1.5 py-0.5 rounded-full">{totalActive}</span>}
                    </TabsTrigger>
                    <TabsTrigger
                        value="history"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground data-[state=active]:text-foreground transition-all"
                    >
                        History
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="active" className="mt-8 space-y-8 focus-visible:outline-none focus-visible:ring-0">
                    {totalActive === 0 && (
                        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground/60">
                            <CheckCircle2 className="w-10 h-10 mb-4 text-emerald-500/30" />
                            <p className="text-sm font-medium">All systems operational</p>
                            <p className="text-xs opacity-70 mt-1">No active incidents or anomalies.</p>
                        </div>
                    )}

                    {/* Critical Outages Section */}
                    {downtimeEvents.length > 0 && (
                        <div className="space-y-3 animation-in fade-in slide-in-from-bottom-2 duration-500">
                            <h3 className="text-xs font-semibold text-red-500 uppercase tracking-widest pl-1">Critical Outages</h3>
                            <div className="space-y-2">
                                {downtimeEvents.map(e => (
                                    <OutageCard
                                        key={e.id}
                                        monitorId={e.monitorId}
                                        monitorName={e.monitorName}
                                        groupName={e.groupName}
                                        type="down"
                                        summary={e.message}
                                        startedAt={e.startedAt}
                                        endedAt={e.resolvedAt}
                                        duration={e.duration ?? "0m"}
                                        rightAction={canEdit ? (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={ev => { ev.stopPropagation(); handlePromoteOutage(e); }}
                                                className="h-7 text-xs"
                                            >
                                                <Megaphone className="w-3 h-3 mr-1.5" />
                                                Promote
                                            </Button>
                                        ) : undefined}
                                    />
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Degraded Performance Section */}
                    {degradedEvents.length > 0 && (
                        <div className="space-y-3 animation-in fade-in slide-in-from-bottom-3 duration-500">
                            <h3 className="text-xs font-semibold text-yellow-500 uppercase tracking-widest pl-1">Performance Issues</h3>
                            <div className="space-y-2">
                                {degradedEvents.map(e => (
                                    <OutageCard
                                        key={e.id}
                                        monitorId={e.monitorId}
                                        monitorName={e.monitorName}
                                        groupName={e.groupName}
                                        type="degraded"
                                        summary={e.message}
                                        startedAt={e.startedAt}
                                        endedAt={e.resolvedAt}
                                        duration={e.duration ?? "0m"}
                                        rightAction={canEdit ? (
                                            <Button
                                                variant="outline"
                                                size="sm"
                                                onClick={ev => { ev.stopPropagation(); handlePromoteOutage(e); }}
                                                className="h-7 text-xs"
                                            >
                                                <Megaphone className="w-3 h-3 mr-1.5" />
                                                Promote
                                            </Button>
                                        ) : undefined}
                                    />
                                ))}
                            </div>
                        </div>
                    )}

                    {/* SSL Certificate Warnings Section */}
                    {sslWarnings.length > 0 && (
                        <div className="space-y-3 animation-in fade-in slide-in-from-bottom-3 duration-500">
                            <h3 className="text-xs font-semibold text-orange-500 uppercase tracking-widest pl-1">Certificate Warnings</h3>
                            <div className="space-y-2">
                                {sslWarnings.map(w => (
                                    <OutageCard
                                        key={w.id}
                                        monitorId={w.monitorId}
                                        monitorName={w.monitorName}
                                        groupName={w.groupName}
                                        type="ssl_expiring"
                                        summary={w.message}
                                        startedAt={w.timestamp}
                                        duration=""
                                    />
                                ))}
                            </div>
                        </div>
                    )}

                    {/* Manual Incidents (General) */}
                    {activeIncidents.length > 0 && (
                        <div className="space-y-3 animation-in fade-in slide-in-from-bottom-4 duration-500">
                            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-widest pl-1">Reported Incidents</h3>
                            <div className="space-y-3">
                                {activeIncidents.map(i => {
                                    const incWithUpdates = incidentUpdates[i.id] || i;
                                    return (
                                        <ManualIncidentCard
                                            key={i.id}
                                            incident={incWithUpdates}
                                            timezone={timezone}
                                            onAddUpdate={canEdit ? handleAddUpdate(i.id) : undefined}
                                            onToggleVisibility={canEdit ? handleToggleVisibility(i.id, incWithUpdates.public || false) : undefined}
                                        />
                                    );
                                })}
                            </div>
                        </div>
                    )}
                </TabsContent>

                <TabsContent value="history" className="mt-8 space-y-4 focus-visible:outline-none focus-visible:ring-0">
                    {historySystemEvents.length === 0 && history.length === 0 && (
                        <div className="text-center text-muted-foreground/50 py-16 text-sm">No recent history.</div>
                    )}

                    <div className="space-y-2">
                        {historySystemEvents.map(e => (
                            <OutageCard
                                key={e.id}
                                monitorId={e.monitorId}
                                monitorName={e.monitorName}
                                groupName={e.groupName}
                                type={e.type as 'down' | 'degraded'}
                                summary={e.message}
                                startedAt={e.startedAt}
                                endedAt={e.resolvedAt}
                                duration={e.duration ?? "0m"}
                            />
                        ))}
                    </div>

                    {history.length > 0 && (
                        <div className="pt-6 space-y-3">
                            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-widest pl-1">Reported Incidents</h3>
                            {history.map(i => {
                                const incWithUpdates = incidentUpdates[i.id] || i;
                                return <ManualIncidentCard key={i.id} incident={incWithUpdates} timezone={timezone} />;
                            })}
                        </div>
                    )}
                </TabsContent>
            </Tabs>

            {/* Promote Outage Dialog */}
            <PromoteOutageDialog
                outage={selectedOutage}
                open={promoteDialogOpen}
                onOpenChange={setPromoteDialogOpen}
                onPromote={handlePromoteSubmit}
            />
        </div>
    )
}
