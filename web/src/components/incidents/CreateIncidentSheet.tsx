import { useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea"; // Need to ensure Textarea exists, or use Input
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
    SheetClose,
} from "@/components/ui/sheet";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Incident } from "@/lib/store";

interface CreateIncidentSheetProps {
    onCreate: (incident: Omit<Incident, 'id'>) => void;
    groups: string[];
}

export function CreateIncidentSheet({ onCreate, groups }: CreateIncidentSheetProps) {
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [type, setType] = useState<Incident['type']>("incident");
    const [severity, setSeverity] = useState<Incident['severity']>("major");
    const [status, setStatus] = useState<Incident['status']>("investigating");
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        onCreate({
            title,
            description,
            type,
            severity,
            status: type === 'maintenance' ? 'scheduled' : status,
            startTime: new Date().toISOString(), // Simplified for demo
            affectedGroups: ["Platform Core"] // Simplified
        });

        setOpen(false);
        setTitle("");
        setDescription("");
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button variant="destructive" className="gap-2">
                    <Plus className="w-4 h-4" /> Report Incident
                </Button>
            </SheetTrigger>
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="text-slate-100">Report Incident or Maintenance</SheetTitle>
                    <SheetDescription className="text-slate-400">
                        Create a status update for your users.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label>Title</Label>
                        <Input value={title} onChange={e => setTitle(e.target.value)} required
                            className="bg-slate-900 border-slate-800" placeholder="e.g. API Downtime" />
                    </div>

                    <div className="grid grid-cols-2 gap-4">
                        <div className="grid gap-2">
                            <Label>Type</Label>
                            <Select value={type} onValueChange={(v: any) => setType(v)}>
                                <SelectTrigger className="bg-slate-900 border-slate-800">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="incident">Incident</SelectItem>
                                    <SelectItem value="maintenance">Maintenance</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="grid gap-2">
                            <Label>Severity</Label>
                            <Select value={severity} onValueChange={(v: any) => setSeverity(v)}>
                                <SelectTrigger className="bg-slate-900 border-slate-800">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="minor">Minor</SelectItem>
                                    <SelectItem value="major">Major</SelectItem>
                                    <SelectItem value="critical">Critical</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>
                    </div>

                    <div className="grid gap-2">
                        <Label>Description</Label>
                        <Input value={description} onChange={e => setDescription(e.target.value)}
                            className="bg-slate-900 border-slate-800" placeholder="Details..." />
                    </div>

                    <SheetFooter className="mt-4">
                        <Button type="submit">Create</Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
