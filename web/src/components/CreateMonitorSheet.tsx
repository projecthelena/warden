import { useState, useEffect } from "react";
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

import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
// ... existing imports

interface CreateMonitorSheetProps {
    onCreate: (name: string, url: string, group: string, interval: number) => void;
    groups: string[];
    defaultGroup?: string;
}

export function CreateMonitorSheet({ onCreate, groups, defaultGroup }: CreateMonitorSheetProps) {
    const [name, setName] = useState("");
    const [url, setUrl] = useState("");
    const [group, setGroup] = useState(defaultGroup || "");

    // Sync group state when defaultGroup changes
    useEffect(() => {
        if (defaultGroup) {
            setGroup(defaultGroup);
        }
    }, [defaultGroup]);
    const [interval, setInterval] = useState(60);
    const [isNewGroup, setIsNewGroup] = useState(false);
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        if (!name || !url) return;
        onCreate(name, url, group, interval);
        // Reset
        setName("");
        setUrl("");
        setGroup("");
        setInterval(60);
        setIsNewGroup(false);
        setOpen(false);
    };

    const handleGroupChange = (value: string) => {
        if (value === "___create_new___") {
            setIsNewGroup(true);
            setGroup("");
        } else {
            setGroup(value);
        }
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button className="gap-2" size="sm">
                    <Plus className="w-4 h-4" /> New Monitor
                </Button>
            </SheetTrigger>
            <SheetContent className="sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle>Add New Monitor</SheetTitle>
                    <SheetDescription>
                        Configure a new endpoint to monitor. You can assign it to an existing group or create a new one.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label htmlFor="name">Display Name</Label>
                        <Input
                            id="name"
                            placeholder="e.g. Production API"
                            value={name}
                            onChange={(e) => setName(e.target.value)}
                            required
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="url">Target URL</Label>
                        <Input
                            id="url"
                            placeholder="https://api.example.com/health"
                            className="font-mono text-sm"
                            value={url}
                            onChange={(e) => setUrl(e.target.value)}
                            required
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="interval">Check Frequency</Label>
                        <Select onValueChange={(v) => setInterval(Number(v))} value={interval.toString()}>
                            <SelectTrigger>
                                <SelectValue placeholder="Select frequency" />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="10" className="cursor-pointer">10 Seconds</SelectItem>
                                <SelectItem value="30" className="cursor-pointer">30 Seconds</SelectItem>
                                <SelectItem value="60" className="cursor-pointer">1 Minute</SelectItem>
                                <SelectItem value="300" className="cursor-pointer">5 Minutes</SelectItem>
                                <SelectItem value="600" className="cursor-pointer">10 Minutes</SelectItem>
                            </SelectContent>
                        </Select>
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="group">Group / Project</Label>
                        {isNewGroup ? (
                            <div className="flex gap-2">
                                <Input
                                    id="group"
                                    placeholder="Enter new group name"
                                    value={group}
                                    onChange={(e) => setGroup(e.target.value)}
                                    autoFocus
                                />
                                <Button
                                    type="button"
                                    variant="outline"
                                    onClick={() => setIsNewGroup(false)}
                                >
                                    Cancel
                                </Button>
                            </div>
                        ) : (
                            <Select onValueChange={handleGroupChange} value={group}>
                                <SelectTrigger>
                                    <SelectValue placeholder="Select a group" />
                                </SelectTrigger>
                                <SelectContent>
                                    {groups.length > 0 ? (
                                        groups.map((g) => (
                                            <SelectItem key={g} value={g} className="cursor-pointer">
                                                {g}
                                            </SelectItem>
                                        ))
                                    ) : (
                                        <SelectItem value="default" disabled className="text-muted-foreground">
                                            No groups found
                                        </SelectItem>
                                    )}
                                    <div className="h-px bg-border my-1" />
                                    <SelectItem value="___create_new___" className="text-blue-500 cursor-pointer">
                                        + Create New Group
                                    </SelectItem>
                                </SelectContent>
                            </Select>
                        )}
                        {!isNewGroup && (
                            <p className="text-[0.8rem] text-muted-foreground">
                                Select an existing group or create a new one.
                            </p>
                        )}
                    </div>
                    <SheetFooter className="mt-4">
                        <SheetClose asChild>
                            <Button variant="outline" className="mr-2">Cancel</Button>
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
