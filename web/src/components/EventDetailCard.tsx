import { useState } from "react";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { Badge } from "@/components/ui/badge";
import { ChevronDown, ChevronRight, AlertTriangle, ArrowDown, ArrowUp, Tornado, Lock, CircleDot, Clock } from "lucide-react";
import { formatDate } from "@/lib/utils";
import { useMonitorStore } from "@/lib/store";
import { EnrichedMonitorEvent } from "@/hooks/useMonitorEvents";
import { cameFromCheck, formatLatency } from "@/lib/eventGroups";

interface EventDetailCardProps {
    event: EnrichedMonitorEvent;
}

const TYPE_META: Record<string, { label: string; color: string; Icon: typeof ArrowDown }> = {
    down:         { label: "Down",         color: "text-rose-500 border-rose-500/30 bg-rose-500/5",        Icon: ArrowDown },
    degraded:     { label: "Degraded",     color: "text-amber-500 border-amber-500/30 bg-amber-500/5",     Icon: AlertTriangle },
    flapping:     { label: "Flapping",     color: "text-purple-500 border-purple-500/30 bg-purple-500/5",  Icon: Tornado },
    ssl_expiring: { label: "SSL Expiring", color: "text-orange-500 border-orange-500/30 bg-orange-500/5",  Icon: Lock },
    stabilized:   { label: "Stabilized",   color: "text-blue-500 border-blue-500/30 bg-blue-500/5",        Icon: CircleDot },
    up:           { label: "Up",           color: "text-emerald-500 border-emerald-500/30 bg-emerald-500/5", Icon: ArrowUp },
    recovered:    { label: "Recovered",    color: "text-emerald-500 border-emerald-500/30 bg-emerald-500/5", Icon: ArrowUp },
};

export function EventDetailCard({ event }: EventDetailCardProps) {
    const [open, setOpen] = useState(false);
    const { user } = useMonitorStore();
    const meta = TYPE_META[event.type] ?? { label: event.type, color: "text-muted-foreground border-border bg-muted/30", Icon: AlertTriangle };
    const Icon = meta.Icon;

    const hasDetails = !!(event.errorMessage || event.responseBody || (event.responseHeaders && Object.keys(event.responseHeaders).length > 0));
    // A check that answered in under a millisecond stores no latency, which used to render
    // as an empty gap that read like missing data. Say "<1ms" instead.
    const latency = formatLatency(event.latency, cameFromCheck(event));

    // The header row content is identical whether the event is expandable or not — extracted
    // so non-expandable events render as a plain (non-clickable) div instead of a button that
    // appears interactive but does nothing on click.
    const header = (
        <>
            <div className="flex-shrink-0">
                {hasDetails ? (
                    open ? <ChevronDown className="w-4 h-4 text-muted-foreground" /> : <ChevronRight className="w-4 h-4 text-muted-foreground" />
                ) : (
                    <div className="w-4 h-4" />
                )}
            </div>
            <Badge variant="outline" className={`gap-1 ${meta.color}`}>
                <Icon className="w-3 h-3" />
                {meta.label}
            </Badge>
            <div className="flex-1 min-w-0">
                <p className="text-sm text-foreground truncate">{event.message}</p>
            </div>
            <div className="flex items-center gap-3 text-xs text-muted-foreground flex-shrink-0">
                {typeof event.statusCode === "number" && event.statusCode > 0 && (
                    <span className="font-mono" title="HTTP status code returned by the check">
                        HTTP {event.statusCode}
                    </span>
                )}
                {latency && (
                    <span className="font-mono" title="Response time measured by the check">
                        {latency}
                    </span>
                )}
                <span className="flex items-center gap-1">
                    <Clock className="w-3 h-3" />
                    {formatDate(event.timestamp, user?.timezone)}
                </span>
            </div>
        </>
    );

    if (!hasDetails) {
        return (
            <div className="border border-border rounded-lg bg-card px-3 py-2.5 flex items-center gap-3">
                {header}
            </div>
        );
    }

    return (
        <Collapsible open={open} onOpenChange={setOpen} className="border border-border rounded-lg bg-card overflow-hidden">
            <CollapsibleTrigger className="w-full text-left px-3 py-2.5 hover:bg-muted/30 transition-colors flex items-center gap-3">
                {header}
            </CollapsibleTrigger>
            <CollapsibleContent className="border-t border-border bg-muted/20 px-3 py-3 space-y-3">
                    {event.errorMessage && (
                        <Section title="Error">
                            <pre className="text-xs font-mono whitespace-pre-wrap break-words text-rose-500/90 bg-rose-500/5 border border-rose-500/20 rounded p-2">
                                {linkify(event.errorMessage)}
                            </pre>
                        </Section>
                    )}
                    {event.responseHeaders && Object.keys(event.responseHeaders).length > 0 && (
                        <Section title="Response headers">
                            <div className="border border-border rounded overflow-hidden">
                                <table className="w-full text-xs font-mono">
                                    <tbody>
                                        {Object.entries(event.responseHeaders).map(([k, v]) => (
                                            <tr key={k} className="border-b border-border last:border-0">
                                                <td className="px-2 py-1 align-top text-muted-foreground bg-muted/40 whitespace-nowrap w-1/3">{k}</td>
                                                <td className="px-2 py-1 break-all">{v}</td>
                                            </tr>
                                        ))}
                                    </tbody>
                                </table>
                            </div>
                        </Section>
                    )}
                    {event.responseBody && (
                        <Section title={`Response body (truncated to ~2KB)`}>
                            <pre className="text-xs font-mono whitespace-pre-wrap break-words bg-muted/40 border border-border rounded p-2 max-h-72 overflow-y-auto">
                                {event.responseBody}
                            </pre>
                        </Section>
                    )}
            </CollapsibleContent>
        </Collapsible>
    );
}

// Some check errors carry a link to the page that explains the fix. A link the reader has
// to select and copy out of a <pre> is a link nobody follows.
function linkify(text: string) {
    return text.split(/(https?:\/\/[^\s)]+)/g).map((part, i) =>
        part.startsWith("http") ? (
            <a
                key={i}
                href={part}
                target="_blank"
                rel="noreferrer"
                className="underline underline-offset-2 hover:text-rose-400"
            >
                {part}
            </a>
        ) : (
            part
        )
    );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
    return (
        <div>
            <p className="text-xs font-medium text-muted-foreground mb-1.5">{title}</p>
            {children}
        </div>
    );
}
