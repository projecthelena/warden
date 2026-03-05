import { useEffect, useState } from "react";
import { useMonitorStore, SystemStats } from "@/lib/store";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { formatBytes } from "@/lib/utils";
import { Activity, CheckCircle2, XCircle, AlertTriangle } from "lucide-react";

function MonitorHealthCard({ stats }: { stats: SystemStats["stats"] }) {
    const total = stats.totalMonitors;
    const upPct = total > 0 ? (stats.activeMonitors / total) * 100 : 0;
    const downPct = total > 0 ? (stats.downMonitors / total) * 100 : 0;
    const degradedPct = total > 0 ? (stats.degradedMonitors / total) * 100 : 0;

    return (
        <Card>
            <CardHeader>
                <CardTitle>Monitor Health</CardTitle>
                <CardDescription>Current status of all monitored services.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                {total > 0 && (
                    <div className="flex h-2 rounded-full overflow-hidden bg-muted">
                        {stats.activeMonitors > 0 && (
                            <div className="bg-green-500 transition-all" style={{ width: `${upPct}%` }} />
                        )}
                        {stats.degradedMonitors > 0 && (
                            <div className="bg-yellow-500 transition-all" style={{ width: `${degradedPct}%` }} />
                        )}
                        {stats.downMonitors > 0 && (
                            <div className="bg-red-500 transition-all" style={{ width: `${downPct}%` }} />
                        )}
                    </div>
                )}
                <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
                    <div className="flex items-center gap-2">
                        <Activity className="h-4 w-4 text-muted-foreground" />
                        <div>
                            <div className="text-2xl font-bold">{stats.totalMonitors}</div>
                            <p className="text-xs text-muted-foreground">Total</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <CheckCircle2 className="h-4 w-4 text-green-500" />
                        <div>
                            <div className="text-2xl font-bold">{stats.activeMonitors}</div>
                            <p className="text-xs text-muted-foreground">Active</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <XCircle className="h-4 w-4 text-red-500" />
                        <div>
                            <div className="text-2xl font-bold">{stats.downMonitors}</div>
                            <p className="text-xs text-muted-foreground">Down</p>
                        </div>
                    </div>
                    <div className="flex items-center gap-2">
                        <AlertTriangle className="h-4 w-4 text-yellow-500" />
                        <div>
                            <div className="text-2xl font-bold">{stats.degradedMonitors}</div>
                            <p className="text-xs text-muted-foreground">Degraded</p>
                        </div>
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

function SystemDetailsCard({ data }: { data: SystemStats }) {
    const details = [
        { label: "Version", value: data.version },
        { label: "Database Size", value: formatBytes(data.dbSize) },
        { label: "Estimated Daily Pings", value: data.stats.dailyPingsEstimate.toLocaleString() },
        { label: "Total Groups", value: String(data.stats.totalGroups) },
    ];

    return (
        <Card>
            <CardHeader>
                <CardTitle>System Details</CardTitle>
                <CardDescription>Runtime and storage information.</CardDescription>
            </CardHeader>
            <CardContent>
                <dl className="grid gap-3">
                    {details.map(({ label, value }) => (
                        <div key={label} className="flex items-center justify-between py-2 border-b border-border last:border-0">
                            <dt className="text-sm text-muted-foreground">{label}</dt>
                            <dd className="text-sm font-medium font-mono">{value}</dd>
                        </div>
                    ))}
                </dl>
            </CardContent>
        </Card>
    );
}

export function SystemTab() {
    const { fetchSystemStats } = useMonitorStore();
    const [data, setData] = useState<SystemStats | null>(null);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        fetchSystemStats().then((stats) => {
            setData(stats);
            setLoading(false);
        });
    }, [fetchSystemStats]);

    if (loading) {
        return <div className="p-4 text-muted-foreground">Loading system stats...</div>;
    }

    if (!data) {
        return <div className="p-4 text-destructive">Failed to load system stats.</div>;
    }

    return (
        <div className="space-y-6">
            <MonitorHealthCard stats={data.stats} />
            <SystemDetailsCard data={data} />
        </div>
    );
}
