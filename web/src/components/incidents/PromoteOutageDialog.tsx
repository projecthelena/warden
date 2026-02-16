import { Button } from "@/components/ui/button";
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { SystemIncident } from "@/lib/store";
import { useState } from "react";
import { AlertTriangle, ArrowRight } from "lucide-react";

interface PromoteOutageDialogProps {
    outage: SystemIncident | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onPromote: (outageId: string, data: {
        title: string;
        description: string;
        severity: string;
        affectedGroups: string[];
    }) => Promise<void>;
}

export function PromoteOutageDialog({
    outage,
    open,
    onOpenChange,
    onPromote,
}: PromoteOutageDialogProps) {
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [severity, setSeverity] = useState<"minor" | "major" | "critical">("critical");
    const [isSubmitting, setIsSubmitting] = useState(false);

    // Reset form when outage changes
    const resetForm = () => {
        if (outage) {
            setTitle(`Service Disruption: ${outage.monitorName}`);
            setDescription(outage.message || "");
            setSeverity(outage.type === "down" ? "critical" : "major");
        }
    };

    const handleOpenChange = (newOpen: boolean) => {
        if (newOpen && outage) {
            resetForm();
        }
        onOpenChange(newOpen);
    };

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!outage) return;

        setIsSubmitting(true);
        try {
            await onPromote(outage.id, {
                title: title || `Service Disruption: ${outage.monitorName}`,
                description: description || outage.message || "",
                severity,
                affectedGroups: [outage.groupId],
            });
            onOpenChange(false);
        } finally {
            setIsSubmitting(false);
        }
    };

    if (!outage) return null;

    return (
        <Dialog open={open} onOpenChange={handleOpenChange}>
            <DialogContent className="sm:max-w-[500px]">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <AlertTriangle className="w-5 h-5 text-yellow-500" />
                        Create Incident from Outage
                    </DialogTitle>
                    <DialogDescription>
                        Promote this auto-detected outage to a trackable incident. This allows you to
                        add status updates and optionally make it visible on public status pages.
                    </DialogDescription>
                </DialogHeader>

                <form onSubmit={handleSubmit} className="space-y-4 mt-4">
                    <div className="p-3 rounded-lg bg-muted/50 border border-border/50 flex items-center gap-3 text-sm">
                        <div className="flex-1 min-w-0">
                            <div className="font-medium text-foreground truncate">
                                {outage.monitorName}
                            </div>
                            <div className="text-xs text-muted-foreground truncate">
                                {outage.groupName} &bull; {outage.type === "down" ? "Unavailable" : "Degraded"}
                            </div>
                        </div>
                        <ArrowRight className="w-4 h-4 text-muted-foreground shrink-0" />
                        <div className="text-xs text-muted-foreground">Incident</div>
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="title">Title</Label>
                        <Input
                            id="title"
                            value={title}
                            onChange={(e) => setTitle(e.target.value)}
                            placeholder={`Service Disruption: ${outage.monitorName}`}
                            className="bg-background"
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="description">Description</Label>
                        <Textarea
                            id="description"
                            value={description}
                            onChange={(e) => setDescription(e.target.value)}
                            placeholder="Brief description of the issue..."
                            className="bg-background resize-none min-h-[80px]"
                        />
                    </div>

                    <div className="space-y-2">
                        <Label htmlFor="severity">Severity</Label>
                        <Select value={severity} onValueChange={(v) => setSeverity(v as typeof severity)}>
                            <SelectTrigger className="bg-background">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="critical">Critical</SelectItem>
                                <SelectItem value="major">Major</SelectItem>
                                <SelectItem value="minor">Minor</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <DialogFooter className="mt-6">
                        <Button
                            type="button"
                            variant="outline"
                            onClick={() => onOpenChange(false)}
                            disabled={isSubmitting}
                        >
                            Cancel
                        </Button>
                        <Button type="submit" disabled={isSubmitting}>
                            {isSubmitting ? "Creating..." : "Create Incident"}
                        </Button>
                    </DialogFooter>
                </form>
            </DialogContent>
        </Dialog>
    );
}
