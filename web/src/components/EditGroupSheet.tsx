import { useState, useEffect } from "react";
import { FolderCog } from "lucide-react";
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
} from "@/components/ui/sheet";
import { Group } from "@/lib/store";

interface EditGroupSheetProps {
    group: Group | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
    onSave: (id: string, name: string) => void;
}

export function EditGroupSheet({ group, open, onOpenChange, onSave }: EditGroupSheetProps) {
    const [name, setName] = useState("");

    useEffect(() => {
        if (group) {
            setName(group.name);
        }
    }, [group]);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!group || !name) return;

        onSave(group.id, name);
        onOpenChange(false);
    };

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="sm:max-w-[400px]">
                <SheetHeader>
                    <SheetTitle className="flex items-center gap-2">
                        <FolderCog className="w-5 h-5 text-primary" />
                        Edit Group
                    </SheetTitle>
                    <SheetDescription>
                        Update the details for this monitor group.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label htmlFor="edit-group-name">Group Name</Label>
                        <Input
                            id="edit-group-name"
                            placeholder="e.g. Mobile Backend"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            required
                        />
                    </div>
                    <SheetFooter className="mt-4">
                        <Button type="button" variant="outline" onClick={() => onOpenChange(false)} className="mr-2">
                            Cancel
                        </Button>
                        <Button type="submit">
                            Save Changes
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
