
import { useEffect } from "react";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Badge } from "@/components/ui/badge";
import { useMonitorStore, Incident } from "@/lib/store";
import { Calendar, CheckCircle2, MoreVertical, Pencil, Trash2, XCircle } from "lucide-react";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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
import { useState } from "react";
import { useToast } from "@/components/ui/use-toast";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog";
// Select imports? Maybe simpler native select or Shadcn Select. Using native for speed/stability if Select not imported.
// Actually let's assume Select is available or use native. User prefers vanilla Shadcn.
// Importing Select components
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";



import { Group } from "@/lib/store";
import { formatDate } from "@/lib/utils";

import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";

function MaintenanceCard({ incident, groups, onEdit, onDelete, onEndNow }: { incident: Incident; groups: Group[], onEdit: (i: Incident) => void, onDelete: (id: string) => void, onEndNow: (i: Incident) => void }) {
    const affectedGroupNames = incident.affectedGroups?.map(id => {
        const g = groups.find(group => group.id === id);
        return g ? g.name : id;
    }) || [];

    const now = new Date();
    const start = new Date(incident.startTime);
    const end = incident.endTime ? new Date(incident.endTime) : null;
    const isOngoing = now >= start && (!end || now < end);
    const isHistory = incident.status === 'completed' || incident.status === 'resolved' || (end && now > end);

    return (
        <Card className="hover:shadow-md transition-all duration-200 border-border/40 bg-card/30 hover:bg-card/50">
            <CardHeader className="pb-3 grid grid-cols-[1fr_auto] gap-4 space-y-0">
                <div className="space-y-1.5">
                    <div className="flex items-center gap-2">
                        <CardTitle className="text-base font-medium leading-none">{incident.title}</CardTitle>
                        {isOngoing ? (
                            <Badge variant="secondary" className="bg-emerald-500/10 text-emerald-500 hover:bg-emerald-500/20 border-0 px-2 py-0 h-5 text-[10px] animate-pulse">
                                Ongoing
                            </Badge>
                        ) : (
                            <Badge variant="secondary" className="bg-blue-500/10 text-blue-500 hover:bg-blue-500/20 border-0 px-2 py-0 h-5 text-[10px]">
                                Scheduled
                            </Badge>
                        )}
                    </div>
                    <CardDescription className="text-sm">{incident.description}</CardDescription>
                </div>
                <div className="flex items-start gap-4">
                    <div className="flex items-center gap-2 text-xs text-muted-foreground font-mono mt-0.5">
                        <Calendar className="w-3.5 h-3.5" />
                        <span>{formatDate(incident.startTime)}</span>
                        {incident.endTime && (
                            <>
                                <span>-</span>
                                <span>{formatDate(incident.endTime)}</span>
                            </>
                        )}
                    </div>

                    {!isHistory && (
                        <DropdownMenu>
                            <DropdownMenuTrigger asChild>
                                <Button variant="ghost" className="h-8 w-8 p-0">
                                    <span className="sr-only">Open menu</span>
                                    <MoreVertical className="h-4 w-4" />
                                </Button>
                            </DropdownMenuTrigger>
                            <DropdownMenuContent align="end">
                                <DropdownMenuLabel>Actions</DropdownMenuLabel>
                                <DropdownMenuItem onClick={() => onEdit(incident)}>
                                    <Pencil className="mr-2 h-4 w-4" />
                                    Edit Details
                                </DropdownMenuItem>
                                {isOngoing && (
                                    <DropdownMenuItem onClick={() => onEndNow(incident)}>
                                        <CheckCircle2 className="mr-2 h-4 w-4" />
                                        End Now
                                    </DropdownMenuItem>
                                )}
                                <DropdownMenuSeparator />
                                <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={() => onDelete(incident.id)}>
                                    <Trash2 className="mr-2 h-4 w-4" />
                                    Delete
                                </DropdownMenuItem>
                            </DropdownMenuContent>
                        </DropdownMenu>
                    )}
                </div>
            </CardHeader>
            <CardContent>
                <div className="pt-3 border-t border-border/30 flex items-center justify-between">
                    <div className="flex items-center gap-2 text-sm">
                        <span className="text-muted-foreground text-xs font-medium uppercase tracking-wider">Affected Groups:</span>
                        <div className="flex flex-wrap gap-1.5">
                            {affectedGroupNames.length > 0 ? affectedGroupNames.map((name, i) => (
                                <Badge key={i} variant="outline" className="font-normal text-xs text-muted-foreground bg-background/50">
                                    {name}
                                </Badge>
                            )) : (
                                <span className="text-muted-foreground text-xs italic">All Groups</span>
                            )}
                        </div>
                    </div>
                    {incident.status && (
                        <span className="text-[10px] uppercase tracking-wider font-mono text-muted-foreground/40">
                            {incident.status.replace('_', ' ')}
                        </span>
                    )}
                </div>
            </CardContent>
        </Card>
    )
}

export function MaintenanceView() {
    const { incidents, groups, fetchIncidents } = useMonitorStore();
    const { toast } = useToast();
    const [editingIncident, setEditingIncident] = useState<Incident | null>(null);
    const [deletingId, setDeletingId] = useState<string | null>(null);

    // Edit Form State
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [startTime, setStartTime] = useState("");
    const [endTime, setEndTime] = useState("");
    const [selectedGroups, setSelectedGroups] = useState<string[]>([]); // Simple Multi-select? Or single? API supports array.
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
        // Format for datetime-local: YYYY-MM-DDTHH:mm
        const toLocalISO = (d: string) => {
            const date = new Date(d);
            date.setMinutes(date.getMinutes() - date.getTimezoneOffset());
            return date.toISOString().slice(0, 16);
        };
        setStartTime(toLocalISO(i.startTime));
        setEndTime(i.endTime ? toLocalISO(i.endTime) : "");
    };

    const handleDelete = (id: string) => {
        setDeletingId(id);
    };

    const handleEndNow = async (i: Incident) => {
        try {
            // Update end time to now
            const res = await fetch(`/api/maintenance/${i.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    title: i.title,
                    description: i.description,
                    status: 'completed', // Or resolved?
                    startTime: i.startTime, // Keep original start
                    endTime: new Date().toISOString(),
                    affectedGroups: i.affectedGroups || []
                })
            });
            if (!res.ok) throw new Error("Failed to end maintenance");

            toast({ title: "Maintenance Ended", description: "The maintenance window has been closed." });
            fetchIncidents();
        } catch (e) {
            toast({ variant: "destructive", title: "Error", description: "Could not end maintenance." });
        }
    };

    const confirmDelete = async () => {
        if (!deletingId) return;
        try {
            const res = await fetch(`/api/maintenance/${deletingId}`, { method: 'DELETE' });
            if (!res.ok) throw new Error("Failed to delete");
            toast({ title: "Deleted", description: "Maintenance window deleted." });
            fetchIncidents();
        } catch (e) {
            toast({ variant: "destructive", title: "Error", description: "Could not delete maintenance." });
        } finally {
            setDeletingId(null);
        }
    };

    const saveEdit = async () => {
        if (!editingIncident) return;
        try {
            const res = await fetch(`/api/maintenance/${editingIncident.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    title,
                    description,
                    status: editingIncident.status, // Keep status unless logic changes it?
                    startTime: new Date(startTime).toISOString(),
                    endTime: endTime ? new Date(endTime).toISOString() : null,
                    affectedGroups: editingIncident.affectedGroups || [] // Keep groups for now
                })
            });
            if (!res.ok) throw new Error("Failed to update");

            toast({ title: "Updated", description: "Maintenance details updated." });
            fetchIncidents();
            setEditingIncident(null);
        } catch (e) {
            toast({ variant: "destructive", title: "Error", description: "Could not update maintenance." });
        }
    };

    // Filter maintenance
    const scheduled = incidents.filter(i => i.type === 'maintenance' && i.status !== 'completed');
    const history = incidents.filter(i => i.type === 'maintenance' && i.status === 'completed');

    return (
        <div className="space-y-8 max-w-5xl mx-auto">
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
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground data-[state=active]:text-foreground transition-all"
                    >
                        Scheduled
                        {scheduled.length > 0 && <span className="ml-2 bg-blue-500/10 text-blue-400 text-[10px] px-1.5 py-0.5 rounded-full">{scheduled.length}</span>}
                    </TabsTrigger>
                    <TabsTrigger
                        value="history"
                        className="rounded-none border-b-2 border-transparent data-[state=active]:border-foreground data-[state=active]:bg-transparent px-0 py-2 text-sm font-medium text-muted-foreground data-[state=active]:text-foreground transition-all"
                    >
                        History
                    </TabsTrigger>
                </TabsList>

                <TabsContent value="scheduled" className="mt-8 space-y-4 focus-visible:outline-none focus-visible:ring-0">
                    {scheduled.length === 0 && (
                        <div className="flex flex-col items-center justify-center py-16 text-muted-foreground/60">
                            <CheckCircle2 className="w-10 h-10 mb-4 text-emerald-500/30" />
                            <p className="text-sm font-medium">No scheduled maintenance</p>
                            <p className="text-xs opacity-70 mt-1">All systems operating normally.</p>
                        </div>
                    )}
                    {scheduled.map(i => <MaintenanceCard key={i.id} incident={i} groups={groups} onEdit={handleEdit} onDelete={handleDelete} onEndNow={handleEndNow} />)}
                </TabsContent>

                <TabsContent value="history" className="mt-8 space-y-4 focus-visible:outline-none focus-visible:ring-0">
                    {history.length === 0 && (
                        <div className="text-center text-muted-foreground/50 py-16 text-sm">No maintenance history.</div>
                    )}
                    {history.map(i => <MaintenanceCard key={i.id} incident={i} groups={groups} onEdit={handleEdit} onDelete={handleDelete} onEndNow={handleEndNow} />)}
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
                    </div>
                    <DialogFooter>
                        <Button variant="outline" onClick={() => setEditingIncident(null)}>Cancel</Button>
                        <Button onClick={saveEdit}>Save Changes</Button>
                    </DialogFooter>
                </DialogContent>
            </Dialog>

            {/* Delete Confirmation */}
            <AlertDialog open={!!deletingId} onOpenChange={(open) => !open && setDeletingId(null)}>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Delete Maintenance Window?</AlertDialogTitle>
                        <AlertDialogDescription>
                            This action cannot be undone. This will permanently delete the maintenance record.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction onClick={confirmDelete} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">Delete</AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    )
}
