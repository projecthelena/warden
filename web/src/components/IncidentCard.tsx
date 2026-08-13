import { useState } from "react";
import { Link } from "react-router-dom";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ChevronDown, ChevronRight, AlertTriangle, ArrowDown, Lock, Clock, Activity } from "lucide-react";
import { formatDate } from "@/lib/utils";
import { useMonitorStore } from "@/lib/store";
import { useIncidentEvents } from "@/hooks/useMonitorEvents";
import { EventDetailCard } from "@/components/EventDetailCard";
import { EventGroupCard } from "@/components/EventGroupCard";
import { groupConsecutiveEvents } from "@/lib/eventGroups";

// IncidentCard renders one "what went wrong, for how long" rollup row. The same component
// drives /incidents (with monitor + group meta and a link through to the monitor) and
// /monitors/:id, where both of those would point at the page you are already on and are
// dropped via onMonitorPage. Click → expand → lazy-fetch the enriched events that fell
// inside the outage window.

export interface IncidentCardProps {
    monitorId: string;
    monitorName: string;
    groupName?: string;
    /**
     * True when the card is already rendered on that monitor's own page. Drops the bits that
     * would just point at the current page: the monitor/group meta line (the breadcrumb
     * already says it) and the "Open monitor page" link (it would link to itself).
     */
    onMonitorPage?: boolean;
    type: "down" | "degraded" | "ssl_expiring";
    summary: string;
    startedAt: string;
    /** Undefined / null => still ongoing. */
    endedAt?: string | null;
    /** Pre-formatted duration string from the backend ("1h 39m", "0m"). */
    duration: string;
    /** Render adornment to the right of the meta (e.g. Promote button). */
    rightAction?: React.ReactNode;
}

const TYPE_META: Record<string, { label: string; color: string; Icon: typeof ArrowDown }> = {
    down: { label: "Down", color: "text-rose-500 border-rose-500/30 bg-rose-500/5", Icon: ArrowDown },
    degraded: { label: "Degraded", color: "text-amber-500 border-amber-500/30 bg-amber-500/5", Icon: AlertTriangle },
    ssl_expiring: { label: "SSL Expiring", color: "text-orange-500 border-orange-500/30 bg-orange-500/5", Icon: Lock },
};

export function IncidentCard({
    monitorId,
    monitorName,
    groupName,
    onMonitorPage,
    type,
    summary,
    startedAt,
    endedAt,
    duration,
    rightAction,
}: IncidentCardProps) {
    const [open, setOpen] = useState(false);
    const { user } = useMonitorStore();
    const meta = TYPE_META[type] ?? { label: type, color: "text-muted-foreground border-border bg-muted/30", Icon: AlertTriangle };
    const Icon = meta.Icon;
    // SSL expiring events aren't rollups of monitor_events with bodies/headers — they're
    // single notifications. We render them with the same chrome but skip the expand drawer
    // AND skip the "ongoing" indicator (the cert isn't actively failing right now).
    const isSSL = type === "ssl_expiring";
    const expandable = !isSSL;
    const isOngoing = !endedAt && !isSSL;

    // The hook handles ongoing vs resolved internally — for ongoing outages it polls
    // periodically and computes `to` at fetch time, instead of recomputing it on every
    // render (which would churn the query key and cause an infinite refetch loop).
    const eventsQ = useIncidentEvents(monitorId, startedAt, endedAt, open && expandable);

    return (
        <Collapsible open={open} onOpenChange={expandable ? setOpen : undefined} className="border border-border rounded-lg bg-card overflow-hidden">
            <CollapsibleTrigger
                disabled={!expandable}
                className={`w-full text-left px-3 py-2.5 flex items-center gap-3 ${expandable ? "hover:bg-muted/30 transition-colors" : "cursor-default"}`}
            >
                <div className="flex-shrink-0">
                    {expandable ? (
                        open ? <ChevronDown className="w-4 h-4 text-muted-foreground" /> : <ChevronRight className="w-4 h-4 text-muted-foreground" />
                    ) : (
                        <div className="w-4 h-4" />
                    )}
                </div>
                <Badge variant="outline" className={`gap-1 flex-shrink-0 ${meta.color}`}>
                    <Icon className="w-3 h-3" />
                    {meta.label}
                </Badge>
                <div className="flex-1 min-w-0">
                    <p className="text-sm text-foreground truncate">{summary}</p>
                    {!onMonitorPage && (
                        <p className="text-xs text-muted-foreground truncate mt-0.5">
                            <Link
                                to={`/monitors/${monitorId}`}
                                className="hover:text-foreground hover:underline"
                                onClick={(e) => e.stopPropagation()}
                            >
                                {monitorName}
                            </Link>
                            {groupName && <span className="opacity-60"> · {groupName}</span>}
                        </p>
                    )}
                </div>
                <div className="flex items-center gap-3 text-xs text-muted-foreground flex-shrink-0">
                    {rightAction}
                    {!isSSL && (
                        <span className={`font-mono ${isOngoing ? "text-rose-500" : ""}`}>
                            {duration}{isOngoing ? " ongoing" : ""}
                        </span>
                    )}
                    <span className="flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {formatDate(startedAt, user?.timezone)}
                    </span>
                </div>
            </CollapsibleTrigger>

            {expandable && (
                <CollapsibleContent className="border-t border-border bg-muted/20 px-3 py-3 space-y-2">
                    <div className="flex items-center gap-2 text-xs text-muted-foreground">
                        <Activity className="w-3.5 h-3.5" />
                        Events during this incident
                        {!onMonitorPage && (
                            <Link
                                to={`/monitors/${monitorId}?date=${startedAt.slice(0, 10)}`}
                                className="ml-auto text-primary hover:underline"
                                onClick={(e) => e.stopPropagation()}
                            >
                                Open monitor page →
                            </Link>
                        )}
                    </div>
                    {eventsQ.isLoading && (
                        <div className="space-y-2">
                            <Skeleton className="h-10 w-full" />
                            <Skeleton className="h-10 w-full" />
                        </div>
                    )}
                    {eventsQ.error && (
                        <div className="text-xs text-rose-500 border border-rose-500/30 rounded p-2 bg-rose-500/5">
                            Failed to load events: {(eventsQ.error as Error).message}
                        </div>
                    )}
                    {eventsQ.data && eventsQ.data.length === 0 && (
                        <div className="text-xs text-muted-foreground italic">No detailed events captured during this window.</div>
                    )}
                    {eventsQ.data && eventsQ.data.length > 0 && (
                        <div className="space-y-2">
                            {groupConsecutiveEvents(eventsQ.data).map(group => (
                                group.events.length === 1
                                    ? <EventDetailCard key={group.key} event={group.first} />
                                    : <EventGroupCard key={group.key} group={group} />
                            ))}
                        </div>
                    )}
                </CollapsibleContent>
            )}
        </Collapsible>
    );
}
