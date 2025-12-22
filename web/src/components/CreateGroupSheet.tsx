import { useState } from "react";
import { useNavigate } from "react-router-dom";
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
    onCreate: (name: string) => Promise<string | undefined>;
}

export function CreateGroupSheet({ onCreate }: CreateGroupSheetProps) {
    const [name, setName] = useState("");
    const [open, setOpen] = useState(false);

    const navigate = useNavigate();

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name) return;

        const newId = await onCreate(name);
        setName("");
        setOpen(false);

        if (newId) {
            navigate(`/groups/${newId}`);
        }
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button variant="outline" size="sm" className="gap-2" data-testid="create-group-trigger">
                    <FolderPlus className="w-4 h-4" /> New Group
                </Button>
            </SheetTrigger>
            <SheetContent className="sm:max-w-[400px]">
                <SheetHeader>
                    <SheetTitle>Create New Group</SheetTitle>
                    <SheetDescription>
                        Organize your monitors into a new dashboard group.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label htmlFor="group-name">Group Name</Label>
                        <Input
                            id="group-name"
                            placeholder="e.g. Mobile Backend"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            required
                            data-testid="create-group-name-input"
                        />
                    </div>
                    <SheetFooter className="mt-4">
                        <SheetClose asChild>
                            <Button variant="outline" className="mr-2">Cancel</Button>
                        </SheetClose>
                        <Button type="submit" data-testid="create-group-submit-btn">
                            Create Group
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
