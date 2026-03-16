import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { Plus, X } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Switch } from "@/components/ui/switch";
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
import { Group, RequestConfig } from "@/lib/store";
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
    const [confirmThreshold, setConfirmThreshold] = useState<string>("");
    const [cooldownMins, setCooldownMins] = useState<string>("");
    const [showAdvanced, setShowAdvanced] = useState(false);
    const [isNewGroup, setIsNewGroup] = useState(false);
    const [newGroupName, setNewGroupName] = useState(""); // For new group input

    // Request Configuration state
    const [httpMethod, setHttpMethod] = useState("GET");
    const [requestTimeout, setRequestTimeout] = useState<string>("");
    const [retryCount, setRetryCount] = useState("0");
    const [followRedirects, setFollowRedirects] = useState(true);
    const [acceptedCodes, setAcceptedCodes] = useState("");
    const [customHeaders, setCustomHeaders] = useState<{ key: string; value: string }[]>([]);
    const [requestBody, setRequestBody] = useState("");

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

            // Build request config if any fields are non-default
            let requestConfig: RequestConfig | undefined;
            const headers: Record<string, string> = {};
            for (const h of customHeaders) {
                if (h.key.trim()) headers[h.key.trim()] = h.value.trim();
            }
            const hasConfig = httpMethod !== "GET" || requestTimeout || parseInt(retryCount) > 0 ||
                !followRedirects || acceptedCodes || Object.keys(headers).length > 0 || requestBody;
            if (hasConfig) {
                requestConfig = {};
                if (httpMethod !== "GET") requestConfig.method = httpMethod;
                if (requestTimeout) requestConfig.timeoutSeconds = parseInt(requestTimeout);
                if (parseInt(retryCount) > 0) requestConfig.retryCount = parseInt(retryCount);
                if (!followRedirects) requestConfig.followRedirects = false;
                if (acceptedCodes) requestConfig.acceptedStatusCodes = acceptedCodes;
                if (Object.keys(headers).length > 0) requestConfig.headers = headers;
                if (requestBody) requestConfig.body = requestBody;
            }

            await createMonitor.mutateAsync({
                name,
                url,
                groupId: finalGroupId,
                interval,
                confirmationThreshold: confirmThreshold ? parseInt(confirmThreshold) : undefined,
                notificationCooldownMinutes: cooldownMins ? parseInt(cooldownMins) : undefined,
                requestConfig,
            });

            toast({ title: "Monitor Created", description: `Monitor "${name}" active and checking.` });

            // Reset
            setName("");
            setUrl("");
            setNewGroupName("");
            setSelectedGroupId("");
            setInterval(60);
            setConfirmThreshold("");
            setCooldownMins("");
            setShowAdvanced(false);
            setIsNewGroup(false);
            setHttpMethod("GET");
            setRequestTimeout("");
            setRetryCount("0");
            setFollowRedirects(true);
            setAcceptedCodes("");
            setCustomHeaders([]);
            setRequestBody("");
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
            <SheetContent className="sm:max-w-[500px] overflow-y-auto">
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
                    <div className="grid gap-2">
                        <button
                            type="button"
                            onClick={() => setShowAdvanced(!showAdvanced)}
                            className="text-sm text-muted-foreground hover:text-foreground transition-colors text-left"
                        >
                            {showAdvanced ? "- Hide Advanced" : "+ Advanced Settings"}
                        </button>
                        {showAdvanced && (
                            <div className="space-y-4 p-3 rounded-lg border border-border bg-muted/30">
                                <div className="grid grid-cols-2 gap-4">
                                    <div className="grid gap-1.5">
                                        <Label htmlFor="create-confirm" className="text-xs">Confirmation Checks</Label>
                                        <Input
                                            id="create-confirm"
                                            type="number"
                                            min={1}
                                            max={100}
                                            placeholder="Global default"
                                            value={confirmThreshold}
                                            onChange={(e) => setConfirmThreshold(e.target.value)}
                                        />
                                    </div>
                                    <div className="grid gap-1.5">
                                        <Label htmlFor="create-cooldown" className="text-xs">Cooldown (min)</Label>
                                        <Input
                                            id="create-cooldown"
                                            type="number"
                                            min={0}
                                            max={1440}
                                            placeholder="Global default"
                                            value={cooldownMins}
                                            onChange={(e) => setCooldownMins(e.target.value)}
                                        />
                                    </div>
                                    <p className="col-span-2 text-xs text-muted-foreground">
                                        Override global notification settings for this monitor. Leave empty to use global defaults.
                                    </p>
                                </div>

                                <div className="border-t border-border pt-3 space-y-3">
                                    <h4 className="text-xs font-medium text-muted-foreground uppercase tracking-wider">Request Configuration</h4>
                                    <div className="grid grid-cols-2 gap-4">
                                        <div className="grid gap-1.5">
                                            <Label className="text-xs">HTTP Method</Label>
                                            <Select onValueChange={setHttpMethod} value={httpMethod}>
                                                <SelectTrigger data-testid="request-method-select"><SelectValue /></SelectTrigger>
                                                <SelectContent>
                                                    {["GET", "HEAD", "POST", "PUT", "DELETE"].map(m => (
                                                        <SelectItem key={m} value={m} className="cursor-pointer">{m}</SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                        </div>
                                        <div className="grid gap-1.5">
                                            <Label className="text-xs">Request Timeout (s)</Label>
                                            <Input
                                                type="number"
                                                min={1}
                                                max={120}
                                                placeholder="5"
                                                value={requestTimeout}
                                                onChange={(e) => setRequestTimeout(e.target.value)}
                                            />
                                        </div>
                                        <div className="grid gap-1.5">
                                            <Label className="text-xs">Retry on Failure</Label>
                                            <Select onValueChange={setRetryCount} value={retryCount}>
                                                <SelectTrigger data-testid="request-retry-select"><SelectValue /></SelectTrigger>
                                                <SelectContent>
                                                    {[0, 1, 2, 3, 4, 5].map(n => (
                                                        <SelectItem key={n} value={n.toString()} className="cursor-pointer">
                                                            {n === 0 ? "No retry" : `${n} ${n === 1 ? "retry" : "retries"}`}
                                                        </SelectItem>
                                                    ))}
                                                </SelectContent>
                                            </Select>
                                        </div>
                                        <div className="grid gap-1.5">
                                            <Label className="text-xs">Accepted Status Codes</Label>
                                            <Input
                                                placeholder="200-399"
                                                value={acceptedCodes}
                                                onChange={(e) => setAcceptedCodes(e.target.value)}
                                            />
                                        </div>
                                    </div>
                                    <div className="flex items-center justify-between">
                                        <Label className="text-xs">Follow Redirects</Label>
                                        <Switch checked={followRedirects} onCheckedChange={setFollowRedirects} />
                                    </div>
                                    <div className="grid gap-1.5">
                                        <div className="flex items-center justify-between">
                                            <Label className="text-xs">Custom Headers</Label>
                                            <Button
                                                type="button"
                                                variant="ghost"
                                                size="sm"
                                                className="h-6 text-xs"
                                                onClick={() => setCustomHeaders([...customHeaders, { key: "", value: "" }])}
                                            >
                                                + Add
                                            </Button>
                                        </div>
                                        {customHeaders.map((h, i) => (
                                            <div key={i} className="flex gap-2 items-center">
                                                <Input
                                                    placeholder="Header name"
                                                    value={h.key}
                                                    onChange={(e) => {
                                                        const next = [...customHeaders];
                                                        next[i] = { ...next[i], key: e.target.value };
                                                        setCustomHeaders(next);
                                                    }}
                                                    className="text-xs"
                                                />
                                                <Input
                                                    placeholder="Value"
                                                    value={h.value}
                                                    onChange={(e) => {
                                                        const next = [...customHeaders];
                                                        next[i] = { ...next[i], value: e.target.value };
                                                        setCustomHeaders(next);
                                                    }}
                                                    className="text-xs"
                                                />
                                                <Button
                                                    type="button"
                                                    variant="ghost"
                                                    size="sm"
                                                    className="h-8 w-8 p-0 shrink-0"
                                                    onClick={() => setCustomHeaders(customHeaders.filter((_, j) => j !== i))}
                                                >
                                                    <X className="w-3 h-3" />
                                                </Button>
                                            </div>
                                        ))}
                                    </div>
                                    {(httpMethod === "POST" || httpMethod === "PUT") && (
                                        <div className="grid gap-1.5">
                                            <Label className="text-xs">Request Body</Label>
                                            <Textarea
                                                placeholder='{"status": "ok"}'
                                                value={requestBody}
                                                onChange={(e) => setRequestBody(e.target.value)}
                                                className="font-mono text-xs min-h-[60px]"
                                            />
                                        </div>
                                    )}
                                </div>
                            </div>
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
