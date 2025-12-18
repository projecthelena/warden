
import { useEffect } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { useMonitorStore, Incident } from "@/lib/store";
import { Calendar, CheckCircle2 } from "lucide-react";
import { cn } from "@/lib/utils";

function MaintenanceCard({ incident }: { incident: Incident }) {
    return (
        <div className="flex items-center justify-between p-4 rounded-xl border border-border/40 bg-card/30 hover:bg-card/50 transition-all duration-200">
            <div className="space-y-1">
                <div className="flex items-center gap-3">
                    <Calendar className="w-4 h-4 text-blue-400" />
                    <span className="font-medium text-foreground">{incident.title}</span>
                    <Badge variant="outline" className={cn(
                        "text-[10px] uppercase tracking-wider font-mono px-1.5 py-0 h-auto border-0 bg-blue-500/10 text-blue-400"
                    )}>
                        {incident.status.replace('_', ' ')}
                    </Badge>
                </div>
                <div className="text-sm text-muted-foreground pl-7">
                    {incident.description}
                </div>
            </div>
            <div className="text-right text-xs text-muted-foreground tabular-nums">
                {new Date(incident.startTime).toLocaleString()}
            </div>
        </div>
    )
}

export function MaintenanceView() {
    const { incidents, fetchIncidents } = useMonitorStore();

    useEffect(() => {
        fetchIncidents();
    }, [fetchIncidents]);

    // Filter maintenance
    const scheduled = incidents.filter(i => i.type === 'maintenance' && i.status !== 'completed');
    const history = incidents.filter(i => i.type === 'maintenance' && i.status === 'completed');

    return (
        <div className="space-y-8 max-w-5xl mx-auto">
            <div className="flex items-center justify-between border-b border-border/40 pb-6">
                <div>
                    <h2 className="text-xl font-semibold tracking-tight text-foreground">Maintenance</h2>
                    <p className="text-sm text-muted-foreground mt-1">Scheduled system maintenance and upgrades.</p>
                </div>
            </div>

            <Tabs defaultValue="scheduled" className="w-full">
                <TabsList className="bg-transparent border-b border-border/40 w-full justify-start h-auto p-0 space-x-6 rounded-none">
                    <TabsTrigger
                        value="scheduled"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground data-[state=active]:text-foreground transition-all"
                    >
                        Scheduled
                        {scheduled.length > 0 && <span className="ml-2 bg-blue-500/10 text-blue-400 text-[10px] px-1.5 py-0.5 rounded-full">{scheduled.length}</span>}
                    </TabsTrigger>
                    <TabsTrigger
                        value="history"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground data-[state=active]:text-foreground transition-all"
                    >
                        History
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="scheduled" className="mt-8 space-y-4 focus-visible:outline-none focus-visible:ring-0">
                    {scheduled.length === 0 && (
                        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground/60">
                            <CheckCircle2 className="w-10 h-10 mb-4 text-emerald-500/30" />
                            <p className="text-sm font-medium">No scheduled maintenance</p>
                            <p className="text-xs opacity-70 mt-1">All systems operating normally.</p>
                        </div>
                    )}
                    {scheduled.map(i => <MaintenanceCard key={i.id} incident={i} />)}
                </TabsContent>

                <TabsContent value="history" className="mt-8 space-y-4 focus-visible:outline-none focus-visible:ring-0">
                    {history.length === 0 && (
                        <div className="text-center text-muted-foreground/50 py-16 text-sm">No maintenance history.</div>
                    )}
                    {history.map(i => <MaintenanceCard key={i.id} incident={i} />)}
                </TabsContent>
            </Tabs>
        </div>
    )
}
