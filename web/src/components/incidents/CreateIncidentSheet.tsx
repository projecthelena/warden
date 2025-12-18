import { useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
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
    const [selectedGroup, setSelectedGroup] = useState<string>("");
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [type] = useState<Incident['type']>("incident");
    const [severity, setSeverity] = useState<Incident['severity']>("major");
    const [status] = useState<Incident['status']>("investigating");
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!selectedGroup) {
            alert("Please select an affected group");
            return;
        }

        onCreate({
            title,
            description,
            type,
            severity,
            status,
            startTime: new Date().toISOString(),
            affectedGroups: [selectedGroup]
        });

        setOpen(false);
        setTitle("");
        setDescription("");
        setSelectedGroup("");
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button size="sm" className="gap-2">
                    <Plus className="w-4 h-4" /> Report Incident
                </Button>
            </SheetTrigger>
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="text-slate-100">Report Incident</SheetTitle>
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
                            <Label>Affected Group</Label>
                            <Select value={selectedGroup} onValueChange={setSelectedGroup}>
                                <SelectTrigger className="bg-slate-900 border-slate-800">
                                    <SelectValue placeholder="Select Group" />
                                </SelectTrigger>
                                <SelectContent>
                                    {groups.map(g => (
                                        <SelectItem key={g} value={g}>{g}</SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                        <div className="grid gap-2">
                            <Label>Severity</Label>
                            <Select value={severity} onValueChange={(v: Incident['severity']) => setSeverity(v)}>
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
                        <Button type="submit">Create Incident</Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
