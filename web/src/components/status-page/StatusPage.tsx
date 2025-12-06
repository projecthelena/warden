import { useEffect, useState } from "react";
import { Monitor, Group, Incident, useMonitorStore } from "@/lib/store";
import { CheckCircle2, AlertTriangle, XCircle, Activity, ChevronDown, ChevronRight } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { UptimeHistory } from "@/components/ui/monitor-visuals";
import { useParams } from "react-router-dom";
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
    const { slug } = useParams();
    const { fetchPublicStatusBySlug } = useMonitorStore();
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [data, setData] = useState<{ title: string, groups: Group[], incidents: Incident[] } | null>(null);

    useEffect(() => {
        const load = async () => {
            setLoading(true);
            const result = await fetchPublicStatusBySlug(slug || 'all');
            if (result) {
                setData(result);
            } else {
                setError("Status page not found or private.");
            }
            setLoading(false);
        };
        load();
    }, [slug]);

    if (loading) {
        return (
            <div className="min-h-screen bg-[#020617] flex items-center justify-center text-slate-100">
                <div className="flex items-center gap-2">
                    <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                    Loading Status...
                </div>
            </div>
        )
    }

    if (error || !data) {
        return (
            <div className="min-h-screen bg-[#020617] flex items-center justify-center text-slate-100">
                <div className="text-center">
                    <Activity className="w-12 h-12 text-slate-600 mx-auto mb-4" />
                    <h1 className="text-xl font-semibold mb-2">Status Page Unavailable</h1>
                    <p className="text-slate-500">{error || "Could not load status information."}</p>
                </div>
            </div>
        )
    }

    return (
        <div className="min-h-screen bg-[#020617] text-slate-100 font-sans">
            <header className="max-w-3xl mx-auto pt-12 pb-8 px-6 text-center">
                <div className="flex items-center justify-center gap-3 mb-2">
                    <Activity className="w-8 h-8 text-blue-500" />
                    <h1 className="text-3xl font-bold tracking-tight">{data.title}</h1>
                </div>
            </header>

            <main className="max-w-3xl mx-auto px-6 pb-20">
                <GlobalStatus groups={data.groups || []} incidents={data.incidents || []} />

                {/* Incidents Section */}
                {data.incidents && data.incidents.length > 0 && (
                    <div className="mb-12">
                        <h2 className="text-xl font-semibold mb-4">Active Incidents</h2>
                        <Card className="bg-slate-900 border-slate-800">
                            <CardContent className="pt-6">
                                {data.incidents.map(i => <IncidentItem key={i.id} incident={i} />)}
                            </CardContent>
                        </Card>
                    </div>
                )}

                <div className="space-y-8 mb-16">
                    {data.groups && data.groups.map(group => (
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
            </main>

            <footer className="border-t border-slate-900 py-8 text-center text-slate-600 text-sm">
                Powered by <a href="#" className="hover:text-slate-400">ClusterUptime</a>
            </footer>
        </div>
    )
}
