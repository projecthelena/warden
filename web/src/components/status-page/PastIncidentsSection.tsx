import { Badge } from "@/components/ui/badge";
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible";
import { cn, formatDate } from "@/lib/utils";
import { Incident, IncidentUpdate } from "@/lib/store";
import { CheckCircle2, ChevronDown, Clock, History } from "lucide-react";
import { useState } from "react";

interface PastIncident extends Incident {
    updates?: IncidentUpdate[];
    duration?: string;
}

interface PastIncidentsSectionProps {
    incidents: PastIncident[];
    timezone?: string;
}

const severityConfig = {
    critical: {
        color: "text-red-500 bg-red-500/10 border-red-500/30",
        label: "Critical",
    },
    major: {
        color: "text-orange-500 bg-orange-500/10 border-orange-500/30",
        label: "Major",
    },
    minor: {
        color: "text-yellow-500 bg-yellow-500/10 border-yellow-500/30",
        label: "Minor",
    },
};

function groupIncidentsByDate(incidents: PastIncident[]) {
    const groups: Record<string, PastIncident[]> = {};

    incidents.forEach((incident) => {
        const date = new Date(incident.startTime);
        const dateKey = date.toISOString().split("T")[0];

        if (!groups[dateKey]) {
            groups[dateKey] = [];
        }
        groups[dateKey].push(incident);
    });

    // Sort by date descending and convert to array
    return Object.entries(groups)
        .sort(([a], [b]) => b.localeCompare(a))
        .map(([dateKey, incidents]) => ({
            dateKey,
            displayDate: new Date(dateKey).toLocaleDateString("en-US", {
                weekday: "long",
                year: "numeric",
                month: "long",
                day: "numeric",
            }),
            incidents,
        }));
}

function IncidentTimeline({ updates, timezone }: { updates?: IncidentUpdate[]; timezone?: string }) {
    if (!updates || updates.length === 0) return null;

    return (
        <div className="mt-3 pl-4 border-l-2 border-border/50 space-y-3">
            {updates.map((update, idx) => (
                <div key={idx} className="relative">
                    <div
                        className="absolute -left-[13px] top-1 w-2 h-2 rounded-full bg-border"
                        style={{ marginTop: "4px" }}
                    />
                    <div className="space-y-0.5">
                        <div className="flex items-center gap-2">
                            <Badge
                                variant="outline"
                                className="text-[9px] uppercase tracking-wider font-mono px-1 py-0 h-4 border-muted-foreground/30"
                            >
                                {update.status}
                            </Badge>
                            <span className="text-[10px] text-muted-foreground tabular-nums">
                                {formatDate(update.createdAt, timezone)}
                            </span>
                        </div>
                        <p className="text-xs text-muted-foreground">{update.message}</p>
                    </div>
                </div>
            ))}
        </div>
    );
}

function PastIncidentCard({ incident, timezone }: { incident: PastIncident; timezone?: string }) {
    const [isOpen, setIsOpen] = useState(false);
    const hasUpdates = incident.updates && incident.updates.length > 0;
    const severity = severityConfig[incident.severity] || severityConfig.minor;

    return (
        <Collapsible open={isOpen} onOpenChange={setIsOpen}>
            <div className="rounded-lg border border-border/50 bg-card/30 overflow-hidden">
                <CollapsibleTrigger asChild>
                    <button
                        className="w-full px-4 py-3 flex items-start justify-between gap-4 text-left hover:bg-accent/30 transition-colors"
                    >
                        <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 mb-1">
                                <CheckCircle2 className="w-4 h-4 text-emerald-500 shrink-0" />
                                <span className="font-medium text-sm text-foreground truncate">
                                    {incident.title}
                                </span>
                                <Badge
                                    variant="outline"
                                    className={cn(
                                        "text-[9px] uppercase tracking-wider font-mono px-1 py-0 h-4 shrink-0",
                                        severity.color
                                    )}
                                >
                                    {severity.label}
                                </Badge>
                            </div>
                            {incident.description && (
                                <p className="text-xs text-muted-foreground truncate pl-6">
                                    {incident.description}
                                </p>
                            )}
                        </div>
                        <div className="flex items-center gap-3 shrink-0">
                            {incident.duration && (
                                <span className="text-xs text-muted-foreground tabular-nums flex items-center gap-1">
                                    <Clock className="w-3 h-3" />
                                    {incident.duration}
                                </span>
                            )}
                            {hasUpdates && (
                                <ChevronDown
                                    className={cn(
                                        "w-4 h-4 text-muted-foreground transition-transform",
                                        isOpen && "rotate-180"
                                    )}
                                />
                            )}
                        </div>
                    </button>
                </CollapsibleTrigger>
                {hasUpdates && (
                    <CollapsibleContent>
                        <div className="px-4 pb-4">
                            <IncidentTimeline updates={incident.updates} timezone={timezone} />
                        </div>
                    </CollapsibleContent>
                )}
            </div>
        </Collapsible>
    );
}

function DateGroup({
    dateKey,
    displayDate,
    incidents,
    timezone,
}: {
    dateKey: string;
    displayDate: string;
    incidents: PastIncident[];
    timezone?: string;
}) {
    const isToday = dateKey === new Date().toISOString().split("T")[0];
    const isYesterday =
        dateKey ===
        new Date(Date.now() - 86400000).toISOString().split("T")[0];

    let dateLabel = displayDate;
    if (isToday) dateLabel = "Today";
    if (isYesterday) dateLabel = "Yesterday";

    return (
        <div className="space-y-3">
            <div className="flex items-center gap-2">
                <span className="text-xs font-medium text-muted-foreground">{dateLabel}</span>
                <div className="flex-1 h-px bg-border/50" />
            </div>
            <div className="space-y-2">
                {incidents.map((incident) => (
                    <PastIncidentCard key={incident.id} incident={incident} timezone={timezone} />
                ))}
            </div>
        </div>
    );
}

function EmptyState() {
    return (
        <div className="flex flex-col items-center justify-center py-8 text-muted-foreground/60">
            <CheckCircle2 className="w-8 h-8 mb-2 text-emerald-500/30" />
            <p className="text-sm">No incidents reported</p>
            <p className="text-xs opacity-70">in the last 14 days</p>
        </div>
    );
}

export function PastIncidentsSection({ incidents, timezone }: PastIncidentsSectionProps) {
    const groupedIncidents = groupIncidentsByDate(incidents);

    if (incidents.length === 0) {
        return (
            <div className="animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards">
                <div className="mb-3 px-1 flex items-center gap-2">
                    <History className="w-4 h-4 text-muted-foreground" />
                    <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                        Past Incidents
                    </h3>
                </div>
                <EmptyState />
            </div>
        );
    }

    return (
        <div className="animate-in slide-in-from-bottom-4 duration-700 fade-in fill-mode-backwards space-y-6">
            <div className="flex items-center gap-2 px-1">
                <History className="w-4 h-4 text-muted-foreground" />
                <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wider">
                    Past Incidents
                </h3>
                <Badge variant="secondary" className="text-[10px] px-1.5 py-0 h-4">
                    {incidents.length}
                </Badge>
            </div>

            <div className="space-y-6">
                {groupedIncidents.map(({ dateKey, displayDate, incidents }) => (
                    <DateGroup
                        key={dateKey}
                        dateKey={dateKey}
                        displayDate={displayDate}
                        incidents={incidents}
                        timezone={timezone}
                    />
                ))}
            </div>
        </div>
    );
}
