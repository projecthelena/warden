import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { cn, formatDate } from "@/lib/utils";
import { IncidentUpdate } from "@/lib/store";
import { useState } from "react";
import { AlertCircle, CheckCircle2, Eye, Search, Clock, MessageCircle, Plus } from "lucide-react";

interface IncidentTimelineProps {
    updates: IncidentUpdate[];
    incidentId?: string;
    onAddUpdate?: (status: string, message: string) => Promise<void>;
    readonly?: boolean;
    timezone?: string;
}

const statusConfig: Record<string, { icon: React.ReactNode; color: string; label: string }> = {
    investigating: {
        icon: <Search className="w-3.5 h-3.5" />,
        color: "text-yellow-500 bg-yellow-500/10 border-yellow-500/30",
        label: "Investigating",
    },
    identified: {
        icon: <Eye className="w-3.5 h-3.5" />,
        color: "text-orange-500 bg-orange-500/10 border-orange-500/30",
        label: "Identified",
    },
    monitoring: {
        icon: <AlertCircle className="w-3.5 h-3.5" />,
        color: "text-blue-500 bg-blue-500/10 border-blue-500/30",
        label: "Monitoring",
    },
    resolved: {
        icon: <CheckCircle2 className="w-3.5 h-3.5" />,
        color: "text-emerald-500 bg-emerald-500/10 border-emerald-500/30",
        label: "Resolved",
    },
    completed: {
        icon: <CheckCircle2 className="w-3.5 h-3.5" />,
        color: "text-emerald-500 bg-emerald-500/10 border-emerald-500/30",
        label: "Completed",
    },
};

function getStatusConfig(status: string) {
    return (
        statusConfig[status] || {
            icon: <MessageCircle className="w-3.5 h-3.5" />,
            color: "text-muted-foreground bg-muted/10 border-muted-foreground/30",
            label: status.charAt(0).toUpperCase() + status.slice(1),
        }
    );
}

function TimelineEntry({ update, timezone }: { update: IncidentUpdate; timezone?: string }) {
    const config = getStatusConfig(update.status);

    return (
        <div className="relative flex gap-4 pb-6 last:pb-0">
            {/* Vertical line */}
            <div className="absolute left-[15px] top-8 bottom-0 w-px bg-border last:hidden" />

            {/* Status icon */}
            <div
                className={cn(
                    "relative z-10 flex items-center justify-center w-8 h-8 rounded-full border shrink-0",
                    config.color
                )}
            >
                {config.icon}
            </div>

            {/* Content */}
            <div className="flex-1 min-w-0 pt-0.5">
                <div className="flex items-center gap-2 mb-1">
                    <Badge
                        variant="outline"
                        className={cn(
                            "text-[10px] uppercase tracking-wider font-mono px-1.5 py-0 h-5",
                            config.color
                        )}
                    >
                        {config.label}
                    </Badge>
                    <span className="text-xs text-muted-foreground flex items-center gap-1">
                        <Clock className="w-3 h-3" />
                        {formatDate(update.createdAt, timezone)}
                    </span>
                </div>
                <p className="text-sm text-foreground/90 whitespace-pre-wrap">{update.message}</p>
            </div>
        </div>
    );
}

function AddUpdateForm({
    onSubmit,
    isSubmitting,
}: {
    onSubmit: (status: string, message: string) => void;
    isSubmitting: boolean;
}) {
    const [status, setStatus] = useState("monitoring");
    const [message, setMessage] = useState("");

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!message.trim()) return;
        onSubmit(status, message.trim());
        setMessage("");
    };

    return (
        <form onSubmit={handleSubmit} className="space-y-3 border-t border-border/50 pt-4 mt-4">
            <div className="flex items-center gap-2">
                <Plus className="w-4 h-4 text-muted-foreground" />
                <span className="text-xs font-medium text-muted-foreground uppercase tracking-wider">
                    Add Update
                </span>
            </div>
            <div className="flex gap-3">
                <Select value={status} onValueChange={setStatus}>
                    <SelectTrigger className="w-[140px] bg-background border-border/50 h-9">
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="investigating">Investigating</SelectItem>
                        <SelectItem value="identified">Identified</SelectItem>
                        <SelectItem value="monitoring">Monitoring</SelectItem>
                        <SelectItem value="resolved">Resolved</SelectItem>
                    </SelectContent>
                </Select>
                <Textarea
                    value={message}
                    onChange={(e) => setMessage(e.target.value)}
                    placeholder="Describe the current status or update..."
                    className="flex-1 min-h-[72px] bg-background border-border/50 resize-none"
                />
            </div>
            <div className="flex justify-end">
                <Button type="submit" size="sm" disabled={isSubmitting || !message.trim()}>
                    {isSubmitting ? "Posting..." : "Post Update"}
                </Button>
            </div>
        </form>
    );
}

export function IncidentTimeline({
    updates = [],
    onAddUpdate,
    readonly = false,
    timezone,
}: IncidentTimelineProps) {
    const [isSubmitting, setIsSubmitting] = useState(false);

    const handleAddUpdate = async (status: string, message: string) => {
        if (!onAddUpdate) return;
        setIsSubmitting(true);
        try {
            await onAddUpdate(status, message);
        } finally {
            setIsSubmitting(false);
        }
    };

    if (updates.length === 0 && readonly) {
        return null;
    }

    return (
        <div className="space-y-4">
            {updates.length > 0 && (
                <div className="space-y-0">
                    {updates.map((update, index) => (
                        <TimelineEntry key={update.id || index} update={update} timezone={timezone} />
                    ))}
                </div>
            )}

            {!readonly && onAddUpdate && (
                <AddUpdateForm onSubmit={handleAddUpdate} isSubmitting={isSubmitting} />
            )}
        </div>
    );
}
