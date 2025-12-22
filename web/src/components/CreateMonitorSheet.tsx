import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { Plus } from "lucide-react";
import { cn } from "@/lib/utils";
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
import { Group } from "@/lib/store";
import { useCreateGroupMutation, useCreateMonitorMutation } from "@/hooks/useMonitors";
import { useToast } from "@/components/ui/use-toast";

interface CreateMonitorSheetProps {
    groups: Group[];
    defaultGroup?: string;
}

export function CreateMonitorSheet({ groups, defaultGroup }: CreateMonitorSheetProps) {
    const [name, setName] = useState("");
    const [url, setUrl] = useState("");
    // Removed unused groupName state

    const [selectedGroupId, setSelectedGroupId] = useState<string>("");
    const [urlError, setUrlError] = useState(false);
    const [nameError, setNameError] = useState(false);
    const [groupError, setGroupError] = useState(false);

    // Sync group state when defaultGroup changes
    useEffect(() => {
        if (defaultGroup) {
            // Find ID for the default group name
            const found = groups.find(g => g.name === defaultGroup);
            if (found) setSelectedGroupId(found.id);
        }
    }, [defaultGroup, groups]);

    const [interval, setInterval] = useState(60);
    const [isNewGroup, setIsNewGroup] = useState(false);
    const [newGroupName, setNewGroupName] = useState(""); // For new group input

    const [open, setOpen] = useState(false);

    const createGroup = useCreateGroupMutation();
    const createMonitor = useCreateMonitorMutation();
    const { toast } = useToast();
    const navigate = useNavigate();

    // Clear error on change if needed
    useEffect(() => {
        if (urlError) setUrlError(false);
    }, [url, urlError]);

    useEffect(() => {
        if (nameError) setNameError(false);
    }, [name, nameError]);

    useEffect(() => {
        if (groupError) setGroupError(false);
    }, [selectedGroupId, newGroupName, isNewGroup, groupError]);

    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        if (!name || !url) return;

        let finalGroupId = selectedGroupId;

        try {
            // Client-side Validation
            // 1. Validate URL
            try {
                new URL(url);
            } catch {
                setUrlError(true);
                toast({ title: "Invalid URL", description: "Please enter a valid URL (e.g. https://example.com)", variant: "destructive" });
                return;
            }

            // 2. Validate Duplicate Name (in loaded groups)
            // Flatten all monitors
            const allMonitors = groups.flatMap(g => g.monitors);
            if (allMonitors.some(m => m.name.toLowerCase() === name.toLowerCase())) {
                setNameError(true);
                toast({ title: "Duplicate Name", description: "A monitor with this name already exists.", variant: "destructive" });
                return;
            }

            if (isNewGroup) {
                if (!newGroupName) {
                    setGroupError(true);
                    toast({ title: "Error", description: "Group name is required", variant: "destructive" });
                    return;
                }
                const newGroup = await createGroup.mutateAsync(newGroupName);
                finalGroupId = newGroup.id;
            } else {
                if (!finalGroupId) {
                    // Fallback to default group or error?
                    // If simple setup, maybe they select nothing?
                    // Let's enforce selection.
                    if (groups.length > 0) {
                        setGroupError(true);
                        toast({ title: "Error", description: "Please select a group", variant: "destructive" });
                        return;
                    }
                    // If no groups exist and proper UI didn't force creation, create default
                    const def = await createGroup.mutateAsync("Default");
                    finalGroupId = def.id;
                }
            }

            await createMonitor.mutateAsync({
                name,
                url,
                groupId: finalGroupId,
                interval
            });

            toast({ title: "Monitor Created", description: `Monitor "${name}" active and checking.` });

            // Reset
            setName("");
            setUrl("");
            setNewGroupName("");
            setSelectedGroupId("");
            setInterval(60);
            setIsNewGroup(false);
            setOpen(false);

            // Redirect to the group page
            if (finalGroupId) {
                navigate(`/groups/${finalGroupId}`);
            }

        } catch (err) {
            console.error(err);
            toast({ title: "Error", description: "Failed to create monitor", variant: "destructive" });
        }
    };

    const handleGroupChange = (value: string) => {
        if (value === "___create_new___") {
            setIsNewGroup(true);
            setSelectedGroupId("");
        } else {
            setSelectedGroupId(value);
            setIsNewGroup(false); // Ensure we switch back if they picked from list
        }
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button className="gap-2" size="sm" data-testid="create-monitor-trigger">
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
                            className={cn(nameError && "border-red-500 focus-visible:ring-red-500")}
                            required
                            data-testid="create-monitor-name-input"
                        />
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="url">Target URL</Label>
                        <Input
                            id="url"
                            placeholder="https://api.example.com/health"
                            className={cn("font-mono text-sm", urlError && "border-red-500 focus-visible:ring-red-500")}
                            value={url}
                            onChange={(e) => setUrl(e.target.value)}
                            required
                            data-testid="create-monitor-url-input"
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
                                    value={newGroupName}
                                    onChange={(e) => setNewGroupName(e.target.value)}
                                    className={cn(groupError && "border-red-500 focus-visible:ring-red-500")}
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
                            <Select onValueChange={handleGroupChange} value={selectedGroupId}>
                                <SelectTrigger className={cn(groupError && "border-red-500 focus:ring-red-500")} data-testid="create-monitor-group-select">
                                    <SelectValue placeholder="Select a group" />
                                </SelectTrigger>
                                <SelectContent>
                                    {groups.length > 0 ? (
                                        groups.map((g) => (
                                            <SelectItem key={g.id} value={g.id} className="cursor-pointer">
                                                {g.name}
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
                        <Button type="submit" disabled={createMonitor.isPending || createGroup.isPending} data-testid="create-monitor-submit-btn">
                            {createMonitor.isPending ? "Creating..." : "Create Monitor"}
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
