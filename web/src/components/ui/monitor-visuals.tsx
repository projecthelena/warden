import { Badge } from "@/components/ui/badge";
import { Monitor } from "@/lib/store";
import { cn } from "@/lib/utils";
import { ArrowUp, ArrowDown, AlertTriangle } from "lucide-react";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";

export const StatusBadge = ({ status }: { status: Monitor['status'] }) => {
    if (status === 'up') {
        return (
            <Badge variant="outline" className="border-green-800 bg-green-950/50 text-green-400 gap-1 px-2 py-1 w-[105px] justify-center">
                <ArrowUp className="w-3 h-3" />
                Operational
            </Badge>
        );
    }
    if (status === 'down') {
        return (
            <Badge variant="destructive" className="bg-red-950/50 border-red-800 text-red-500 gap-1 px-2 py-1 animate-pulse w-[105px] justify-center">
                <ArrowDown className="w-3 h-3" />
                Downtime
            </Badge>
        );
    }
    return (
        <Badge variant="secondary" className="bg-yellow-950/50 border-yellow-800 text-yellow-500 gap-1 px-2 py-1 w-[105px] justify-center">
            <AlertTriangle className="w-3 h-3" />
            Degraded
        </Badge>
    );
};

export const UptimeHistory = ({ history }: { history: Monitor['history'] }) => {
    const safeHistory = history || [];
    const LIMIT = 30;
    const emptyCount = Math.max(0, LIMIT - safeHistory.length);
    const emptySlots = Array(emptyCount).fill(null);
    const displaySlots = [...emptySlots, ...safeHistory].slice(-LIMIT);

    return (
        <TooltipProvider delayDuration={0}>
            <div className="flex gap-1 h-6 items-end w-full max-w-[360px]" title="Last 30 checks">
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
                        {slot && (
                            <TooltipContent className="text-xs bg-slate-900 border-slate-800 text-slate-200">
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
                        )}
                    </Tooltip>
                ))}
            </div>
        </TooltipProvider>
    );
};
