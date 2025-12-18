import { useState, useEffect, useCallback } from "react";
import { Monitor, useMonitorStore } from "@/lib/store";
import { formatDate } from "@/lib/utils";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from "@/components/ui/sheet";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/monitor-visuals";
import { Trash2, Save, Activity, Clock, BarChart } from "lucide-react";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { AreaChart, Area, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, ReferenceArea } from 'recharts';

interface MonitorDetailsSheetProps {
    monitor: Monitor;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function MonitorDetailsSheet({ monitor, open, onOpenChange }: MonitorDetailsSheetProps) {
    const { updateMonitor, deleteMonitor, user } = useMonitorStore();
    const [name, setName] = useState(monitor.name);
    const [url, setUrl] = useState(monitor.url);
    const [interval, setInterval] = useState(monitor.interval || 60);
    const [stats, setStats] = useState({ uptime24h: 100, uptime7d: 100, uptime30d: 100 });
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const [latencyData, setLatencyData] = useState<any[]>([]);
    const [timeRange, setTimeRange] = useState("1h");

    useEffect(() => {
        if (open) {
            setName(monitor.name);
            setUrl(monitor.url);
            setInterval(monitor.interval || 60);
        }
    }, [open, monitor]);

    const fetchLatency = useCallback((id: string, range: string) => {
        fetch(`/api/monitors/${id}/latency?range=${range}`)
            .then(res => res.json())
            .then(data => {
                if (data) {
                    const sortedData = data
                        // eslint-disable-next-line @typescript-eslint/no-explicit-any
                        .map((point: any) => ({
                            ...point,
                            timestamp: new Date(point.timestamp).getTime()
                        }))
                        // eslint-disable-next-line @typescript-eslint/no-explicit-any
                        .sort((a: any, b: any) => a.timestamp - b.timestamp);

                    const filledData = processChartData(sortedData, range);
                    setLatencyData(filledData);
                }
            })
            .catch(err => console.error("Failed to fetch latency", err));
    }, []);

    useEffect(() => {
        if (open && monitor.id) {
            // Fetch Uptime Stats
            fetch(`/api/monitors/${monitor.id}/uptime`)
                .then(res => res.json())
                .then(data => setStats(data))
                .catch(err => console.error("Failed to fetch stats", err));

            // Fetch Latency Data
            fetchLatency(monitor.id, timeRange);
        }
    }, [open, monitor.id, timeRange, fetchLatency]);

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const processChartData = (data: any[], range: string) => {
        const now = Date.now();
        let step = 60 * 60 * 1000;
        let start = now;

        // Determine step size and start time
        if (range === "1h") {
            step = 60 * 1000; // 1 minute
            start = now - (60 * 60 * 1000);
        }

        const filled = [];
        let current = start;

        // Create a map for quick lookup of data points within a time window
        // Optimization: Since data is sorted, we could do this in a single pass,
        // but a filter per bucket is easier to read and performance is fine for < 2000 points.
        // Let's do a slightly optimized single pass.

        let dataIndex = 0;

        while (current <= now) {
            const bucketEnd = current + step;
            const bucketPoints = [];

            // Collect all points belonging to this bucket [current, current + step)
            // We assume data is sorted by timestamp asc
            while (dataIndex < data.length) {
                const p = data[dataIndex];
                if (p.timestamp < current) {
                    // Skip old data (shouldn't happen if sorted and start is correct, but safety)
                    dataIndex++;
                    continue;
                }
                if (p.timestamp >= bucketEnd) {
                    // Point belongs to next bucket
                    break;
                }
                bucketPoints.push(p);
                dataIndex++;
            }

            if (bucketPoints.length > 0) {
                // Calculate average latency
                const validPoints = bucketPoints.filter(p => !p.failed && p.latency !== null);

                // Determine status for the bucket
                // If any point failed, does the whole bucket fail? 
                // Or do we visualize it as "some failures"? 
                // User wants "average". 
                // Let's say if > 50% failed, mark as failed? Or if ANY failed?
                // Visualizing specific failures is handled by red bars (reference areas).
                // For the line, we just want the average latency of successful checks.

                if (validPoints.length > 0) {
                    const totalLatency = validPoints.reduce((sum, p) => sum + p.latency, 0);
                    const avgLatency = Math.round(totalLatency / validPoints.length);
                    filled.push({ timestamp: current, latency: avgLatency });
                } else {
                    // All points in this bucket were failures
                    filled.push({ timestamp: current, latency: null, failed: true });
                }
            } else {
                // No data in this bucket -> Gap
                filled.push({ timestamp: current, latency: null });
            }

            current += step;
        }

        return filled;
    }

    // Identify continuous failure zones for the red bars
    // Updated to work with processed data? 
    // Actually, distinct failures might be lost in aggregation if we only look at the averaged bucket.
    // Ideally, we keep the raw data for failure zones, OR we compute failure zones from raw data before aggregation.
    // The current implementation uses 'latencyData' which IS the processed data.
    // If a bucket has 'failed: true', it counts as a failure zone.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const getFailureZones = (data: any[]) => {
        const zones = [];
        let zoneStart = null;

        for (let i = 0; i < data.length; i++) {
            const point = data[i];
            // Check for explicit failure flag from aggregation
            if (point.failed) {
                if (zoneStart === null) {
                    zoneStart = point.timestamp;
                }
            } else {
                if (zoneStart !== null) {
                    zones.push({ start: zoneStart, end: data[i - 1].timestamp });
                    zoneStart = null;
                }
            }
        }
        if (zoneStart !== null) {
            zones.push({ start: zoneStart, end: data[data.length - 1].timestamp });
        }
        return zones;
    };


    const handleSave = () => {
        updateMonitor(monitor.id, { name, url, interval });
        onOpenChange(false);
    };

    const formatUptime = (val: number) => {
        if (val === 100) return "100%";
        return val.toFixed(2) + "%";
    }

    const formatXAxis = (tickItem: string | number) => {
        const date = new Date(tickItem);
        const tz = user?.timezone || 'UTC';

        if (timeRange === "1h" || timeRange === "24h") {
            return new Intl.DateTimeFormat("en-US", {
                hour: '2-digit',
                minute: '2-digit',
                timeZone: tz
            }).format(date);
        }
        return new Intl.DateTimeFormat("en-US", {
            month: 'short',
            day: 'numeric',
            timeZone: tz
        }).format(date);
    }

    const getUptimeColor = (val: number) => {
        if (val >= 98) return "text-emerald-400";
        if (val >= 90) return "text-amber-400";
        return "text-rose-400"; // Red for < 90%
    }

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="sm:max-w-[600px] overflow-y-auto">
                <SheetHeader className="mb-6">
                    <div className="flex items-center justify-between">
                        <SheetTitle>{monitor.name}</SheetTitle>
                        <StatusBadge status={monitor.status} />
                    </div>
                    <SheetDescription className="font-mono text-xs">
                        ID: {monitor.id}
                    </SheetDescription>
                </SheetHeader>

                <Tabs defaultValue="metrics" className="w-full">
                    <TabsList className="w-full grid grid-cols-3">
                        <TabsTrigger value="metrics">Metrics</TabsTrigger>
                        <TabsTrigger value="events">Events</TabsTrigger>
                        <TabsTrigger value="settings">Settings</TabsTrigger>
                    </TabsList>

                    <TabsContent value="metrics" className="mt-6 space-y-6">
                        {/* Uptime Cards */}
                        <div className="grid grid-cols-3 gap-2">
                            <div className="bg-card border border-border rounded-lg p-3 text-center shadow-sm">
                                <span className="text-xs text-muted-foreground block mb-1">24h Uptime</span>
                                <span className={`text-lg font-semibold ${getUptimeColor(stats.uptime24h)}`}>{formatUptime(stats.uptime24h)}</span>
                            </div>
                            <div className="bg-card border border-border rounded-lg p-3 text-center shadow-sm">
                                <span className="text-xs text-muted-foreground block mb-1">7d Uptime</span>
                                <span className={`text-lg font-semibold ${getUptimeColor(stats.uptime7d)}`}>{formatUptime(stats.uptime7d)}</span>
                            </div>
                            <div className="bg-card border border-border rounded-lg p-3 text-center shadow-sm">
                                <span className="text-xs text-muted-foreground block mb-1">30d Uptime</span>
                                <span className={`text-lg font-semibold ${getUptimeColor(stats.uptime30d)}`}>{formatUptime(stats.uptime30d)}</span>
                            </div>
                        </div>

                        {/* Latency Chart */}
                        <div className="space-y-4">
                            <div className="flex items-center justify-between">
                                <h3 className="text-sm font-medium text-foreground flex items-center gap-2">
                                    <BarChart className="w-4 h-4 text-muted-foreground" /> Response Time
                                </h3>
                                <div className="flex bg-muted rounded-lg p-0.5 border border-border">
                                    {["1h"].map((r) => (
                                        <button
                                            key={r}
                                            onClick={() => setTimeRange(r)}
                                            className={`px-3 py-1 text-xs rounded-md transition-all ${timeRange === r ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
                                                }`}
                                        >
                                            {r}
                                        </button>
                                    ))}
                                </div>
                            </div>

                            <div className="h-[250px] w-full bg-card border border-border rounded-lg p-4 shadow-sm">
                                <ResponsiveContainer width="100%" height="100%">
                                    <AreaChart data={latencyData}>
                                        <defs>
                                            <linearGradient id="colorLatency" x1="0" y1="0" x2="0" y2="1">
                                                <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.3} />
                                                <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                                            </linearGradient>
                                        </defs>
                                        <CartesianGrid strokeDasharray="3 3" stroke="hsl(var(--border))" vertical={false} />
                                        <XAxis
                                            dataKey="timestamp"
                                            stroke="hsl(var(--muted-foreground))"
                                            fontSize={10}
                                            tickFormatter={formatXAxis}
                                            minTickGap={30}
                                            type="number"
                                            scale="time"
                                            domain={[
                                                (dataMin: number) => {
                                                    const now = Date.now();
                                                    if (timeRange === "1h") return now - 60 * 60 * 1000;
                                                    return dataMin;
                                                },
                                                (_dataMax: number) => Date.now()
                                            ]}
                                        />
                                        <YAxis
                                            stroke="hsl(var(--muted-foreground))"
                                            fontSize={10}
                                            tickFormatter={(val) => `${val}ms`}
                                            width={50}
                                            domain={[0, (dataMax: number) => Math.ceil(dataMax * 1.1)]}
                                        />
                                        <Tooltip
                                            contentStyle={{
                                                backgroundColor: 'hsl(var(--popover))',
                                                borderColor: 'hsl(var(--border))',
                                                borderRadius: 'var(--radius)',
                                                color: 'hsl(var(--popover-foreground))'
                                            }}
                                            itemStyle={{ color: 'hsl(var(--foreground))', fontSize: '12px' }}
                                            labelStyle={{ color: 'hsl(var(--muted-foreground))', fontSize: '11px', marginBottom: '4px' }}
                                            labelFormatter={(label) => {
                                                return formatDate(label, user?.timezone);
                                            }}
                                            // eslint-disable-next-line @typescript-eslint/no-explicit-any
                                            formatter={(value: any, name: any, props: any) => {
                                                if (props.payload.failed) return ['Failed', 'Status'];
                                                return [value != null ? `${value}ms` : 'No Data', 'Latency'];
                                            }}
                                        />

                                        {/* Failure Zones */}
                                        {/* eslint-disable-next-line @typescript-eslint/no-explicit-any */}
                                        {getFailureZones(latencyData).map((zone: any, i: number) => (
                                            <ReferenceArea
                                                key={i}
                                                x1={zone.start}
                                                x2={zone.end}
                                                fill="hsl(var(--destructive))"
                                                fillOpacity={0.1}
                                            />
                                        ))}

                                        <Area
                                            type="monotone"
                                            dataKey="latency"
                                            stroke="hsl(var(--primary))"
                                            strokeWidth={2}
                                            fillOpacity={1}
                                            fill="url(#colorLatency)"
                                            isAnimationActive={false}
                                            connectNulls={timeRange === '1h'}
                                        />
                                    </AreaChart>
                                </ResponsiveContainer>
                            </div>
                        </div>
                    </TabsContent>

                    <TabsContent value="events" className="mt-6 space-y-4">
                        <h3 className="text-sm font-medium text-foreground mb-4 flex items-center gap-2">
                            <Activity className="w-4 h-4 text-muted-foreground" /> Activity Log
                        </h3>
                        {monitor.events && monitor.events.length > 0 ? (
                            <div className="relative border-l border-border ml-2 space-y-6">
                                {monitor.events.map((event) => (
                                    <div key={event.id} className="ml-6 relative">
                                        <div className={`absolute -left-[31px] top-1 w-2.5 h-2.5 rounded-full ring-4 ring-background ${event.type === 'up' ? 'bg-emerald-500' :
                                            event.type === 'down' ? 'bg-destructive' : 'bg-yellow-500'
                                            }`} />
                                        <div className="flex flex-col gap-1">

                                            <span className="text-xs text-muted-foreground flex items-center gap-1">
                                                <Clock className="w-3 h-3" />
                                                {formatDate(event.timestamp, user?.timezone)}
                                            </span>
                                            <p className="text-sm text-foreground">{event.message}</p>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <div className="text-center py-12 text-muted-foreground text-sm border border-dashed border-border rounded-lg">
                                No events recorded yet.
                            </div>
                        )}
                    </TabsContent>

                    <TabsContent value="settings" className="mt-6 space-y-6">
                        <div className="space-y-4">
                            <div className="grid gap-2">
                                <Label>Display Name</Label>
                                <Input value={name} onChange={e => setName(e.target.value)} />
                            </div>
                            <div className="grid gap-2">
                                <Label>Target URL</Label>
                                <Input value={url} onChange={e => setUrl(e.target.value)} className="font-mono text-xs" />
                            </div>
                            <div className="grid gap-2">
                                <Label>Check Frequency</Label>
                                <Select onValueChange={(v) => setInterval(Number(v))} value={interval.toString()}>
                                    <SelectTrigger>
                                        <SelectValue placeholder="Select frequency" />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="10" className="cursor-pointer">10 Seconds</SelectItem>
                                        <SelectItem value="30" className="cursor-pointer">30 Seconds</SelectItem>
                                        <SelectItem value="60" className="cursor-pointer">1 Minute</SelectItem>
                                        <SelectItem value="300" className="cursor-pointer">5 Minutes</SelectItem>
                                        <SelectItem value="600" className="cursor-pointer">10 Minutes</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                            <Button onClick={handleSave} className="w-full">
                                <Save className="w-4 h-4 mr-2" /> Save Changes
                            </Button>
                        </div>

                        <div className="pt-6 border-t border-border">
                            <h3 className="text-sm font-medium text-destructive mb-2">Danger Zone</h3>
                            <p className="text-xs text-muted-foreground mb-4">
                                Deleting this monitor is irreversible. All history will be lost.
                            </p>
                            <AlertDialog>
                                <AlertDialogTrigger asChild>
                                    <Button variant="destructive" className="w-full">
                                        <Trash2 className="w-4 h-4 mr-2" /> Delete Monitor
                                    </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent>
                                    <AlertDialogHeader>
                                        <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                                        <AlertDialogDescription>
                                            This action cannot be undone. This will permanently delete the monitor
                                            <strong> {monitor.name} </strong> and remove all its data.
                                        </AlertDialogDescription>
                                    </AlertDialogHeader>
                                    <AlertDialogFooter>
                                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                                        <AlertDialogAction
                                            onClick={() => {
                                                deleteMonitor(monitor.id);
                                                onOpenChange(false);
                                            }}
                                            className="bg-destructive hover:bg-destructive/90 text-destructive-foreground border-none"
                                        >
                                            Delete Monitor
                                        </AlertDialogAction>
                                    </AlertDialogFooter>
                                </AlertDialogContent>
                            </AlertDialog>
                        </div>
                    </TabsContent>

                </Tabs>
            </SheetContent>
        </Sheet>
    )
}
