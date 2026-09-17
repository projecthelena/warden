/* Hallmark · genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app */
import { useEffect, useMemo, useState } from "react";
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { Activity, Clock3, Gauge, Waves } from "lucide-react";
import { Monitor } from "@/lib/store";
import { formatDate } from "@/lib/utils";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { UptimeHistory } from "@/components/ui/monitor-visuals";
import { InsightsCard } from "@/components/InsightsCard";
import { Tooltip as UiTooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { formatUptimeDetail, formatUptimeSummary, uptimeSummaryTone, type UptimeSummary } from "@/lib/uptime";

type Range = "1h" | "24h" | "7d" | "30d";
type UptimeStats = {
    windows: { last24Hours: UptimeSummary; last7Days: UptimeSummary; last30Days: UptimeSummary };
};
type LatencyPoint = { timestamp: string; latency: number | null; failed?: boolean };

const ranges: Range[] = ["1h", "24h", "7d", "30d"];

export function MonitorOverview({ monitor, timezone }: { monitor: Monitor; timezone?: string }) {
    const [range, setRange] = useState<Range>("1h");
    const [stats, setStats] = useState<UptimeStats | null>(null);
    const [latency, setLatency] = useState<LatencyPoint[]>([]);
    const [statsLoading, setStatsLoading] = useState(true);
    const [latencyLoading, setLatencyLoading] = useState(true);

    useEffect(() => {
        const controller = new AbortController();
        setStatsLoading(true);
        fetch(`/api/monitors/${monitor.id}/uptime`, { credentials: "include", signal: controller.signal }).then(r => {
            if (!r.ok) throw new Error("Failed to load uptime");
            return r.json();
        }).then(setStats).catch(error => {
            if (error.name !== "AbortError") setStats(null);
        }).finally(() => {
            if (!controller.signal.aborted) setStatsLoading(false);
        });
        return () => controller.abort();
    }, [monitor.id]);

    useEffect(() => {
        const controller = new AbortController();
        setLatencyLoading(true);
        fetch(`/api/monitors/${monitor.id}/latency?range=${range}`, { credentials: "include", signal: controller.signal }).then(r => {
            if (!r.ok) throw new Error("Failed to load latency");
            return r.json();
        }).then(setLatency).catch(error => {
            if (error.name !== "AbortError") setLatency([]);
        }).finally(() => {
            if (!controller.signal.aborted) setLatencyLoading(false);
        });
        return () => controller.abort();
    }, [monitor.id, range]);

    const chartData = useMemo(() => latency.map(point => ({
        ...point,
        timestampMs: new Date(point.timestamp).getTime(),
    })).sort((a, b) => a.timestampMs - b.timestampMs), [latency]);

    const averageLatency = useMemo(() => {
        const values = latency.flatMap(point => typeof point.latency === "number" ? [point.latency] : []);
        return values.length ? Math.round(values.reduce((sum, value) => sum + value, 0) / values.length) : null;
    }, [latency]);

    const tick = (value: number) => new Intl.DateTimeFormat("en-US", range === "1h" || range === "24h"
        ? { hour: "numeric", minute: "2-digit", timeZone: timezone || "UTC" }
        : { month: "short", day: "numeric", timeZone: timezone || "UTC" }
    ).format(new Date(value));

    return (
        <div className="space-y-6" data-testid="monitor-overview">
            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
                {statsLoading && !stats ? [...Array(4)].map((_, index) => <Skeleton key={index} className="h-28 rounded-xl" />) : (
                    <>
                        <Metric label="Current latency" value={monitor.status === "paused" ? "Paused" : `${monitor.latency} ms`} icon={<Gauge />} />
                        <Metric label="24h uptime" value={formatUptimeSummary(stats?.windows?.last24Hours)} detail={formatUptimeDetail(stats?.windows?.last24Hours)} tone={uptimeSummaryTone(stats?.windows?.last24Hours)} icon={<Activity />} />
                        <Metric label="7d uptime" value={formatUptimeSummary(stats?.windows?.last7Days)} detail={formatUptimeDetail(stats?.windows?.last7Days)} tone={uptimeSummaryTone(stats?.windows?.last7Days)} icon={<Waves />} />
                        <Metric label="30d uptime" value={formatUptimeSummary(stats?.windows?.last30Days)} detail={formatUptimeDetail(stats?.windows?.last30Days)} tone={uptimeSummaryTone(stats?.windows?.last30Days)} icon={<Clock3 />} />
                    </>
                )}
            </div>

            <Card className="overflow-hidden border-border bg-card shadow-none">
                <CardHeader className="flex flex-row items-center justify-between gap-4 space-y-0 border-b border-border p-4 sm:p-5">
                    <div className="min-w-0">
                        <CardTitle className="text-base">Response time</CardTitle>
                        <p className="mt-1 text-xs text-muted-foreground">
                            {averageLatency == null ? "No successful checks in this range" : `${averageLatency} ms average over ${range}`}
                        </p>
                    </div>
                    <div className="flex shrink-0 rounded-lg border border-border bg-muted/40 p-1" aria-label="Response time range">
                        {ranges.map(item => (
                            <Button key={item} type="button" size="sm" variant={range === item ? "secondary" : "ghost"}
                                className="h-8 min-w-10 px-2 text-xs" onClick={() => setRange(item)} aria-pressed={range === item}>
                                {item}
                            </Button>
                        ))}
                    </div>
                </CardHeader>
                <CardContent className="p-2 sm:p-5">
                    {latencyLoading ? <Skeleton className="h-72 w-full" /> : chartData.length === 0 ? (
                        <div className="grid h-72 place-items-center text-sm text-muted-foreground">No response-time data for this range.</div>
                    ) : (
                        <div className="h-72 w-full" aria-label={`Response time over ${range}`}>
                            <ResponsiveContainer width="100%" height="100%">
                                <AreaChart data={chartData} margin={{ top: 12, right: 12, left: -12, bottom: 0 }}>
                                    <defs>
                                        <linearGradient id="monitorLatency" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
                                            <stop offset="100%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                                        </linearGradient>
                                    </defs>
                                    <CartesianGrid vertical={false} stroke="hsl(var(--border))" strokeDasharray="3 3" />
                                    <XAxis dataKey="timestampMs" type="number" scale="time" domain={["dataMin", "dataMax"]} tickFormatter={tick}
                                        stroke="hsl(var(--muted-foreground))" fontSize={10} minTickGap={42} tickLine={false} axisLine={false} />
                                    <YAxis width={48} tickFormatter={value => `${value}ms`} stroke="hsl(var(--muted-foreground))"
                                        fontSize={10} tickLine={false} axisLine={false} />
                                    <Tooltip labelFormatter={value => formatDate(value, timezone)} formatter={(value) => [`${value} ms`, "Latency"]}
                                        contentStyle={{ background: "hsl(var(--popover))", border: "1px solid hsl(var(--border))", borderRadius: "0.5rem" }} />
                                    <Area type="monotone" dataKey="latency" stroke="hsl(var(--primary))" strokeWidth={2}
                                        fill="url(#monitorLatency)" isAnimationActive={false} connectNulls={false} />
                                </AreaChart>
                            </ResponsiveContainer>
                        </div>
                    )}
                </CardContent>
            </Card>

            <Card className="border-border bg-card shadow-none">
                <CardHeader className="pb-3"><CardTitle className="text-base">Recent checks</CardTitle></CardHeader>
                <CardContent className="overflow-x-auto">
                    <UptimeHistory className="max-w-none" history={monitor.history} interval={monitor.interval} isPaused={monitor.status === "paused"} />
                </CardContent>
            </Card>

            <InsightsCard monitorId={monitor.id} />
        </div>
    );
}

function Metric({ label, value, detail, tone = "text-foreground", icon }: { label: string; value: string; detail?: string; tone?: string; icon: React.ReactElement }) {
    const card = (
        <Card className="border-border bg-card shadow-none" tabIndex={detail ? 0 : undefined}>
            <CardContent className="p-4 sm:p-5">
                <div className="mb-5 flex items-center justify-between text-muted-foreground">
                    <span className="text-xs font-medium">{label}</span>
                    <span className="[&>svg]:h-4 [&>svg]:w-4" aria-hidden="true">{icon}</span>
                </div>
                <div className={`text-xl font-semibold tabular-nums sm:text-2xl ${tone}`}>{value}</div>
            </CardContent>
        </Card>
    );
    if (!detail) return card;
    return (
        <UiTooltip>
            <TooltipTrigger asChild>{card}</TooltipTrigger>
            <TooltipContent side="top" className="max-w-72 text-xs">{detail}</TooltipContent>
        </UiTooltip>
    );
}
