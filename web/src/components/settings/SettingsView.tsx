import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Switch } from "@/components/ui/switch";
import { SystemTab } from "./SystemTab";
import { SSOSettings } from "./SSOSettings";
import { SelectTimezone } from "@/components/ui/select-timezone";

import { useState, useEffect } from "react";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";

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

function NotificationIntelligence() {
    const { settings, fetchSettings, updateSettings } = useMonitorStore();
    const { toast } = useToast();

    const [confirmThreshold, setConfirmThreshold] = useState(settings?.["notification.confirmation_threshold"] || "3");
    const [cooldownMins, setCooldownMins] = useState(settings?.["notification.cooldown_minutes"] || "30");
    const [flapEnabled, setFlapEnabled] = useState(settings?.["notification.flap_detection_enabled"] !== "false");
    const [flapWindow, setFlapWindow] = useState(settings?.["notification.flap_window_checks"] || "21");
    const [flapThreshold, setFlapThreshold] = useState(settings?.["notification.flap_threshold_percent"] || "25");

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
        }
    }, [settings]);

    const handleSave = async () => {
        await updateSettings({
            "notification.confirmation_threshold": confirmThreshold,
            "notification.cooldown_minutes": cooldownMins,
            "notification.flap_detection_enabled": flapEnabled ? "true" : "false",
            "notification.flap_window_checks": flapWindow,
            "notification.flap_threshold_percent": flapThreshold,
        });
        toast({ title: "Settings Saved", description: "Notification intelligence settings updated." });
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle>Notification Intelligence</CardTitle>
                <CardDescription>
                    Reduce notification noise by requiring confirmation, applying cooldowns, and detecting flapping monitors.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                <div className="grid gap-2">
                    <Label htmlFor="confirm-threshold">Confirmation Checks</Label>
                    <div className="text-sm text-muted-foreground mb-2">
                        Require this many consecutive failures before declaring a monitor down and sending a notification. A value of 1 means immediate alerting (no confirmation).
                    </div>
                    <Input
                        id="confirm-threshold"
                        type="number"
                        min={1}
                        max={100}
                        value={confirmThreshold}
                        onChange={(e) => setConfirmThreshold(e.target.value)}
                        className="max-w-[200px]"
                    />
                </div>
                <div className="grid gap-2">
                    <Label htmlFor="cooldown-mins">Notification Cooldown (Minutes)</Label>
                    <div className="text-sm text-muted-foreground mb-2">
                        After sending an alert, suppress duplicate notifications for the same condition for this many minutes. Recovery notifications always send immediately. Set to 0 to disable.
                    </div>
                    <Input
                        id="cooldown-mins"
                        type="number"
                        min={0}
                        max={1440}
                        value={cooldownMins}
                        onChange={(e) => setCooldownMins(e.target.value)}
                        className="max-w-[200px]"
                    />
                </div>
                <Separator />
                <div className="flex items-center justify-between">
                    <div className="space-y-1">
                        <Label>Flap Detection</Label>
                        <p className="text-sm text-muted-foreground">
                            Detect monitors oscillating rapidly between states and suppress individual notifications during flapping. A single "flapping" alert is sent instead.
                        </p>
                    </div>
                    <Switch
                        checked={flapEnabled}
                        onCheckedChange={setFlapEnabled}
                    />
                </div>
                {flapEnabled && (
                    <div className="grid grid-cols-2 gap-4 pl-1">
                        <div className="grid gap-2">
                            <Label htmlFor="flap-window">Window (checks)</Label>
                            <div className="text-sm text-muted-foreground mb-1">
                                Number of recent checks to analyze.
                            </div>
                            <Input
                                id="flap-window"
                                type="number"
                                min={3}
                                max={100}
                                value={flapWindow}
                                onChange={(e) => setFlapWindow(e.target.value)}
                                className="max-w-[160px]"
                            />
                        </div>
                        <div className="grid gap-2">
                            <Label htmlFor="flap-threshold">Threshold (%)</Label>
                            <div className="text-sm text-muted-foreground mb-1">
                                State transitions percentage that triggers flapping.
                            </div>
                            <Input
                                id="flap-threshold"
                                type="number"
                                min={1}
                                max={100}
                                value={flapThreshold}
                                onChange={(e) => setFlapThreshold(e.target.value)}
                                className="max-w-[160px]"
                            />
                        </div>
                    </div>
                )}
                <Button onClick={handleSave} className="w-fit">Save Settings</Button>
            </CardContent>
        </Card>
    );
}

export function SettingsView() {
    const { user, updateUser } = useMonitorStore();
    const { toast } = useToast();
    const [isLoading, setIsLoading] = useState(false);

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
            <Separator />

            <div className="space-y-6">
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

                <GeneralSettings />

                <NotificationIntelligence />

                <SSOSettings />

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
            </div>
        </div>
    )
}
