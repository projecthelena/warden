import { useState, useMemo } from "react";
import { useMonitorStore, Monitor } from "@/lib/store";
import { formatDate } from "@/lib/utils";
import { StatusBadge, UptimeHistory } from "@/components/ui/monitor-visuals";
import { MonitorDetailsSheet } from "@/components/MonitorDetailsSheet";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export function MonitorCard({ monitor, groupId }: { monitor: Monitor, groupId: string }) {
    const [detailsOpen, setDetailsOpen] = useState(false);
    const { user, incidents } = useMonitorStore();

    const formattedFullDate = formatDate(monitor.lastCheck, user?.timezone);

    // Check for active maintenance
    const isMaintenance = useMemo(() => {
        return incidents.some(i =>
            i.type === 'maintenance' &&
            i.status !== 'completed' &&
            i.status !== 'resolved' &&
            i.affectedGroups.includes(groupId)
        );
    }, [incidents, groupId]);

    // Format just the time (e.g. 9:41 PM)
    const timeOnly = useMemo(() => {
        if (!monitor.lastCheck || monitor.lastCheck === 'Never') return monitor.lastCheck || '';
        try {
            return new Intl.DateTimeFormat('en-US', {
                hour: 'numeric',
                minute: '2-digit',
                timeZone: user?.timezone,
            }).format(new Date(monitor.lastCheck));
        } catch (e) {
            return monitor.lastCheck;
        }
    }, [monitor.lastCheck, user?.timezone]);


    return (
        <>
            <div
                onClick={() => setDetailsOpen(true)}
                className={cn(
                    "flex flex-col sm:flex-row items-center justify-between p-4 border border-border rounded-lg bg-card hover:bg-accent/50 transition-all gap-4 cursor-pointer group w-full",
                    isMaintenance && "border-blue-500/20 bg-blue-500/5 hover:bg-blue-500/10"
                )}
            >
                <div className="space-y-1 flex-1 min-w-0 mr-4">
                    <div className="flex items-center gap-2.5">
                        <span className="font-medium text-sm group-hover:text-primary transition-colors truncate block" title={monitor.name}>{monitor.name}</span>
                    </div>
                    <div className="text-xs text-muted-foreground font-mono truncate block opacity-60" title={monitor.url}>{monitor.url}</div>
                </div>

                <div className="flex-none hidden sm:block">
                    <UptimeHistory history={monitor.history} interval={monitor.interval} />
                </div>

                <div className="flex items-center gap-3 w-[160px] justify-end shrink-0">
                    <div className="text-right whitespace-nowrap">
                        <div className="text-xs font-mono text-muted-foreground">{monitor.latency}ms</div>
                        <Tooltip>
                            <TooltipTrigger asChild>
                                <div className="text-[10px] text-muted-foreground opacity-50 hover:opacity-100 cursor-help transition-opacity">
                                    {timeOnly}
                                </div>
                            </TooltipTrigger>
                            <TooltipContent className="text-xs">
                                <p>{formattedFullDate}</p>
                            </TooltipContent>
                        </Tooltip>
                    </div>
                    <StatusBadge status={monitor.status} isMaintenance={isMaintenance} />
                </div>
            </div>
            <MonitorDetailsSheet monitor={monitor} open={detailsOpen} onOpenChange={setDetailsOpen} />
        </>
    )
}
