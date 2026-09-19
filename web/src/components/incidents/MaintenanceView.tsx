/* Hallmark · component: maintenance cards · genre: modern-minimal · theme: existing Warden tokens
 * Hallmark · pre-emit critique: P5 H5 E4 S5 R5 V4 · contrast: pass · responsive: pass
 */
import { useEffect } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { useMonitorStore, Incident } from "@/lib/store";
import { useRole } from "@/hooks/useRole";
import { ArrowRight, CalendarDays, CheckCircle2, Clock3, MoreVertical, Pencil, Trash2 } from "lucide-react";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { useState } from "react";
import { useToast } from "@/components/ui/use-toast";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog";
// Select imports? Maybe simpler native select or Shadcn Select. Using native for speed/stability if Select not imported.
// Actually let's assume Select is available or use native. User prefers vanilla Shadcn.
// Importing Select components

import { Group } from "@/lib/store";
import { formatDate } from "@/lib/utils";
import { formatDateTimeLocal, isMaintenanceActive, isMaintenanceFinished, zonedDateTimeToISOString } from "@/lib/maintenance";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

function MaintenanceCard({
    incident,
    groups,
    timezone,
    onEdit,
    onDelete,
    onEndNow,
}: {
    incident: Incident;
    groups: Group[];
    timezone: string;
    onEdit?: (i: Incident) => void;
    onDelete?: (id: string) => void;
    onEndNow?: (i: Incident) => void;
}) {
    const affectedGroupNames =
        incident.affectedGroups?.map((id) => {
            const g = groups.find((group) => group.id === id);
            return g ? g.name : id;
        }) || [];

    const now = new Date();
    const isOngoing = isMaintenanceActive(incident, now);
    const isHistory = isMaintenanceFinished(incident, now);

    return (
        <Card data-testid={`maintenance-card-${incident.id}`} className="overflow-hidden border-border/60 bg-card shadow-none">
            <CardHeader className="grid gap-4 space-y-0 p-5 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-start">
                <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                        <CardTitle className="min-w-0 text-base font-semibold leading-snug">{incident.title}</CardTitle>
                        {isOngoing ? (
                            <Badge variant="secondary" className="h-5 border-0 bg-emerald-500/10 px-2 py-0 text-[10px] font-medium text-emerald-500">
                                Ongoing
                            </Badge>
                        ) : isHistory ? (
                            <Badge variant="secondary" className="h-5 border-0 px-2 py-0 text-[10px] font-medium text-muted-foreground">
                                Completed
                            </Badge>
                        ) : (
                            <Badge variant="secondary" className="h-5 border-0 bg-blue-500/10 px-2 py-0 text-[10px] font-medium text-blue-500">
                                Scheduled
                            </Badge>
                        )}
                    </div>
                    {incident.description && <CardDescription className="max-w-2xl text-sm leading-relaxed">{incident.description}</CardDescription>}
                </div>
                <div className="flex items-start justify-between gap-2 sm:justify-end">
                    <div className="grid gap-1.5 text-xs text-muted-foreground sm:min-w-64">
                        <div className="flex items-center gap-2">
                            <CalendarDays className="h-3.5 w-3.5 shrink-0" />
                            <span>{formatDate(incident.startTime, timezone)}</span>
                        </div>
                        {incident.endTime && (
                            <div className="flex items-center gap-2 pl-5">
                                <ArrowRight className="h-3 w-3 shrink-0" />
                                <span>{formatDate(incident.endTime, timezone)}</span>
                            </div>
                        )}
                    </div>
                    {onDelete && (
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button variant="ghost" size="icon" className="h-9 w-9 shrink-0" data-testid={`maintenance-actions-${incident.id}`}>
                                    <span className="sr-only">Open menu</span>
                                    <MoreVertical className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuLabel>Actions</DropdownMenuLabel>
                                {!isHistory && onEdit && (
                                    <DropdownMenuItem onClick={() => onEdit(incident)}>
                                        <Pencil className="mr-2 h-4 w-4" />
                                        Edit Details
                                    </DropdownMenuItem>
                                )}
                                {isOngoing && (
                                    <DropdownMenuItem onClick={() => onEndNow?.(incident)}>
                                        <CheckCircle2 className="mr-2 h-4 w-4" />
                                        End Now
                                    </DropdownMenuItem>
                                )}
                                <DropdownMenuSeparator />
                                <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDelete?.(incident.id)}>
                                    <Trash2 className="mr-2 h-4 w-4" />
                                    Delete
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    )}
                </div>
            </CardHeader>
            <CardContent className="border-t border-border/50 bg-muted/20 px-5 py-3">
                <div className="flex flex-wrap items-center gap-2">
                    <span className="text-xs text-muted-foreground">Affected</span>
                    <div className="flex flex-wrap gap-1.5">
                        {affectedGroupNames.length > 0 ? (
                            affectedGroupNames.map((name, i) => (
                                <Badge key={i} variant="outline" className="bg-background text-xs font-normal text-foreground/80">
                                    {name}
                                </Badge>
                            ))
                        ) : (
                            <span className="text-xs text-muted-foreground">All groups</span>
                        )}
                    </div>
                </div>
            </CardContent>
        </Card>
    );
}

export function MaintenanceView() {
    const { incidents, groups, user, fetchIncidents } = useMonitorStore();
    const { canEdit } = useRole();
    const { toast } = useToast();
    const [editingIncident, setEditingIncident] = useState<Incident | null>(null);
    const [deletingId, setDeletingId] = useState<string | null>(null);

    // Edit Form State
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [startTime, setStartTime] = useState("");
    const [endTime, setEndTime] = useState("");
    const timezone = user?.timezone || "UTC";
    // const [selectedGroups, setSelectedGroups] = useState<string[]>([]); // Simple Multi-select? Or single? API supports array.
    // For simplicity, we might just support "All" or toggle.
    // Let's implement full editing if possible, or minimalistic.
    // Assuming UI simplicity: standard Shadcn doesn't have MultiSelect native. I'll use simple select for single group or "All" logic if needed, or checkboxes.
    // But existing UI shows badges for multiple groups.
    // For now, I'll allow *keeping* existing groups or clearing.
    // Actually, I won't implement group editing in this first pass to keep it simple, or just a simple text area for IDs? No that's bad.
    // I'll skip group editing for now to minimize complexity, focus on Title/Desc/Time.

    useEffect(() => {
        fetchIncidents();
    }, [fetchIncidents]);

    const handleEdit = (i: Incident) => {
        setEditingIncident(i);
        setTitle(i.title);
        setDescription(i.description || "");
        setStartTime(formatDateTimeLocal(i.startTime, timezone));
        setEndTime(i.endTime ? formatDateTimeLocal(i.endTime, timezone) : "");
    };

    const handleDelete = (id: string) => {
        setDeletingId(id);
    };

    const handleEndNow = async (i: Incident) => {
        try {
            // Update end time to now
            const res = await fetch(`/api/maintenance/${i.id}`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    title: i.title,
                    description: i.description,
                    status: "completed", // Or resolved?
                    startTime: i.startTime, // Keep original start
                    endTime: new Date().toISOString(),
                    affectedGroups: i.affectedGroups || [],
                }),
            });
            if (!res.ok) throw new Error("Failed to end maintenance");

            toast({
                title: "Maintenance Ended",
                description: "The maintenance window has been closed.",
            });
            fetchIncidents();
        } catch (_e) {
            toast({
                variant: "destructive",
                title: "Error",
                description: "Could not end maintenance.",
            });
        }
    };

    const confirmDelete = async () => {
        if (!deletingId) return;
        try {
            const res = await fetch(`/api/maintenance/${deletingId}`, {
                method: "DELETE",
            });
            if (!res.ok) throw new Error("Failed to delete");
            toast({ title: "Deleted", description: "Maintenance window deleted." });
            fetchIncidents();
        } catch (_e) {
            toast({
                variant: "destructive",
                title: "Error",
                description: "Could not delete maintenance.",
            });
        } finally {
            setDeletingId(null);
        }
    };

    const saveEdit = async () => {
        if (!editingIncident) return;
        const start = zonedDateTimeToISOString(startTime, timezone);
        const end = endTime ? zonedDateTimeToISOString(endTime, timezone) : null;
        if (end && new Date(end) <= new Date(start)) {
            toast({
                variant: "destructive",
                title: "Invalid time range",
                description: "End time must be after start time.",
            });
            return;
        }
        try {
            const res = await fetch(`/api/maintenance/${editingIncident.id}`, {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({
                    title,
                    description,
                    status: editingIncident.status, // Keep status unless logic changes it?
                    startTime: start,
                    endTime: end,
                    affectedGroups: editingIncident.affectedGroups || [], // Keep groups for now
                }),
            });
            if (!res.ok) throw new Error("Failed to update");

            toast({ title: "Updated", description: "Maintenance details updated." });
            fetchIncidents();
            setEditingIncident(null);
        } catch (_e) {
            toast({
                variant: "destructive",
                title: "Error",
                description: "Could not update maintenance.",
            });
        }
    };

    // Filter maintenance
    const scheduled = incidents.filter((i) => i.type === "maintenance" && !isMaintenanceFinished(i));
    const history = incidents.filter((i) => i.type === "maintenance" && isMaintenanceFinished(i));

    return (
        <div className="mx-auto max-w-5xl space-y-7">
            <div className="flex items-center justify-between border-b border-border/40 pb-6">
                <div>
                    <h2 className="text-xl font-semibold tracking-tight text-foreground">Maintenance</h2>
                    <p className="text-sm text-muted-foreground mt-1">Scheduled system maintenance and upgrades.</p>
                </div>
            </div>

            <Tabs defaultValue="scheduled" className="w-full">
                <TabsList className="bg-transparent border-b border-border/40 w-full justify-start h-auto p-0 space-x-6 rounded-none">
                    <TabsTrigger
                        value="scheduled"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground transition-colors data-[state=active]:text-foreground"
                    >
                        Scheduled
                        {scheduled.length > 0 && <span className="ml-2 bg-blue-500/10 text-blue-400 text-[10px] px-1.5 py-0.5 rounded-full">{scheduled.length}</span>}
                    </TabsTrigger>
                    <TabsTrigger
                        value="history"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground transition-colors data-[state=active]:text-foreground"
                    >
                        History
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="scheduled" className="mt-6 space-y-3 focus-visible:outline-none focus-visible:ring-0">
                    {scheduled.length === 0 && (
                        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground/60">
                            <Clock3 className="mb-4 h-9 w-9 text-muted-foreground/40" />
                            <p className="text-sm font-medium">No scheduled maintenance</p>
                            <p className="text-xs opacity-70 mt-1">All systems operating normally.</p>
                        </div>
                    )}
                    {scheduled.map((i) => (
                        <MaintenanceCard
                            key={i.id}
                            incident={i}
                            groups={groups}
                            timezone={timezone}
                            onEdit={canEdit ? handleEdit : undefined}
                            onDelete={canEdit ? handleDelete : undefined}
                            onEndNow={canEdit ? handleEndNow : undefined}
                        />
                    ))}
                </TabsContent>

                <TabsContent value="history" className="mt-6 space-y-3 focus-visible:outline-none focus-visible:ring-0">
                    {history.length === 0 && <div className="text-center text-muted-foreground/50 py-16 text-sm">No maintenance history.</div>}
                    {history.map((i) => (
                        <MaintenanceCard key={i.id} incident={i} groups={groups} timezone={timezone} onDelete={canEdit ? handleDelete : undefined} />
                    ))}
                </TabsContent>
            </Tabs>

            {/* Edit Dialog */}
            <Dialog open={!!editingIncident} onOpenChange={(open) => !open && setEditingIncident(null)}>
                <DialogContent>
                    <DialogHeader>
                        <DialogTitle>Edit Maintenance</DialogTitle>
                        <DialogDescription>Update maintenance details.</DialogDescription>
                    </DialogHeader>
                    <div className="grid gap-4 py-4">
                        <div className="grid gap-2">
                            <Label htmlFor="title">Title</Label>
                            <Input id="title" value={title} onChange={(e) => setTitle(e.target.value)} />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="desc">Description</Label>
                            <Textarea id="desc" value={description} onChange={(e) => setDescription(e.target.value)} />
                        </div>
                        <div className="grid grid-cols-2 gap-4">
                            <div className="grid gap-2">
                                <Label htmlFor="start">Start Time</Label>
                                <Input id="start" type="datetime-local" value={startTime} onChange={(e) => setStartTime(e.target.value)} />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="end">End Time</Label>
                                <Input id="end" type="datetime-local" value={endTime} onChange={(e) => setEndTime(e.target.value)} />
                            </div>
                        </div>
                        <p className="text-xs text-muted-foreground">Times use your configured timezone: {timezone}</p>
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setEditingIncident(null)}>
                            Cancel
                        </Button>
                        <Button onClick={saveEdit}>Save Changes</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete Confirmation */}
            <AlertDialog open={!!deletingId} onOpenChange={(open) => !open && setDeletingId(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Delete Maintenance Window?</AlertDialogTitle>
                        <AlertDialogDescription>This action cannot be undone. This will permanently delete the maintenance record.</AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                            Delete
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    );
}
