import { Badge } from "@/components/ui/badge";
import { Monitor } from "@/lib/store";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, AlertTriangle, Pause } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useMemo } from "react";

export const StatusBadge = ({ status, isMaintenance }: { status: Monitor['status']; isMaintenance?: boolean }) => {
    if (isMaintenance) {
        return (
            <Badge variant="outline" className="border-blue-500/30 text-blue-500 gap-1 px-2 py-1 w-[105px] justify-center bg-blue-500/5">
                <AlertTriangle className="w-3 h-3" />
                Maintenance
            </Badge>
        );
    }
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
            <Badge variant="outline" className="border-rose-500/30 text-rose-500 gap-1 px-2 py-1 w-[105px] justify-center animate-pulse">
                <ArrowDown className="w-3 h-3" />
                Unavailable
            </Badge>
        );
    }
    if (status === 'paused') {
        return (
            <Badge variant="outline" className="border-slate-500/30 text-slate-400 gap-1 px-2 py-1 w-[105px] justify-center">
                <Pause className="w-3 h-3" />
                Paused
            </Badge>
        );
    }
    return (
        <Badge variant="outline" className="border-amber-500/30 text-amber-500 gap-1 px-2 py-1 w-[105px] justify-center">
            <AlertTriangle className="w-3 h-3" />
            Degraded
        </Badge>
    );
};

export interface DisplaySlot {
    time: Date;
    point: Monitor['history'][number] | null;
}

export function buildTimeSlots(
    history: Monitor['history'] | undefined,
    interval: number | undefined,
    now: number = Date.now()
): DisplaySlot[] {
    const LIMIT = 30;
    const step = (interval && interval > 0 ? interval : 60) * 1000; // ms
    const tolerance = step / 2;

    // Anchor to the latest history point so the rightmost bar always shows data.
    // Falls back to `now` only when there is no history at all.
    let anchor = now;
    if (history && history.length > 0) {
        const latestTs = Math.max(...history.map(h => new Date(h.timestamp).getTime()));
        // Anchor to latest point only if within 2 intervals of now (normal polling drift).
        // Beyond that, the gap is real (e.g. monitor was paused) — anchor to now to show it.
        if (now - latestTs <= 2 * step) {
            anchor = latestTs;
        }
    }

    // Build 30 time slots anchored to the latest check, going backward
    const slots: DisplaySlot[] = [];
    for (let i = 0; i < LIMIT; i++) {
        slots.push({
            time: new Date(anchor - (LIMIT - 1 - i) * step),
            point: null,
        });
    }

    if (!history || history.length === 0) {
        return slots;
    }

    // Sort history newest-first so that when multiple checks fall in the same slot,
    // the latest one wins (we assign on first match and skip duplicates)
    const sorted = [...history].sort(
        (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
    );

    for (const point of sorted) {
        const ts = new Date(point.timestamp).getTime();
        let bestIdx = -1;
        let bestDist = Infinity;
        for (let i = 0; i < LIMIT; i++) {
            if (slots[i].point !== null) continue; // already filled
            const dist = Math.abs(slots[i].time.getTime() - ts);
            if (dist < bestDist) {
                bestDist = dist;
                bestIdx = i;
            }
        }
        if (bestIdx !== -1 && bestDist <= tolerance) {
            slots[bestIdx].point = point;
        }
    }

    return slots;
}

export const UptimeHistory = ({ history, interval, isPaused }: { history: Monitor['history'], interval?: number, isPaused?: boolean }) => {
    const displaySlots = useMemo(() => {
        return buildTimeSlots(history, interval);
    }, [history, interval]);

    return (
        <div className="flex gap-1 h-6 items-end w-full max-w-[500px]">
            {displaySlots.map((slot, i) => (
                <Tooltip key={i}>
                    <TooltipTrigger asChild>
                        <div
                            className={cn(
                                "flex-1 rounded-sm transition-all duration-300 min-w-[6px] cursor-pointer",
                                slot.point === null && !isPaused && "bg-slate-800/30 h-full hover:bg-slate-800/50",
                                slot.point === null && isPaused && "bg-slate-600/40 h-full hover:bg-slate-500/50",
                                slot.point?.status === 'up' && "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.3)] h-full hover:bg-emerald-400 hover:shadow-[0_0_12px_rgba(16,185,129,0.6)] hover:scale-y-105",
                                slot.point?.status === 'degraded' && "bg-amber-500 shadow-[0_0_8px_rgba(245,158,11,0.3)] h-full hover:bg-amber-400 hover:scale-y-105",
                                slot.point?.status === 'down' && "bg-rose-500 shadow-[0_0_8px_rgba(244,63,94,0.3)] h-full hover:bg-rose-400 hover:scale-y-105",
                            )}
                        />
                    </TooltipTrigger>
                    {slot.point ? (
                        <TooltipContent className="text-xs">
                            <div className="font-semibold mb-1">
                                {new Date(slot.point.timestamp).toLocaleTimeString()}
                            </div>
                            <div className="flex items-center gap-2">
                                <span className={cn(
                                    "w-2 h-2 rounded-full",
                                    slot.point.status === 'up' ? "bg-green-500" :
                                        slot.point.status === 'down' ? "bg-red-500" : "bg-yellow-500"
                                )} />
                                <span>
                                    {slot.point.status === 'up' ? 'Operational' :
                                        slot.point.status === 'down' ? 'Unavailable' : 'Degraded'}
                                </span>
                            </div>
                            <div className="mt-1 opacity-70">
                                {slot.point.statusCode ? `Status: ${slot.point.statusCode}` : 'Status: Unknown'}
                            </div>
                            <div className="opacity-70">
                                Latency: {slot.point.latency}ms
                            </div>
                        </TooltipContent>
                    ) : (
                        <TooltipContent className="text-xs">
                            <div className="font-semibold text-muted-foreground">{isPaused ? "Paused" : "No Data"}</div>
                            <div className="opacity-70">{slot.time.toLocaleTimeString()}</div>
                        </TooltipContent>
                    )}
                </Tooltip>
            ))}
        </div>
    );
};
