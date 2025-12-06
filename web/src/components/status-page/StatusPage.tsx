import { Monitor, Group, Incident, useMonitorStore } from "@/lib/store";
import { CheckCircle2, AlertTriangle, XCircle, Activity, ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { UptimeHistory } from "@/components/ui/monitor-visuals";
import {
    Accordion,
    AccordionContent,
    AccordionItem,
    AccordionTrigger,
} from "@/components/ui/accordion"

function GlobalStatus({ groups, incidents }: { groups: Group[], incidents: Incident[] }) {
    const hasActiveIncidents = incidents.some(i => i.status !== 'resolved' && i.status !== 'completed' && i.status !== 'scheduled');
    const hasDown = groups.some(g => g.monitors.some(m => m.status === 'down'));
    const hasDegraded = groups.some(g => g.monitors.some(m => m.status === 'degraded'));

    if (hasActiveIncidents || hasDown) {
        return (
            <div className="bg-red-500 text-white p-4 rounded-lg flex items-center gap-3 mb-8 shadow-lg shadow-red-900/20">
                <XCircle className="w-6 h-6" />
                <div className="font-semibold text-lg">System Outage</div>
            </div>
        )
    }
    if (hasDegraded) {
        return (
            <div className="bg-yellow-500 text-slate-900 p-4 rounded-lg flex items-center gap-3 mb-8">
                <AlertTriangle className="w-6 h-6" />
                <div className="font-semibold text-lg">Partial System Degraded</div>
            </div>
        )
    }
    return (
        <div className="bg-green-500 text-white p-4 rounded-lg flex items-center gap-3 mb-8 shadow-lg shadow-green-900/20">
            <CheckCircle2 className="w-6 h-6" />
            <div className="font-semibold text-lg">All Systems Operational</div>
        </div>
    )
}

function PublicMonitor({ monitor }: { monitor: Monitor }) {
    const statusColor =
        monitor.status === 'up' ? 'text-green-500' :
            monitor.status === 'degraded' ? 'text-yellow-500' : 'text-red-500';

    return (
        <div className="flex flex-col sm:flex-row items-center justify-between py-4 border-b border-slate-800 last:border-0 gap-4">
            <div className="flex items-center justify-between w-full sm:w-auto gap-4">
                <div className="font-medium text-slate-200">{monitor.name}</div>
                <span className={`text-sm ${statusColor} capitalize sm:hidden`}>
                    {monitor.status.replace('_', ' ')}
                </span>
            </div>
            <div className="hidden sm:block flex-1 px-8 max-w-[200px]">
                <UptimeHistory history={monitor.history} />
            </div>
            <div className={`hidden sm:block text-sm font-medium ${statusColor} capitalize min-w-[100px] text-right`}>
                {monitor.status.replace('_', ' ')}
            </div>
        </div>
    )
}

function IncidentItem({ incident }: { incident: Incident }) {
    return (
        <div className="mb-6 last:mb-0 border-l-2 border-slate-700 pl-4 ml-1">
            <div className="font-semibold text-lg text-slate-200 mb-1">{incident.title}</div>
            <div className="flex gap-2 mb-2">
                <Badge variant={incident.type === 'maintenance' ? 'secondary' : 'destructive'}
                    className="uppercase text-[10px] tracking-wider">
                    {incident.status.replace('_', ' ')}
                </Badge>
                <span className="text-xs text-slate-500 mt-0.5">{new Date(incident.startTime).toLocaleString()}</span>
            </div>
            <div className="text-slate-400 text-sm">
                {incident.description}
            </div>
        </div>
    )
}

export function StatusPage() {
    const { groups, incidents } = useMonitorStore();

    const activeIncidents = incidents.filter(i => i.status !== 'resolved' && i.status !== 'completed');
    const pastIncidents = incidents.filter(i => i.status === 'resolved' || i.status === 'completed');

    return (
        <div className="min-h-screen bg-[#020617] text-slate-100 font-sans">
            <header className="max-w-3xl mx-auto pt-12 pb-8 px-6 text-center">
                <div className="flex items-center justify-center gap-3 mb-2">
                    <Activity className="w-8 h-8 text-blue-500" />
                    <h1 className="text-3xl font-bold tracking-tight">ClusterUptime Status</h1>
                </div>
            </header>

            <main className="max-w-3xl mx-auto px-6 pb-20">
                <GlobalStatus groups={groups} incidents={incidents} />

                {activeIncidents.length > 0 && (
                    <div className="mb-12">
                        <h2 className="text-xl font-semibold mb-4">Active Incidents</h2>
                        <Card className="bg-slate-900 border-slate-800">
                            <CardContent className="pt-6">
                                {activeIncidents.map(i => <IncidentItem key={i.id} incident={i} />)}
                            </CardContent>
                        </Card>
                    </div>
                )}

                <div className="space-y-8 mb-16">
                    {groups.map(group => (
                        <Card key={group.id} className="bg-slate-900/40 border-slate-800">
                            <CardHeader className="pb-2 border-b border-slate-800/50">
                                <CardTitle className="text-lg">{group.name}</CardTitle>
                            </CardHeader>
                            <CardContent className="pt-0">
                                {group.monitors.map(m => <PublicMonitor key={m.id} monitor={m} />)}
                            </CardContent>
                        </Card>
                    ))}
                </div>

                {pastIncidents.length > 0 && (
                    <div className="mb-12">
                        <h2 className="text-xl font-semibold mb-4">Past Incidents</h2>
                        <Accordion type="single" collapsible className="w-full">
                            <AccordionItem value="history" className="border-slate-800">
                                <AccordionTrigger className="text-slate-400 hover:text-slate-200 hover:no-underline">
                                    View Incident History
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="pt-4 space-y-6">
                                        {pastIncidents.map(i => <IncidentItem key={i.id} incident={i} />)}
                                    </div>
                                </AccordionContent>
                            </AccordionItem>
                        </Accordion>
                    </div>
                )}
            </main>

            <footer className="border-t border-slate-900 py-8 text-center text-slate-600 text-sm">
                Powered by <a href="#" className="hover:text-slate-400">ClusterUptime</a>
            </footer>
        </div>
    )
}
