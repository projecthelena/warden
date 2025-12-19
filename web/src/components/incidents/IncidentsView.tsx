
import { useEffect } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { useMonitorStore, Incident, SystemIncident } from "@/lib/store";
import { Calendar, CheckCircle2, ArrowDownCircle, AlertTriangle, Clock } from "lucide-react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { cn, formatDate } from "@/lib/utils";

function IncidentCard({ incident, timezone }: { incident: Incident, timezone?: string }) {
    const isMaintenance = incident.type === 'maintenance';

    return (
        <div className="flex items-center justify-between p-4 rounded-xl border border-border/40 bg-card/30 hover:bg-card/50 transition-all duration-200">
            <div className="space-y-1">
                <div className="flex items-center gap-3">
                    {isMaintenance ? <Calendar className="w-4 h-4 text-blue-400" /> : <AlertTriangle className="w-4 h-4 text-red-500" />}
                    <span className="font-medium text-foreground">{incident.title}</span>
                    <Badge variant="outline" className={cn(
                        "text-[10px] uppercase tracking-wider font-mono px-1.5 py-0 h-auto border-0",
                        isMaintenance ? "bg-blue-500/10 text-blue-400" : "bg-red-500/10 text-red-500"
                    )}>
                        {incident.status.replace('_', ' ')}
                    </Badge>
                </div>
                <div className="text-sm text-muted-foreground pl-7">
                    {incident.description}
                </div>
            </div>
            <div className="text-right text-xs text-muted-foreground tabular-nums">
                {formatDate(incident.startTime, timezone)}
            </div>
        </div>
    )
}

function SystemEventRow({ event, active, timezone }: { event: SystemIncident; active: boolean; timezone?: string }) {
    const navigate = useNavigate();
    const isDown = event.type === 'down';

    // Minimalist Icon & Color Logic
    const colorClass = isDown ? "text-red-500" : "text-yellow-500";
    const bgBadge = isDown ? "bg-red-500/10 text-red-500 hover:bg-red-500/20" : "bg-yellow-500/10 text-yellow-500 hover:bg-yellow-500/20";

    // Duration Calculation (Client-side for active to be reactive, Server provided for history)
    let durationStr = event.duration || "Just now";

    // Override for active to be live (optional, but good UX)
    if (active && event.startedAt) {
        const start = new Date(event.startedAt).getTime();
        const now = new Date().getTime();
        const diffMins = Math.floor((now - start) / 60000);
        durationStr = diffMins < 1 ? "Just now" : `${diffMins}m ongoing`;
        if (diffMins >= 60) {
            const h = Math.floor(diffMins / 60);
            const m = diffMins % 60;
            durationStr = `${h}h ${m}m ongoing`;
        }
    }

    return (
        <div
            onClick={() => navigate(`/groups/${event.groupId}`)}
            className="group flex items-center justify-between py-3 px-4 -mx-4 hover:bg-muted/30 rounded-lg transition-colors cursor-pointer"
        >
            <div className="flex items-center gap-4">
                {/* Status Indicator Icon */}
                <div className={cn("flex items-center justify-center w-8 h-8 rounded-full bg-background border border-border/50", active ? "shadow-sm" : "opacity-70")}>
                    {isDown ? (
                        <ArrowDownCircle className={cn("w-4 h-4", colorClass)} />
                    ) : (
                        <AlertTriangle className={cn("w-4 h-4", colorClass)} />
                    )}
                </div>

                <div className="space-y-0.5">
                    <div className="flex items-center gap-2 text-sm">
                        {event.groupName && (
                            <>
                                <span className="text-muted-foreground hover:text-foreground transition-colors">
                                    {event.groupName}
                                </span>
                                <span className="text-muted-foreground/30">/</span>
                            </>
                        )}
                        <span className={cn("font-medium text-foreground", !active && "text-muted-foreground line-through decoration-border")}>
                            {event.monitorName}
                        </span>
                        {active && (
                            <Badge variant="secondary" className={cn("ml-2 rounded-sm px-1.5 py-0 text-[10px] font-medium uppercase tracking-wider border-0", bgBadge)}>
                                {event.type === 'down' ? 'UNAVAILABLE' : event.type}
                            </Badge>
                        )}
                    </div>
                    <div className="text-xs text-muted-foreground/70 font-mono">
                        {event.message}
                    </div>
                </div>
            </div>

            <div className="flex items-center gap-4 text-xs text-muted-foreground tabular-nums">
                <span className="flex items-center gap-1.5 opacity-50 group-hover:opacity-100 transition-opacity">
                    <Clock className="w-3 h-3" />
                    {active ?
                        // For active items, maybe showing relative time or time started? 
                        // User wants local time.
                        formatDate(event.startedAt, timezone).split(',')[1]?.trim() || formatDate(event.startedAt, timezone)
                        : formatDate(event.startedAt, timezone)
                    }
                </span>
                <span className={cn("font-medium", active ? colorClass : "text-emerald-500")}>
                    {durationStr}
                </span>
            </div>
        </div>
    )
}

export function IncidentsView() {
    const { incidents, systemEvents, fetchSystemEvents, fetchIncidents } = useMonitorStore();
    const [searchParams] = useSearchParams();
    const navigate = useNavigate();

    useEffect(() => {
        fetchSystemEvents();
        fetchIncidents();
    }, [fetchSystemEvents, fetchIncidents]);

    const currentTab = searchParams.get('tab') || 'active';

    const handleTabChange = (value: string) => {
        if (value === 'active') {
            navigate('/incidents');
        } else {
            navigate(`/incidents?tab=${value}`);
        }
    };

    const activeIncidents = incidents.filter(i => i.type === 'incident' && i.status !== 'resolved');
    const history = incidents.filter(i => i.status === 'resolved' || i.status === 'completed');

    const activeSystemEvents = systemEvents?.active || [];
    const historySystemEvents = systemEvents?.history || [];

    // Split Active Events
    const downtimeEvents = activeSystemEvents.filter(e => e.type === 'down');
    const degradedEvents = activeSystemEvents.filter(e => e.type === 'degraded');

    const totalActive = activeIncidents.length + activeSystemEvents.length;

    return (
        <div className="space-y-8 max-w-5xl mx-auto">
            <div className="flex items-center justify-between border-b border-border/40 pb-6">
                <div>
                    <h2 className="text-xl font-semibold tracking-tight text-foreground">Monitor Events</h2>
                    <p className="text-sm text-muted-foreground mt-1">Track active downtimes and review historical monitor events.</p>
                </div>
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
                            <div className="rounded-xl border border-red-900/20 bg-red-950/5 overflow-hidden px-4">
                                {downtimeEvents.map((e, i) => <SystemEventRow key={e.id + i} event={e} active={true} />)}
                            </div>
                        </div>
                    )}

                    {/* Degraded Performance Section */}
                    {degradedEvents.length > 0 && (
                        <div className="space-y-3 animation-in fade-in slide-in-from-bottom-3 duration-500">
                            <h3 className="text-xs font-semibold text-yellow-500 uppercase tracking-widest pl-1">Performance Issues</h3>
                            <div className="rounded-xl border border-yellow-900/20 bg-yellow-950/5 overflow-hidden px-4">
                                {degradedEvents.map((e, i) => <SystemEventRow key={e.id + i} event={e} active={true} />)}
                            </div>
                        </div>
                    )}

                    {/* Manual Incidents (General) */}
                    {activeIncidents.length > 0 && (
                        <div className="space-y-3 animation-in fade-in slide-in-from-bottom-4 duration-500">
                            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-widest pl-1">Reported Incidents</h3>
                            <div className="space-y-3">
                                {activeIncidents.map(i => <IncidentCard key={i.id} incident={i} />)}
                            </div>
                        </div>
                    )}
                </TabsContent>

                <TabsContent value="history" className="mt-8 space-y-4 focus-visible:outline-none focus-visible:ring-0">
                    {historySystemEvents.length === 0 && history.length === 0 && (
                        <div className="text-center text-muted-foreground/50 py-16 text-sm">No recent history.</div>
                    )}

                    <div className="divide-y divide-border/30">
                        {historySystemEvents.map((e, i) => <SystemEventRow key={e.id + i} event={e} active={false} />)}
                    </div>

                    <div className="pt-6 space-y-3">
                        {history.map(i => <IncidentCard key={i.id} incident={i} />)}
                    </div>
                </TabsContent>
            </Tabs>
        </div>
    )
}
