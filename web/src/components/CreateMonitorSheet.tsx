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
    SheetClose,
} from "@/components/ui/sheet";

interface CreateMonitorSheetProps {
    onCreate: (name: string, url: string, group: string) => void;
    groups: string[];
}

export function CreateMonitorSheet({ onCreate, groups }: CreateMonitorSheetProps) {
    const [name, setName] = useState("");
    const [url, setUrl] = useState("");
    const [group, setGroup] = useState("");
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!name || !url) return;

        onCreate(name, url, group);

        // Reset
        setName("");
        setUrl("");
        setGroup("");
        setOpen(false);
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button className="gap-2">
                    <Plus className="w-4 h-4" /> New Monitor
                </Button>
            </SheetTrigger>
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="text-slate-100">Add New Monitor</SheetTitle>
                    <SheetDescription className="text-slate-400">
                        Configure a new endpoint to monitor. You can assign it to an existing group or create a new one.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label htmlFor="name" className="text-slate-200">Display Name</Label>
                        <Input
                            id="name"
                            placeholder="e.g. Production API"
                            className="bg-slate-900 border-slate-800 focus-visible:ring-blue-600"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            required
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="url" className="text-slate-200">Target URL</Label>
                        <Input
                            id="url"
                            placeholder="https://api.example.com/health"
                            className="bg-slate-900 border-slate-800 focus-visible:ring-blue-600 font-mono text-sm"
                            value={url}
                            onChange={(e) => setUrl(e.target.value)}
                            required
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="group" className="text-slate-200">Group / Project</Label>
                        <Input
                            id="group"
                            placeholder="Default"
                            list="existing-groups"
                            className="bg-slate-900 border-slate-800 focus-visible:ring-blue-600"
                            value={group}
                            onChange={(e) => setGroup(e.target.value)}
                        />
                        <datalist id="existing-groups">
                            {groups.map(g => <option key={g} value={g} />)}
                        </datalist>
                        <p className="text-[0.8rem] text-slate-500">
                            Leave empty to use "Default". Typing a new name will create a new group.
                        </p>
                    </div>
                    <SheetFooter className="mt-4">
                        <SheetClose asChild>
                            <Button variant="outline" className="border-slate-800 text-slate-400 hover:text-slate-100 mr-2">Cancel</Button>
                        </SheetClose>
                        <Button type="submit">
                            Create Monitor
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
