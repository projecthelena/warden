import { Badge } from "@/components/ui/badge";
import { Monitor, HistoryPoint } from "@/lib/store";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, AlertTriangle } from "lucide-react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { useMemo } from "react";

export const StatusBadge = ({ status }: { status: Monitor['status'] }) => {
    if (status === 'up') {
        return (
            <Badge variant="outline" className="border-emerald-500/30 text-emerald-500 gap-1 px-2 py-1 w-[105px] justify-center">
                <ArrowUp className="w-3 h-3" />
                Operational
            </Badge>
        );
    }
    if (status === 'down') {
        return (
            <Badge variant="destructive" className="gap-1 px-2 py-1 w-[105px] justify-center animate-pulse">
                <ArrowDown className="w-3 h-3" />
                Downtime
            </Badge>
        );
    }
    return (
        <Badge variant="secondary" className="gap-1 px-2 py-1 w-[105px] justify-center">
            <AlertTriangle className="w-3 h-3" />
            Degraded
        </Badge>
    );
};

export const UptimeHistory = ({ history, interval = 60 }: { history: Monitor['history'], interval?: number }) => {
    const LIMIT = 30;
    const now = Date.now();
    const safeInterval = interval || 60; // Default to 60s if 0 or undefined
    const checkWindow = safeInterval * 1000; // Interval in ms
    // Tolerance for matching a check to a slot (e.g. +/- 50% of interval)
    // We actually want to find if ANY check occurred in the slot window.
    // The slot window i represents time range: [now - (i+1)*checkWindow, now - i*checkWindow]

    const displaySlots = useMemo(() => {
        const slots: (HistoryPoint | null)[] = [];
        // We want 30 slots, from oldest (index 0) to newest (index 29)
        // Wait, standard visualization: right is newest.
        // So displaySlots[29] is "now". displaySlots[0] is "limit intervals ago".

        for (let i = LIMIT - 1; i >= 0; i--) {
            // Calculate start and end of this time bucket
            // i=0 corresponds to "now" (most recent bucket) in loop logic? 
            // Let's use i as "number of intervals ago".
            // i=0 => [now - interval, now]
            // i=29 => [now - 30*interval, now - 29*interval]

            const endTime = now - (i * checkWindow);
            const startTime = endTime - checkWindow;

            // Find a history point in this range
            // History is ordered. We can search or filter.
            // Since N is small, filter is fine.
            const match = history?.find(h => {
                const t = new Date(h.timestamp).getTime();
                return t >= startTime && t <= endTime;
            });

            slots.push(match || null);
        }
        // The loop above pushes "most recent (i=0)" LAST if i goes from LIMIT-1 down to 0?
        // Wait:
        // i=29 (start of loop): endTime = now - 29*window. This is oldest.
        // i=0 (end of loop): endTime = now. This is newest.
        // So slots will be [oldest, ..., newest]. Correct.
        return slots;
    }, [history, now, checkWindow]);

    return (
        <TooltipProvider delayDuration={0}>
            <div className="flex gap-1 h-6 items-end w-full max-w-[500px]">
                {displaySlots.map((slot, i) => (
                    <Tooltip key={i}>
                        <TooltipTrigger asChild>
                            <div
                                className={cn(
                                    "flex-1 rounded-sm transition-all duration-300 min-w-[6px] cursor-pointer",
                                    slot === null && "bg-slate-800/30 h-full hover:bg-slate-800/50", // Empty slot
                                    slot?.status === 'up' && "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.3)] h-full hover:bg-emerald-400 hover:shadow-[0_0_12px_rgba(16,185,129,0.6)] hover:scale-y-105",
                                    slot?.status === 'degraded' && "bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.3)] h-full hover:bg-amber-400 hover:scale-y-105",
                                    slot?.status === 'down' && "bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.3)] h-full hover:bg-rose-400 hover:scale-y-105",
                                )}
                            />
                        </TooltipTrigger>
                        {slot ? (
                            <TooltipContent className="text-xs">
                                <div className="font-semibold mb-1">
                                    {new Date(slot.timestamp).toLocaleTimeString()}
                                </div>
                                <div className="flex items-center gap-2">
                                    <span className={cn(
                                        "w-2 h-2 rounded-full",
                                        slot.status === 'up' ? "bg-green-500" :
                                            slot.status === 'down' ? "bg-red-500" : "bg-yellow-500"
                                    )} />
                                    <span>
                                        {slot.status === 'up' ? 'Operational' :
                                            slot.status === 'down' ? 'Downtime' : 'Degraded'}
                                    </span>
                                </div>
                                <div className="mt-1 opacity-70">
                                    {slot.statusCode ? `Status: ${slot.statusCode}` : 'Status: Unknown'}
                                </div>
                                <div className="opacity-70">
                                    Latency: {slot.latency}ms
                                </div>
                            </TooltipContent>
                        ) : (
                            <TooltipContent className="text-xs">
                                <div className="font-semibold text-muted-foreground">No Data</div>
                            </TooltipContent>
                        )}
                    </Tooltip>
                ))}
            </div>
        </TooltipProvider>
    );
};
