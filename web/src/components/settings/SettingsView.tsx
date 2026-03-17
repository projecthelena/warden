import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { SystemTab } from "./SystemTab";
import { SSOSettings } from "./SSOSettings";
import { APIKeysView } from "./APIKeysView";

import { NotificationsView } from "@/components/notifications/NotificationsView";
import { SelectTimezone } from "@/components/ui/select-timezone";

import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "@/components/ui/tooltip";
import { Info, Monitor, Moon, Sun } from "lucide-react";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useState, useEffect } from "react";
import { useSearchParams } from "react-router-dom";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";
import { useTheme } from "@/hooks/use-theme";

import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from "@/components/ui/alert-dialog"

function ResetDatabaseDialog() {
    const { resetDatabase } = useMonitorStore();
    const { toast } = useToast();
    const [open, setOpen] = useState(false);

    const handleReset = async () => {
        const success = await resetDatabase();
        setOpen(false);
        if (success) {
            toast({ title: "System Reset", description: "Database has been reset. Redirecting..." });
            window.location.reload();
        } else {
            toast({ title: "Error", description: "Failed to reset database", variant: "destructive" });
        }
    };

    return (
        <div className="flex items-center justify-between">
            <div className="space-y-1">
                <Label className="text-base text-red-400">Reset Database</Label>
                <p className="text-sm text-muted-foreground">
                    Permanently delete all data (monitors, history, users) and restore defaults.
                </p>
            </div>
            <AlertDialog open={open} onOpenChange={setOpen}>
                <AlertDialogTrigger asChild>
                    <Button variant="destructive">Reset Everything</Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                        <AlertDialogDescription>
                            This action cannot be undone. This will permanently delete your
                            account and remove your data from our servers.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction onClick={handleReset} className="bg-destructive text-destructive-foreground hover:bg-destructive/90">
                            Reset Everything
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    );
}

function AppearanceSettings() {
    const { theme, setTheme } = useTheme();

    return (
        <Card>
            <CardHeader>
                <CardTitle>Appearance</CardTitle>
                <CardDescription>Customize the look of your dashboard.</CardDescription>
            </CardHeader>
            <CardContent>
                <div className="grid gap-2">
                    <Label>Theme</Label>
                    <Select value={theme} onValueChange={(v) => setTheme(v as "light" | "dark" | "system")}>
                        <SelectTrigger className="max-w-[200px]">
                            <SelectValue />
                        </SelectTrigger>
                        <SelectContent>
                            <SelectItem value="light">
                                <span className="flex items-center gap-2">
                                    <Sun className="h-4 w-4" /> Light
                                </span>
                            </SelectItem>
                            <SelectItem value="dark">
                                <span className="flex items-center gap-2">
                                    <Moon className="h-4 w-4" /> Dark
                                </span>
                            </SelectItem>
                            <SelectItem value="system">
                                <span className="flex items-center gap-2">
                                    <Monitor className="h-4 w-4" /> System
                                </span>
                            </SelectItem>
                        </SelectContent>
                    </Select>
                </div>
            </CardContent>
        </Card>
    );
}

function GeneralSettings() {
    const { settings, fetchSettings, updateSettings } = useMonitorStore();
    const { toast } = useToast();
    const [threshold, setThreshold] = useState(settings?.latency_threshold || "1000");
    const [retention, setRetention] = useState(settings?.data_retention_days || "30");

    // Fetch settings on mount
    useEffect(() => {
        fetchSettings();
    }, [fetchSettings]);

    // Update state when settings load
    useEffect(() => {
        if (settings) {
            setThreshold(settings.latency_threshold || "1000");
            setRetention(settings.data_retention_days || "30");
        }
    }, [settings]);

    const handleSave = async () => {
        await updateSettings({
            latency_threshold: threshold,
            data_retention_days: retention
        });
        toast({ title: "Settings Saved", description: "Global settings updated." });
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle>General Settings</CardTitle>
                <CardDescription>Global configuration for your monitors.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
                <div className="grid gap-2">
                    <Label htmlFor="latency">Latency Threshold (ms)</Label>
                    <div className="text-sm text-muted-foreground mb-2">
                        Response times higher than this value will mark the service as "Degraded".
                    </div>
                    <Input
                        id="latency"
                        type="number"
                        value={threshold}
                        onChange={(e) => setThreshold(e.target.value)}
                        className="max-w-[200px]"
                    />
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="retention">Data Retention (Days)</Label>
                    <div className="text-sm text-muted-foreground mb-2">
                        Monitor checks older than this will be automatically deleted.
                    </div>
                    <Input
                        id="retention"
                        type="number"
                        value={retention}
                        onChange={(e) => setRetention(e.target.value)}
                        className="max-w-[200px]"
                    />
                </div>
                <div className="rounded-lg border border-border/50 bg-muted/30 p-4">
                    <Label className="text-sm font-medium">SSL Certificate Warnings</Label>
                    <p className="text-sm text-muted-foreground mt-1">
                        Notifications are sent at 30, 14, 7, and 1 days before certificate expiry (at mid-day in your configured timezone).
                    </p>
                </div>
                <Button onClick={handleSave} className="w-fit">Save Settings</Button>
            </CardContent>
        </Card>
    );
}

const EVENT_TOGGLES = [
    { key: "notification.event.down.enabled", label: "Down", description: "Monitor is confirmed down" },
    { key: "notification.event.up.enabled", label: "Recovered", description: "Monitor recovered from down or degraded" },
    { key: "notification.event.degraded.enabled", label: "Degraded", description: "High latency detected" },
    { key: "notification.event.flapping.enabled", label: "Flapping", description: "Monitor oscillating between states" },
    { key: "notification.event.stabilized.enabled", label: "Stabilized", description: "Monitor stopped flapping" },
    { key: "notification.event.ssl_expiring.enabled", label: "SSL Expiring", description: "SSL certificate nearing expiry" },
] as const;

const DIGEST_EVENT_OPTIONS = [
    { value: "degraded", label: "Degraded" },
    { value: "flapping", label: "Flapping" },
    { value: "stabilized", label: "Stabilized" },
    { value: "ssl_expiring", label: "SSL Expiring" },
    { value: "down", label: "Down" },
    { value: "up", label: "Recovered" },
] as const;

function HelpTip({ text }: { text: string }) {
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <Info className="h-3.5 w-3.5 text-muted-foreground cursor-help inline-block ml-1 align-text-top" />
            </TooltipTrigger>
            <TooltipContent side="top" className="max-w-[260px]">
                {text}
            </TooltipContent>
        </Tooltip>
    );
}

function NotificationIntelligence() {
    const { settings, fetchSettings, updateSettings } = useMonitorStore();
    const { toast } = useToast();

    const [confirmThreshold, setConfirmThreshold] = useState(settings?.["notification.confirmation_threshold"] || "3");
    const [cooldownMins, setCooldownMins] = useState(settings?.["notification.cooldown_minutes"] || "30");
    const [flapEnabled, setFlapEnabled] = useState(settings?.["notification.flap_detection_enabled"] !== "false");
    const [flapWindow, setFlapWindow] = useState(settings?.["notification.flap_window_checks"] || "21");
    const [flapThreshold, setFlapThreshold] = useState(settings?.["notification.flap_threshold_percent"] || "25");
    const [recoveryChecks, setRecoveryChecks] = useState(settings?.["notification.recovery_confirmation_checks"] || "1");

    // Event type toggles
    const [eventToggles, setEventToggles] = useState<Record<string, boolean>>(() => {
        const toggles: Record<string, boolean> = {};
        EVENT_TOGGLES.forEach(({ key }) => {
            toggles[key] = settings?.[key] !== "false";
        });
        return toggles;
    });

    // Digest settings
    const [digestEnabled, setDigestEnabled] = useState(settings?.["notification.digest.enabled"] === "true");
    const [digestTime, setDigestTime] = useState(settings?.["notification.digest.time"] || "09:00");
    const [digestEventTypes, setDigestEventTypes] = useState<Set<string>>(() => {
        const types = settings?.["notification.digest.event_types"] || "degraded,flapping,stabilized,ssl_expiring";
        return new Set(types.split(",").map(t => t.trim()).filter(Boolean));
    });

    useEffect(() => {
        fetchSettings();
    }, [fetchSettings]);

    useEffect(() => {
        if (settings) {
            setConfirmThreshold(settings["notification.confirmation_threshold"] || "3");
            setCooldownMins(settings["notification.cooldown_minutes"] || "30");
            setFlapEnabled(settings["notification.flap_detection_enabled"] !== "false");
            setFlapWindow(settings["notification.flap_window_checks"] || "21");
            setFlapThreshold(settings["notification.flap_threshold_percent"] || "25");
            setRecoveryChecks(settings["notification.recovery_confirmation_checks"] || "1");

            const toggles: Record<string, boolean> = {};
            EVENT_TOGGLES.forEach(({ key }) => {
                toggles[key] = settings[key] !== "false";
            });
            setEventToggles(toggles);

            setDigestEnabled(settings["notification.digest.enabled"] === "true");
            setDigestTime(settings["notification.digest.time"] || "09:00");
            const types = settings["notification.digest.event_types"] || "degraded,flapping,stabilized,ssl_expiring";
            setDigestEventTypes(new Set(types.split(",").map(t => t.trim()).filter(Boolean)));
        }
    }, [settings]);

    const handleSave = async () => {
        const updates: Record<string, string> = {
            "notification.confirmation_threshold": confirmThreshold,
            "notification.cooldown_minutes": cooldownMins,
            "notification.flap_detection_enabled": flapEnabled ? "true" : "false",
            "notification.flap_window_checks": flapWindow,
            "notification.flap_threshold_percent": flapThreshold,
            "notification.recovery_confirmation_checks": recoveryChecks,
            "notification.digest.enabled": digestEnabled ? "true" : "false",
            "notification.digest.time": digestTime,
            "notification.digest.event_types": Array.from(digestEventTypes).join(","),
        };

        EVENT_TOGGLES.forEach(({ key }) => {
            updates[key] = eventToggles[key] ? "true" : "false";
        });

        await updateSettings(updates);
        toast({ title: "Settings Saved", description: "Notification intelligence settings updated." });
    };

    const toggleDigestEventType = (type: string) => {
        setDigestEventTypes(prev => {
            const next = new Set(prev);
            if (next.has(type)) {
                next.delete(type);
            } else {
                next.add(type);
            }
            return next;
        });
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle>Notification Intelligence</CardTitle>
                <CardDescription>
                    Control when and how notifications are sent.
                </CardDescription>
            </CardHeader>
            <CardContent>
                <TooltipProvider>
                    <div className="space-y-6">
                        {/* Event Types — always visible */}
                        <div className="space-y-3">
                            <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">Event Types</h3>
                            <div className="grid grid-cols-2 gap-3">
                                {EVENT_TOGGLES.map(({ key, label }) => (
                                    <div key={key} className="flex items-center justify-between">
                                        <Label className="text-sm">{label}</Label>
                                        <Switch
                                            checked={eventToggles[key] ?? true}
                                            onCheckedChange={(checked) =>
                                                setEventToggles(prev => ({ ...prev, [key]: checked }))
                                            }
                                        />
                                    </div>
                                ))}
                            </div>
                            <p className="text-xs text-muted-foreground">Disabled events are still logged.</p>
                        </div>

                        <Separator />

                        {/* Accordion sections */}
                        <Accordion type="multiple" className="space-y-2">
                            {/* Alerting Thresholds */}
                            <AccordionItem value="thresholds" className="border-none">
                                <AccordionTrigger className="hover:no-underline text-sm font-semibold text-muted-foreground uppercase tracking-wider py-2">
                                    Alerting Thresholds
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="grid grid-cols-3 gap-4 pt-2">
                                        <div className="grid gap-2">
                                            <Label htmlFor="confirm-threshold">
                                                Confirmation
                                                <HelpTip text="Consecutive failures before alerting. 1 = immediate." />
                                            </Label>
                                            <Input
                                                id="confirm-threshold"
                                                type="number"
                                                min={1}
                                                max={100}
                                                value={confirmThreshold}
                                                onChange={(e) => setConfirmThreshold(e.target.value)}
                                            />
                                        </div>
                                        <div className="grid gap-2">
                                            <Label htmlFor="cooldown-mins">
                                                Cooldown (min)
                                                <HelpTip text="Minutes to suppress duplicate alerts. Recovery always sends. 0 = disabled." />
                                            </Label>
                                            <Input
                                                id="cooldown-mins"
                                                type="number"
                                                min={0}
                                                max={1440}
                                                value={cooldownMins}
                                                onChange={(e) => setCooldownMins(e.target.value)}
                                            />
                                        </div>
                                        <div className="grid gap-2">
                                            <Label htmlFor="recovery-checks">
                                                Recovery
                                                <HelpTip text="Consecutive successes before recovery notification. 1 = immediate." />
                                            </Label>
                                            <Input
                                                id="recovery-checks"
                                                type="number"
                                                min={1}
                                                max={20}
                                                value={recoveryChecks}
                                                onChange={(e) => setRecoveryChecks(e.target.value)}
                                            />
                                        </div>
                                    </div>
                                </AccordionContent>
                            </AccordionItem>

                            {/* Flap Detection */}
                            <AccordionItem value="flap-detection" className="border-none">
                                <AccordionTrigger className="hover:no-underline text-sm font-semibold text-muted-foreground uppercase tracking-wider py-2">
                                    Flap Detection
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="space-y-4 pt-2">
                                        <div className="flex items-center justify-between">
                                            <Label>Enabled</Label>
                                            <Switch
                                                checked={flapEnabled}
                                                onCheckedChange={setFlapEnabled}
                                            />
                                        </div>
                                        {flapEnabled && (
                                            <div className="grid grid-cols-2 gap-4">
                                                <div className="grid gap-2">
                                                    <Label htmlFor="flap-window">
                                                        Window (checks)
                                                        <HelpTip text="Recent checks analyzed for state changes." />
                                                    </Label>
                                                    <Input
                                                        id="flap-window"
                                                        type="number"
                                                        min={3}
                                                        max={100}
                                                        value={flapWindow}
                                                        onChange={(e) => setFlapWindow(e.target.value)}
                                                    />
                                                </div>
                                                <div className="grid gap-2">
                                                    <Label htmlFor="flap-threshold">
                                                        Threshold (%)
                                                        <HelpTip text="State transition percentage that triggers flapping." />
                                                    </Label>
                                                    <Input
                                                        id="flap-threshold"
                                                        type="number"
                                                        min={1}
                                                        max={100}
                                                        value={flapThreshold}
                                                        onChange={(e) => setFlapThreshold(e.target.value)}
                                                    />
                                                </div>
                                            </div>
                                        )}
                                    </div>
                                </AccordionContent>
                            </AccordionItem>

                            {/* Daily Digest */}
                            <AccordionItem value="daily-digest" className="border-none">
                                <AccordionTrigger className="hover:no-underline text-sm font-semibold text-muted-foreground uppercase tracking-wider py-2">
                                    Daily Digest
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="space-y-4 pt-2">
                                        <div className="flex items-center justify-between">
                                            <Label>Enabled</Label>
                                            <Switch
                                                checked={digestEnabled}
                                                onCheckedChange={setDigestEnabled}
                                            />
                                        </div>
                                        {digestEnabled && (
                                            <>
                                                <div className="grid gap-2">
                                                    <Label htmlFor="digest-time">
                                                        Send at
                                                        <HelpTip text="Uses your configured timezone." />
                                                    </Label>
                                                    <Input
                                                        id="digest-time"
                                                        type="time"
                                                        value={digestTime}
                                                        onChange={(e) => setDigestTime(e.target.value)}
                                                        className="max-w-[160px]"
                                                    />
                                                </div>
                                                <div className="grid gap-2">
                                                    <Label>Batched Events</Label>
                                                    <div className="grid grid-cols-2 gap-2">
                                                        {DIGEST_EVENT_OPTIONS.map(({ value, label }) => (
                                                            <div key={value} className="flex items-center gap-2">
                                                                <Switch
                                                                    checked={digestEventTypes.has(value)}
                                                                    onCheckedChange={() => toggleDigestEventType(value)}
                                                                />
                                                                <Label className="text-sm font-normal">{label}</Label>
                                                            </div>
                                                        ))}
                                                    </div>
                                                </div>
                                            </>
                                        )}
                                    </div>
                                </AccordionContent>
                            </AccordionItem>
                        </Accordion>

                        <Button onClick={handleSave} className="w-fit">Save Settings</Button>
                    </div>
                </TooltipProvider>
            </CardContent>
        </Card>
    );
}

const VALID_TABS = ["general", "notifications", "security", "system"] as const;
type SettingsTab = typeof VALID_TABS[number];

export function SettingsView() {
    const { user, updateUser } = useMonitorStore();
    const { toast } = useToast();
    const [isLoading, setIsLoading] = useState(false);
    const [searchParams, setSearchParams] = useSearchParams();

    const tabParam = searchParams.get("tab") as SettingsTab | null;
    const activeTab = tabParam && VALID_TABS.includes(tabParam) ? tabParam : "general";

    const handleTabChange = (value: string) => {
        if (value === "general") {
            setSearchParams({});
        } else {
            setSearchParams({ tab: value });
        }
    };

    const [selectedTimezone, setSelectedTimezone] = useState(user?.timezone || 'UTC');

    useEffect(() => {
        if (user?.timezone) {
            setSelectedTimezone(user.timezone);
        }
    }, [user?.timezone]);

    const handleUpdateProfile = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        setIsLoading(true);
        const formData = new FormData(e.currentTarget);
        const timezone = formData.get("timezone") as string;
        const password = formData.get("password") as string;
        const currentPassword = formData.get("currentPassword") as string;

        try {
            await updateUser({
                timezone,
                password: password || undefined,
                currentPassword: currentPassword || undefined
            });
            toast({ title: "Settings updated", description: "Your profile has been updated successfully." });
        } catch (error) {
            toast({
                title: "Error",
                description: error instanceof Error ? error.message : "Failed to update settings",
                variant: "destructive"
            });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-medium">Settings</h3>
                <p className="text-sm text-muted-foreground">
                    Manage your workspace preferences.
                </p>
            </div>

            <Tabs value={activeTab} onValueChange={handleTabChange}>
                <div className="flex items-center justify-between">
                    <TabsList>
                        <TabsTrigger value="general">General</TabsTrigger>
                        <TabsTrigger value="notifications">Notifications</TabsTrigger>
                        <TabsTrigger value="security">Security</TabsTrigger>
                        <TabsTrigger value="system">System</TabsTrigger>
                    </TabsList>
                </div>

                <TabsContent value="general" className="space-y-6 mt-6">
                    <Card>
                        <CardHeader>
                            <CardTitle>Account Settings</CardTitle>
                            <CardDescription>
                                Manage your account preferences and security.
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <form onSubmit={handleUpdateProfile} className="space-y-4">
                                <div className="grid gap-2">
                                    <Label>Username</Label>
                                    <Input value={user?.username || user?.name || ''} disabled className="max-w-md" />
                                </div>
                                <div className="grid gap-2">
                                    <Label>Timezone</Label>
                                    <input type="hidden" name="timezone" value={selectedTimezone} />
                                    <SelectTimezone
                                        value={selectedTimezone}
                                        onValueChange={setSelectedTimezone}
                                    />
                                </div>

                                <Separator />

                                <div className="grid gap-2">
                                    <Label>Change Password</Label>
                                    <Input
                                        name="currentPassword"
                                        type="password"
                                        placeholder="Current Password (Required)"
                                        className="max-w-md"
                                    />
                                    <Input
                                        name="password"
                                        type="password"
                                        placeholder="New Password"
                                        className="max-w-md mt-2"
                                    />
                                </div>

                                <Button type="submit" disabled={isLoading}>
                                    {isLoading ? "Saving..." : "Save Changes"}
                                </Button>
                            </form>
                        </CardContent>
                    </Card>

                    <AppearanceSettings />

                    <GeneralSettings />
                </TabsContent>

                <TabsContent value="notifications" className="space-y-6 mt-6">
                    <NotificationsView />
                    <NotificationIntelligence />
                </TabsContent>

                <TabsContent value="security" className="space-y-6 mt-6">
                    <APIKeysView />
                    <SSOSettings />
                </TabsContent>

                <TabsContent value="system" className="space-y-6 mt-6">
                    <SystemTab />

                    <Card className="border-destructive/50">
                        <CardHeader>
                            <CardTitle className="text-destructive">Danger Zone</CardTitle>
                            <CardDescription>
                                Destructive actions that cannot be undone.
                            </CardDescription>
                        </CardHeader>
                        <CardContent>
                            <ResetDatabaseDialog />
                        </CardContent>
                    </Card>
                </TabsContent>
            </Tabs>
        </div>
    )
}
