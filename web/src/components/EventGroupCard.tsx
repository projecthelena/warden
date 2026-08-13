import { useState } from "react";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Badge } from "@/components/ui/badge";
import { ChevronDown, ChevronRight, AlertTriangle, ArrowDown, ArrowUp, Tornado, Lock, CircleDot, Repeat } from "lucide-react";
import { formatTime } from "@/lib/utils";
import { useMonitorStore } from "@/lib/store";
import { EventGroup, cameFromCheck, formatLatency } from "@/lib/eventGroups";
import { EventDetailCard } from "@/components/EventDetailCard";

// A run of consecutive identical checks — "it stayed down, here is the shape of it" —
// instead of making the reader scroll past 84 rows that say the same thing.
//
// Two renderings share these parts. When an incident contains several runs, each one gets
// its own EventGroupCard so the reader can see where the behaviour changed. When it
// contains a single run (the common case: down the whole time for the same reason) the
// card would be a box around the only thing in the drawer, so IncidentCard drops it and
// uses RunMeta / RunChips / EventGroupBody directly.

const TYPE_META: Record<string, { label: string; color: string; Icon: typeof ArrowDown }> = {
    down:         { label: "Down",         color: "text-rose-500 border-rose-500/30 bg-rose-500/5",          Icon: ArrowDown },
    degraded:     { label: "Degraded",     color: "text-amber-500 border-amber-500/30 bg-amber-500/5",       Icon: AlertTriangle },
    flapping:     { label: "Flapping",     color: "text-purple-500 border-purple-500/30 bg-purple-500/5",    Icon: Tornado },
    ssl_expiring: { label: "SSL Expiring", color: "text-orange-500 border-orange-500/30 bg-orange-500/5",    Icon: Lock },
    stabilized:   { label: "Stabilized",   color: "text-blue-500 border-blue-500/30 bg-blue-500/5",          Icon: CircleDot },
    up:           { label: "Up",           color: "text-emerald-500 border-emerald-500/30 bg-emerald-500/5", Icon: ArrowUp },
    recovered:    { label: "Recovered",    color: "text-emerald-500 border-emerald-500/30 bg-emerald-500/5", Icon: ArrowUp },
};

/** "⟳ 128 identical checks · every 10s · 22:01:59 → 22:22:59" */
export function RunMeta({ group }: { group: EventGroup }) {
    const { user } = useMonitorStore();
    return (
        <>
            <Repeat className="w-3 h-3 flex-shrink-0" />
            <span>{group.events.length} identical checks</span>
            {group.intervalSeconds && <span className="opacity-60">· every {group.intervalSeconds}s</span>}
            <span className="opacity-60 truncate">
                · {formatTime(group.first.timestamp, user?.timezone)} → {formatTime(group.last.timestamp, user?.timezone)}
            </span>
        </>
    );
}

/** "HTTP 500   ~1ms" — the response shape shared by every check in the run. */
export function RunChips({ group }: { group: EventGroup }) {
    const sample = group.first;
    const latency = formatLatency(group.medianLatency, cameFromCheck(sample));
    return (
        <>
            {typeof sample.statusCode === "number" && sample.statusCode > 0 && (
                <span className="font-mono" title="HTTP status code returned by every check in this run">
                    HTTP {sample.statusCode}
                </span>
            )}
            {latency && (
                <span className="font-mono" title="Median response time across this run">
                    ~{latency}
                </span>
            )}
        </>
    );
}

/** First and last check of the run, with the full list one more click away. */
export function EventGroupBody({ group }: { group: EventGroup }) {
    const [showAll, setShowAll] = useState(false);
    const count = group.events.length;

    if (showAll) {
        return (
            <>
                <p className="text-xs text-muted-foreground">All {count} checks, newest first</p>
                {group.events.map(ev => <EventDetailCard key={ev.id} event={ev} />)}
                <button type="button" onClick={() => setShowAll(false)} className="text-xs text-primary hover:underline">
                    Collapse back to first and last
                </button>
            </>
        );
    }

    return (
        <>
            <p className="text-xs text-muted-foreground">First check of the run</p>
            <EventDetailCard event={group.first} />
            <p className="text-xs text-muted-foreground pt-1">Last check of the run</p>
            <EventDetailCard event={group.last} />
            {count > 2 && (
                <button type="button" onClick={() => setShowAll(true)} className="text-xs text-primary hover:underline">
                    Show all {count} checks →
                </button>
            )}
        </>
    );
}

export function EventGroupCard({ group }: { group: EventGroup }) {
    const [open, setOpen] = useState(false);

    const sample = group.first;
    const meta = TYPE_META[sample.type] ?? { label: sample.type, color: "text-muted-foreground border-border bg-muted/30", Icon: AlertTriangle };
    const Icon = meta.Icon;

    return (
        <Collapsible open={open} onOpenChange={setOpen} className="border border-border rounded-lg bg-card overflow-hidden">
            <CollapsibleTrigger className="w-full text-left px-3 py-2.5 hover:bg-muted/30 transition-colors flex items-center gap-3">
                <div className="flex-shrink-0">
                    {open ? <ChevronDown className="w-4 h-4 text-muted-foreground" /> : <ChevronRight className="w-4 h-4 text-muted-foreground" />}
                </div>
                <Badge variant="outline" className={`gap-1 flex-shrink-0 ${meta.color}`}>
                    <Icon className="w-3 h-3" />
                    {meta.label}
                </Badge>
                <div className="flex-1 min-w-0">
                    <p className="text-sm text-foreground truncate">{sample.message}</p>
                    <p className="text-xs text-muted-foreground mt-0.5 flex items-center gap-1.5">
                        <RunMeta group={group} />
                    </p>
                </div>
                <div className="flex items-center gap-3 text-xs text-muted-foreground flex-shrink-0">
                    <RunChips group={group} />
                </div>
            </CollapsibleTrigger>

            <CollapsibleContent className="border-t border-border bg-muted/20 px-3 py-3 space-y-2">
                <EventGroupBody group={group} />
            </CollapsibleContent>
        </Collapsible>
    );
}
