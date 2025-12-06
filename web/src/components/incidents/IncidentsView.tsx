import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { useMonitorStore, Incident } from "@/lib/store";
import { AlertCircle, Calendar, CheckCircle2 } from "lucide-react";

function IncidentCard({ incident }: { incident: Incident }) {
    const isMaintenance = incident.type === 'maintenance';

    return (
        <Card className="bg-slate-900/20 border-slate-800">
            <CardHeader>
                <div className="flex justify-between items-start">
                    <div className="space-y-1">
                        <CardTitle className="text-base flex items-center gap-2">
                            {isMaintenance ? <Calendar className="w-4 h-4 text-blue-400" /> : <AlertCircle className="w-4 h-4 text-red-500" />}
                            {incident.title}
                        </CardTitle>
                        <CardDescription>{new Date(incident.startTime).toLocaleString()}</CardDescription>
                    </div>
                    <Badge variant={isMaintenance ? "secondary" : "destructive"} className="uppercase text-[10px]">
                        {incident.status.replace('_', ' ')}
                    </Badge>
                </div>
            </CardHeader>
            <CardContent>
                <p className="text-sm text-slate-300 mb-4">{incident.description}</p>
                <div className="text-xs text-slate-500">
                    Affected: {incident.affectedGroups.join(", ")}
                </div>
            </CardContent>
        </Card>
    )
}

export function IncidentsView() {
    const { incidents } = useMonitorStore();

    const activeIncidents = incidents.filter(i => i.type === 'incident' && i.status !== 'resolved');
    const maintenance = incidents.filter(i => i.type === 'maintenance' && i.status !== 'completed');
    const history = incidents.filter(i => i.status === 'resolved' || i.status === 'completed');

    return (
        <div className="space-y-6">
            <div className="flex items-center justify-between">
                <h2 className="text-2xl font-bold tracking-tight">Incidents & Maintenance</h2>
            </div>

            <Tabs defaultValue="active" className="w-full">
                <TabsList className="bg-slate-900 border border-slate-800">
                    <TabsTrigger value="active">Open Incidents ({activeIncidents.length})</TabsTrigger>
                    <TabsTrigger value="maintenance">Scheduled ({maintenance.length})</TabsTrigger>
                    <TabsTrigger value="history">History</TabsTrigger>
                </TabsList>

                <TabsContent value="active" className="mt-6 space-y-4">
                    {activeIncidents.length === 0 && (
                        <div className="flex flex-col items-center justify-center p-12 text-slate-500 border border-dashed border-slate-800 rounded-lg">
                            <CheckCircle2 className="w-12 h-12 mb-4 text-green-500/50" />
                            <p>All systems operational. No active incidents.</p>
                        </div>
                    )}
                    {activeIncidents.map(i => <IncidentCard key={i.id} incident={i} />)}
                </TabsContent>

                <TabsContent value="maintenance" className="mt-6 space-y-4">
                    {maintenance.length === 0 && <div className="text-center text-slate-500 py-12">No scheduled maintenance.</div>}
                    {maintenance.map(i => <IncidentCard key={i.id} incident={i} />)}
                </TabsContent>

                <TabsContent value="history" className="mt-6 space-y-4">
                    {history.map(i => <IncidentCard key={i.id} incident={i} />)}
                </TabsContent>
            </Tabs>
        </div>
    )
}
