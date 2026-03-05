import { Badge } from "@/components/ui/badge";
import { Monitor } from "@/lib/store";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, AlertTriangle, Pause } from "lucide-react";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { useMemo } from "react";
import { buildTimeSlots } from "./monitor-visuals-utils";

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
            <Badge variant="outline" className="border-muted-foreground/30 text-muted-foreground gap-1 px-2 py-1 w-[105px] justify-center">
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
                                slot.point === null && !isPaused && "bg-muted-foreground/20 h-full hover:bg-muted-foreground/30",
                                slot.point === null && isPaused && "bg-muted-foreground/15 h-full hover:bg-muted-foreground/25",
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
