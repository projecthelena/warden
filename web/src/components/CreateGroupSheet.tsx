import { useState } from "react";
import { FolderPlus } from "lucide-react";
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

interface CreateGroupSheetProps {
    onCreate: (name: string) => void;
}

export function CreateGroupSheet({ onCreate }: CreateGroupSheetProps) {
    const [name, setName] = useState("");
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!name) return;

        onCreate(name);
        setName("");
        setOpen(false);
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button variant="outline" size="sm" className="gap-2 border-slate-700 bg-slate-900/50 hover:bg-slate-800 text-slate-300">
                    <FolderPlus className="w-4 h-4" /> New Group
                </Button>
            </SheetTrigger>
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[400px]">
                <SheetHeader>
                    <SheetTitle className="text-slate-100">Create New Group</SheetTitle>
                    <SheetDescription className="text-slate-400">
                        Organize your monitors into a new dashboard group.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label htmlFor="group-name" className="text-slate-200">Group Name</Label>
                        <Input
                            id="group-name"
                            placeholder="e.g. Mobile Backend"
                            className="bg-slate-900 border-slate-800 focus-visible:ring-blue-600"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            required
                        />
                    </div>
                    <SheetFooter className="mt-4">
                        <SheetClose asChild>
                            <Button variant="outline" className="border-slate-800 text-slate-400 hover:text-slate-100 mr-2">Cancel</Button>
                        </SheetClose>
                        <Button type="submit" className="bg-blue-600 hover:bg-blue-500 text-white">
                            Create Group
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
