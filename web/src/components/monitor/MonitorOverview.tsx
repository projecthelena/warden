/* Hallmark · genre: modern-minimal · macrostructure: Workbench · design-system: design.md · designed-as-app */
import { useEffect, useMemo, useState } from "react";
import { Area, AreaChart, CartesianGrid, ReferenceArea, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";
import { useReducedMotion } from "framer-motion";
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
import {
    averageSuccessfulLatency,
    bucketDuration,
    chartDomain,
    toChartData,
    type LatencyChartPoint,
    type LatencyRange,
    type RenderedLatencyPoint,
} from "./latencyChart";

type UptimeStats = {
    windows: { last24Hours: UptimeSummary; last7Days: UptimeSummary; last30Days: UptimeSummary };
};

const ranges: LatencyRange[] = ["1h", "24h", "7d", "30d"];

export function MonitorOverview({ monitor, timezone }: { monitor: Monitor; timezone?: string }) {
    const [range, setRange] = useState<LatencyRange>("24h");
    const [stats, setStats] = useState<UptimeStats | null>(null);
    const [latency, setLatency] = useState<LatencyChartPoint[]>([]);
    const [statsLoading, setStatsLoading] = useState(true);
    const [latencyLoading, setLatencyLoading] = useState(true);
    const reduceMotion = useReducedMotion();

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

    const chartData = useMemo(() => toChartData(latency), [latency]);

    const averageLatency = useMemo(() => averageSuccessfulLatency(latency), [latency]);
    const domain = useMemo(() => chartDomain(chartData, range), [chartData, range]);
    const step = bucketDuration(range);

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
                        <div className="h-72 w-full" role="img" aria-label={`Response time over ${range}. Missing data is blank; failed checks are marked in red and mixed buckets in amber.`}
                            data-testid="response-time-chart" data-motion={reduceMotion ? "reduced" : "animated"}>
                            <ResponsiveContainer width="100%" height="100%">
                                <AreaChart data={chartData} margin={{ top: 12, right: 12, left: -12, bottom: 0 }}>
                                    <defs>
                                        <linearGradient id="monitorLatency" x1="0" y1="0" x2="0" y2="1">
                                            <stop offset="0%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
                                            <stop offset="100%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                                        </linearGradient>
                                        <pattern id="monitorNoData" width="6" height="6" patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
                                            <line x1="0" y1="0" x2="0" y2="6" stroke="hsl(var(--muted-foreground))" strokeOpacity="0.16" strokeWidth="2" />
                                        </pattern>
                                    </defs>
                                    <CartesianGrid vertical={false} stroke="hsl(var(--border))" strokeDasharray="3 3" />
                                    {chartData.map(point => point.state === "up" ? null : (
                                        <ReferenceArea key={`${point.timestamp}-${point.state}`} x1={point.timestampMs} x2={point.timestampMs + step}
                                            fill={point.state === "down" ? "hsl(var(--destructive))" : point.state === "mixed" ? "hsl(var(--chart-3))" : "url(#monitorNoData)"}
                                            fillOpacity={point.state === "no_data" ? 1 : 0.14} strokeOpacity={0} />
                                    ))}
                                    <XAxis dataKey="timestampMs" type="number" scale="time" domain={domain} tickFormatter={tick}
                                        stroke="hsl(var(--muted-foreground))" fontSize={10} minTickGap={42} tickLine={false} axisLine={false} />
                                    <YAxis width={48} tickFormatter={value => `${value}ms`} stroke="hsl(var(--muted-foreground))"
                                        fontSize={10} tickLine={false} axisLine={false} />
                                    <Tooltip content={<LatencyTooltip timezone={timezone} />} filterNull={false} />
                                    <Area type="monotone" dataKey="latency" stroke="hsl(var(--primary))" strokeWidth={2}
                                        fill="url(#monitorLatency)" connectNulls={false} isAnimationActive={!reduceMotion}
                                        animationDuration={500} animationEasing="ease-out" />
                                </AreaChart>
                            </ResponsiveContainer>
                        </div>
                    )}
                    <div className="mt-3 flex flex-wrap gap-x-4 gap-y-2 text-xs text-muted-foreground" aria-label="Chart legend">
                        <ChartKey className="bg-primary" label="Successful response" />
                        <ChartKey className="bg-destructive" label="Failed checks" />
                        <ChartKey className="bg-[hsl(var(--chart-3))]" label="Mixed result" />
                        <ChartKey className="border border-muted-foreground/30 bg-muted/30" label="No data" />
                    </div>
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

function ChartKey({ className, label }: { className: string; label: string }) {
    return <span className="inline-flex items-center gap-1.5"><span className={`h-2 w-2 rounded-sm ${className}`} aria-hidden="true" />{label}</span>;
}

function LatencyTooltip({ active, payload, timezone }: {
    active?: boolean;
    payload?: Array<{ payload: RenderedLatencyPoint }>;
    timezone?: string;
}) {
    const point = payload?.[0]?.payload;
    if (!active || !point) return null;
    const status = point.state === "no_data" ? "No data collected"
        : point.state === "down" ? "All checks failed"
            : point.state === "mixed" ? "Some checks failed"
                : "All checks succeeded";

    return (
        <div className="rounded-md border border-border bg-popover px-3 py-2 text-xs text-popover-foreground shadow-lg">
            <div className="font-medium">{formatDate(point.timestampMs, timezone)}</div>
            <div className="mt-1 text-muted-foreground">{status}</div>
            {point.latency != null && <div className="mt-1 tabular-nums">Successful responses: {point.latency} ms average</div>}
            {point.totalChecks > 0 && <div className="mt-1 tabular-nums text-muted-foreground">{point.successfulChecks} succeeded · {point.failedChecks} failed</div>}
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
