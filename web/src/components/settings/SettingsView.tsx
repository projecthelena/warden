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

import { UsersView } from "./UsersView";
import { CreateUserSheet } from "@/components/CreateUserSheet";
import { NotificationsView } from "@/components/notifications/NotificationsView";
import { SelectTimezone } from "@/components/ui/select-timezone";
import { useRole } from "@/hooks/useRole";

import { Accordion, AccordionItem, AccordionTrigger, AccordionContent } from "@/components/ui/accordion";
import { Tooltip, TooltipTrigger, TooltipContent, TooltipProvider } from "@/components/ui/tooltip";
import { Info, Monitor, Moon, RotateCcw, Sun } from "lucide-react";
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
                <CardTitle>Monitoring defaults</CardTitle>
                <CardDescription>Fallbacks used when a monitor does not have its own setting.</CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
                <div className="grid gap-4 sm:grid-cols-2">
                    <div className="grid gap-2">
                        <Label htmlFor="latency">Slow after</Label>
                        <div className="relative max-w-xs">
                            <Input id="latency" type="number" value={threshold} onChange={(e) => setThreshold(e.target.value)} className="pr-24" />
                            <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">milliseconds</span>
                        </div>
                        <p className="text-xs text-muted-foreground">Marks the monitor as degraded.</p>
                    </div>
                    <div className="grid gap-2">
                        <Label htmlFor="retention">Keep check data for</Label>
                        <div className="relative max-w-xs">
                            <Input id="retention" type="number" value={retention} onChange={(e) => setRetention(e.target.value)} className="pr-16" />
                            <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">days</span>
                        </div>
                        <p className="text-xs text-muted-foreground">Incident history is always kept.</p>
                    </div>
                </div>
                <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
                    <p className="text-sm text-muted-foreground">SSL expiry alerts are sent 30, 14, 7, and 1 days before expiry.</p>
                    <Button onClick={handleSave} className="w-fit whitespace-nowrap">Save defaults</Button>
                </div>
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
    const { settings, fetchSettings, updateSettings, relearnLatencyBaselines } = useMonitorStore();
    const { toast } = useToast();
    // Dashboard URL is admin-only because the underlying PATCH /api/settings endpoint
    // requires the admin role; hiding the field prevents editors from staging a value
    // and losing the rest of their edits to a 403.
    const { isAdmin } = useRole();
    const [settingsReady, setSettingsReady] = useState(Boolean(settings));

    const [confirmThreshold, setConfirmThreshold] = useState(settings?.["notification.confirmation_threshold"] || "3");
    const [cooldownMins, setCooldownMins] = useState(settings?.["notification.cooldown_minutes"] || "30");
    const [alertSustained, setAlertSustained] = useState(settings?.["notification.alert.sustained_seconds"] || "180");
    const [alertReminder, setAlertReminder] = useState(settings?.["notification.alert.reminder_minutes"] || "30");
    const [alertRepeat, setAlertRepeat] = useState(settings?.["notification.alert.repeat_reminder_minutes"] || "60");
    const [adaptiveLatency, setAdaptiveLatency] = useState(settings?.["notification.latency.adaptive_enabled"] !== "false");
    const [latencyFactor, setLatencyFactor] = useState(settings?.["notification.latency.factor_percent"] || "150");
    const [latencyBaselineDays, setLatencyBaselineDays] = useState(settings?.["notification.latency.baseline_days"] || "7");
    const [weeklyInsights, setWeeklyInsights] = useState(settings?.["notification.insights.weekly_enabled"] === "true");
    const [weeklyInsightsDay, setWeeklyInsightsDay] = useState(settings?.["notification.insights.weekly_day"] || "1");
    const [weeklyInsightsTime, setWeeklyInsightsTime] = useState(settings?.["notification.insights.weekly_time"] || "09:00");
    const [flapEnabled, setFlapEnabled] = useState(settings?.["notification.flap_detection_enabled"] !== "false");
    const [flapWindow, setFlapWindow] = useState(settings?.["notification.flap_window_checks"] || "21");
    const [flapThreshold, setFlapThreshold] = useState(settings?.["notification.flap_threshold_percent"] || "25");
    const [recoveryChecks, setRecoveryChecks] = useState(settings?.["notification.recovery_confirmation_checks"] || "1");
    const [relearningLatency, setRelearningLatency] = useState(false);

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
    const [appUrl, setAppUrl] = useState(settings?.["app_url"] || "");

    useEffect(() => {
        fetchSettings();
    }, [fetchSettings]);

    useEffect(() => {
        if (settings) {
            setConfirmThreshold(settings["notification.confirmation_threshold"] || "3");
            setCooldownMins(settings["notification.cooldown_minutes"] || "30");
            setAlertSustained(settings["notification.alert.sustained_seconds"] || "180");
            setAlertReminder(settings["notification.alert.reminder_minutes"] || "30");
            setAlertRepeat(settings["notification.alert.repeat_reminder_minutes"] || "60");
            setAdaptiveLatency(settings["notification.latency.adaptive_enabled"] !== "false");
            setLatencyFactor(settings["notification.latency.factor_percent"] || "150");
            setLatencyBaselineDays(settings["notification.latency.baseline_days"] || "7");
            setWeeklyInsights(settings["notification.insights.weekly_enabled"] === "true");
            setWeeklyInsightsDay(settings["notification.insights.weekly_day"] || "1");
            setWeeklyInsightsTime(settings["notification.insights.weekly_time"] || "09:00");
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
            setAppUrl(settings["app_url"] || "");
            setSettingsReady(true);
        }
    }, [settings]);

    const handleSave = async () => {
        const updates: Record<string, string> = {
            "notification.confirmation_threshold": confirmThreshold,
            "notification.cooldown_minutes": cooldownMins,
            "notification.alert.sustained_seconds": alertSustained,
            "notification.alert.reminder_minutes": alertReminder,
            "notification.alert.repeat_reminder_minutes": alertRepeat,
            "notification.latency.adaptive_enabled": String(adaptiveLatency),
            "notification.latency.factor_percent": latencyFactor,
            "notification.latency.baseline_days": latencyBaselineDays,
            "notification.insights.weekly_enabled": String(weeklyInsights),
            "notification.insights.weekly_day": weeklyInsightsDay,
            "notification.insights.weekly_time": weeklyInsightsTime,
            "notification.flap_detection_enabled": flapEnabled ? "true" : "false",
            "notification.flap_window_checks": flapWindow,
            "notification.flap_threshold_percent": flapThreshold,
            "notification.recovery_confirmation_checks": recoveryChecks,
            "notification.digest.enabled": digestEnabled ? "true" : "false",
            "notification.digest.time": digestTime,
            "notification.digest.event_types": Array.from(digestEventTypes).join(","),
        };
        // app_url is admin-only — only include in the patch when the current user has
        // the admin role, so editors don't unintentionally send (and trip backend RBAC).
        if (isAdmin) {
            updates["app_url"] = appUrl.trim();
        }

        EVENT_TOGGLES.forEach(({ key }) => {
            updates[key] = eventToggles[key] ? "true" : "false";
        });

        await updateSettings(updates);
        toast({ title: "Settings Saved", description: "Notification intelligence settings updated." });
    };

    const handleRelearnLatency = async () => {
        setRelearningLatency(true);
        const success = await relearnLatencyBaselines();
        setRelearningLatency(false);
        toast(success
            ? { title: "Learning restarted", description: "Old latency baselines were forgotten. Check history was kept." }
            : { title: "Could not restart learning", description: "Nothing was changed. Try again.", variant: "destructive" });
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

    if (!settingsReady) {
        return (
            <Card>
                <CardContent className="py-8 text-sm text-muted-foreground">Loading notification settings…</CardContent>
            </Card>
        );
    }

    return (
        <TooltipProvider>
            <div className="space-y-6">
                <Card>
                    <CardHeader>
                        <CardTitle>Alert rules</CardTitle>
                        <CardDescription>Choose what reaches your channels and when. Every event stays in history.</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-6">
                        <section className="space-y-3" aria-labelledby="alert-events-heading">
                            <div>
                                <h3 id="alert-events-heading" className="text-sm font-medium">Send an alert for</h3>
                                <p className="mt-1 text-sm text-muted-foreground">Turn off anything that does not need your attention.</p>
                            </div>
                            <div className="grid gap-2 sm:grid-cols-2">
                                {EVENT_TOGGLES.map(({ key, label, description }) => (
                                    <div key={key} className="flex min-h-14 items-center justify-between gap-4 rounded-md border px-3 py-2.5">
                                        <div className="min-w-0">
                                            <Label htmlFor={key} className="text-sm">{label}</Label>
                                            <p className="truncate text-xs text-muted-foreground">{description}</p>
                                        </div>
                                        <Switch
                                            id={key}
                                            checked={eventToggles[key] ?? true}
                                            onCheckedChange={(checked) => setEventToggles(prev => ({ ...prev, [key]: checked }))}
                                        />
                                    </div>
                                ))}
                            </div>
                        </section>

                        <Separator />

                        <section className="space-y-4" aria-labelledby="outage-timing-heading" data-testid="alert-ladder">
                            <div>
                                <h3 id="outage-timing-heading" className="text-sm font-medium">Outage timing</h3>
                                <p className="mt-1 text-sm text-muted-foreground">Wait through brief blips, then keep the team informed until recovery.</p>
                            </div>
                            <div className="grid gap-4 sm:grid-cols-3">
                                <div className="grid gap-2">
                                    <Label htmlFor="alert-sustained">Alert after</Label>
                                    <div className="relative">
                                        <Input id="alert-sustained" data-testid="alert-sustained" type="number" min={0} max={86400} value={alertSustained} onChange={(e) => setAlertSustained(e.target.value)} className="pr-20" />
                                        <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">seconds</span>
                                    </div>
                                </div>
                                <div className="grid gap-2">
                                    <Label htmlFor="alert-reminder">First reminder</Label>
                                    <div className="relative">
                                        <Input id="alert-reminder" data-testid="alert-reminder" type="number" min={0} max={10080} value={alertReminder} onChange={(e) => setAlertReminder(e.target.value)} className="pr-20" />
                                        <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">minutes</span>
                                    </div>
                                </div>
                                <div className="grid gap-2">
                                    <Label htmlFor="alert-repeat">Repeat every</Label>
                                    <div className="relative">
                                        <Input id="alert-repeat" data-testid="alert-repeat" type="number" min={0} max={10080} value={alertRepeat} onChange={(e) => setAlertRepeat(e.target.value)} className="pr-20" />
                                        <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-xs text-muted-foreground">minutes</span>
                                    </div>
                                </div>
                            </div>
                            <p className="text-xs text-muted-foreground">Use 0 to alert immediately or turn reminders off.</p>
                        </section>

                        <Separator />

                        <Accordion type="multiple">
                            <AccordionItem value="advanced-delivery">
                                <AccordionTrigger className="hover:no-underline">
                                    <span className="text-left">
                                        <span className="block text-sm font-medium">Advanced delivery</span>
                                        <span className="mt-1 block text-xs font-normal text-muted-foreground">Failure confirmation, recovery confirmation, and cooldown.</span>
                                    </span>
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="grid gap-4 pt-2 sm:grid-cols-3">
                                        <div className="grid gap-2">
                                            <Label htmlFor="confirm-threshold">Failures before alert <HelpTip text="Consecutive failed checks required before Warden changes the monitor state." /></Label>
                                            <Input id="confirm-threshold" type="number" min={1} max={100} value={confirmThreshold} onChange={(e) => setConfirmThreshold(e.target.value)} />
                                        </div>
                                        <div className="grid gap-2">
                                            <Label htmlFor="recovery-checks">Successes before recovery <HelpTip text="Consecutive successful checks required before Warden announces recovery." /></Label>
                                            <Input id="recovery-checks" type="number" min={1} max={20} value={recoveryChecks} onChange={(e) => setRecoveryChecks(e.target.value)} />
                                        </div>
                                        <div className="grid gap-2">
                                            <Label htmlFor="cooldown-mins">Flap cooldown <HelpTip text="Minutes to suppress repeat flapping and stabilized alerts. 0 disables the cooldown." /></Label>
                                            <Input id="cooldown-mins" type="number" min={0} max={1440} value={cooldownMins} onChange={(e) => setCooldownMins(e.target.value)} />
                                        </div>
                                    </div>
                                </AccordionContent>
                            </AccordionItem>

                            <AccordionItem value="flap-detection" className="border-none">
                                <AccordionTrigger className="hover:no-underline">
                                    <span className="text-left">
                                        <span className="block text-sm font-medium">Flap detection</span>
                                        <span className="mt-1 block text-xs font-normal text-muted-foreground">Detect monitors that rapidly alternate between healthy and unhealthy.</span>
                                    </span>
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="space-y-4 pt-2">
                                        <div className="flex items-center justify-between">
                                            <Label>Detect flapping</Label>
                                            <Switch
                                                checked={flapEnabled}
                                                onCheckedChange={setFlapEnabled}
                                            />
                                        </div>
                                        {flapEnabled && (
                                            <div className="grid gap-4 sm:grid-cols-2">
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

                            <AccordionItem value="latency-baseline" className="border-none">
                                <AccordionTrigger className="hover:no-underline">
                                    <span className="text-left">
                                        <span className="block text-sm font-medium">High latency</span>
                                        <span className="mt-1 block text-xs font-normal text-muted-foreground">Let each monitor learn its normal response time.</span>
                                    </span>
                                </AccordionTrigger>
                                <AccordionContent>
                                    <div className="space-y-4 pt-2">
                                        <div className="flex items-center justify-between">
                                            <div className="pr-4">
                                                <Label>Learn what is normal for each monitor</Label>
                                                <div className="text-sm text-muted-foreground mt-1">
                                                    Judges each monitor against its own recent latency instead of one number
                                                    for everything. A health check that answers in 250ms and a homepage that
                                                    answers in 430ms are not slow at the same point.
                                                </div>
                                            </div>
                                            <Switch
                                                checked={adaptiveLatency}
                                                onCheckedChange={setAdaptiveLatency}
                                                data-testid="adaptive-latency-switch"
                                            />
                                        </div>
                                        {adaptiveLatency && (
                                            <div className="space-y-4">
                                                <div className="grid gap-4 sm:grid-cols-2">
                                                    <div className="grid gap-2">
                                                        <Label htmlFor="latency-factor">
                                                            Slow at (% of p95)
                                                            <HelpTip text="A monitor counts as degraded above this multiple of its own 95th-percentile latency. 150 means 1.5x. The threshold never sits closer than 100ms above p95, so very fast targets do not alert on noise." />
                                                        </Label>
                                                        <Input
                                                            id="latency-factor"
                                                            type="number"
                                                            min={100}
                                                            max={10000}
                                                            value={latencyFactor}
                                                            onChange={(e) => setLatencyFactor(e.target.value)}
                                                        />
                                                    </div>
                                                    <div className="grid gap-2">
                                                        <Label htmlFor="latency-baseline-days">
                                                            Learn from (days)
                                                            <HelpTip text="How far back the baseline looks. Shorter adapts faster to a service that genuinely changed; longer is steadier." />
                                                        </Label>
                                                        <Input
                                                            id="latency-baseline-days"
                                                            type="number"
                                                            min={1}
                                                            max={90}
                                                            value={latencyBaselineDays}
                                                            onChange={(e) => setLatencyBaselineDays(e.target.value)}
                                                        />
                                                    </div>
                                                </div>
                                                {isAdmin && (
                                                    <div className="flex flex-col gap-4 rounded-md border p-4 sm:flex-row sm:items-center sm:justify-between">
                                                        <div className="pr-4">
                                                            <Label>Moved Warden?</Label>
                                                            <div className="text-sm text-muted-foreground mt-1">
                                                                Start learning latency from this network. Check history stays intact,
                                                                and the fixed threshold is used until enough new checks arrive.
                                                            </div>
                                                        </div>
                                                        <AlertDialog>
                                                            <AlertDialogTrigger asChild>
                                                                <Button variant="outline" disabled={relearningLatency} data-testid="relearn-latency-button">
                                                                    <RotateCcw className="h-4 w-4 mr-2" />
                                                                    {relearningLatency ? "Restarting..." : "Relearn from now"}
                                                                </Button>
                                                            </AlertDialogTrigger>
                                                            <AlertDialogContent>
                                                                <AlertDialogHeader>
                                                                    <AlertDialogTitle>Relearn latency from this network?</AlertDialogTitle>
                                                                    <AlertDialogDescription>
                                                                        Warden will forget its derived latency baselines, close current
                                                                        degraded-latency alerts, and learn only from new successful checks.
                                                                        Monitor history and uptime data will not be deleted.
                                                                    </AlertDialogDescription>
                                                                </AlertDialogHeader>
                                                                <AlertDialogFooter>
                                                                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                                                                    <AlertDialogAction onClick={handleRelearnLatency}>Relearn from now</AlertDialogAction>
                                                                </AlertDialogFooter>
                                                            </AlertDialogContent>
                                                        </AlertDialog>
                                                    </div>
                                                )}
                                            </div>
                                        )}
                                        <div className="text-sm text-muted-foreground">
                                            {adaptiveLatency
                                                ? "Monitors without enough history yet, and monitors with their own latency threshold set, keep using that fixed number."
                                                : "Every monitor is judged against the fixed latency threshold above."}
                                        </div>
                                    </div>
                                </AccordionContent>
                            </AccordionItem>

                        </Accordion>
                    </CardContent>
                </Card>

                <Card>
                    <CardHeader>
                        <CardTitle>Summaries</CardTitle>
                        <CardDescription>Optional reports for context that does not need an immediate alert.</CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-6">
                        <section className="space-y-4" aria-labelledby="daily-digest-heading">
                            <div className="flex items-start justify-between gap-4">
                                <div>
                                    <Label id="daily-digest-heading" htmlFor="digest-enabled">Daily digest</Label>
                                    <p className="mt-1 text-sm text-muted-foreground">One daily recap of the event types you choose.</p>
                                </div>
                                <Switch id="digest-enabled" checked={digestEnabled} onCheckedChange={setDigestEnabled} data-testid="digest-enabled" />
                            </div>
                            {digestEnabled && (
                                <div className="space-y-4 rounded-md border p-4">
                                    <div className="grid gap-4 sm:grid-cols-[10rem_minmax(0,1fr)]">
                                        <div className="grid gap-2">
                                            <Label htmlFor="digest-time">Send at</Label>
                                            <Input id="digest-time" type="time" value={digestTime} onChange={(e) => setDigestTime(e.target.value)} />
                                        </div>
                                        <div className="grid gap-2">
                                            <Label>Include</Label>
                                            <div className="grid gap-2 sm:grid-cols-3">
                                                {DIGEST_EVENT_OPTIONS.map(({ value, label }) => (
                                                    <label key={value} className="flex items-center gap-2 text-sm">
                                                        <Switch checked={digestEventTypes.has(value)} onCheckedChange={() => toggleDigestEventType(value)} />
                                                        {label}
                                                    </label>
                                                ))}
                                            </div>
                                        </div>
                                    </div>
                                    <p className="text-xs text-muted-foreground" data-testid="digest-scope-note">Immediate alerts are controlled separately above.</p>
                                    {isAdmin && (
                                        <div className="grid gap-2">
                                            <Label htmlFor="digest-app-url">Dashboard URL <HelpTip text="Adds links back to Warden from the digest. Leave empty for plain text." /></Label>
                                            <Input id="digest-app-url" type="url" placeholder="https://warden.example.com" value={appUrl} onChange={(e) => setAppUrl(e.target.value)} className="max-w-md font-mono text-xs" />
                                        </div>
                                    )}
                                </div>
                            )}
                        </section>

                        <Separator />

                        <section className="space-y-4" aria-labelledby="weekly-patterns-heading">
                            <div className="flex items-start justify-between gap-4">
                                <div>
                                    <Label id="weekly-patterns-heading" htmlFor="weekly-insights">Weekly patterns</Label>
                                    <p className="mt-1 text-sm text-muted-foreground">A weekly note when Warden finds recurring slowdowns or related failures.</p>
                                </div>
                                <Switch id="weekly-insights" checked={weeklyInsights} onCheckedChange={setWeeklyInsights} data-testid="weekly-insights-switch" />
                            </div>
                            {weeklyInsights && (
                                <div className="grid gap-4 rounded-md border p-4 sm:grid-cols-2">
                                    <div className="grid gap-2">
                                        <Label htmlFor="weekly-insights-day">Send on</Label>
                                        <select id="weekly-insights-day" value={weeklyInsightsDay} onChange={(e) => setWeeklyInsightsDay(e.target.value)} className="flex h-9 w-full rounded-md border border-input bg-transparent px-3 py-1 text-sm shadow-sm">
                                            {["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"].map((label, i) => <option key={label} value={String(i)}>{label}</option>)}
                                        </select>
                                    </div>
                                    <div className="grid gap-2">
                                        <Label htmlFor="weekly-insights-time">At</Label>
                                        <Input id="weekly-insights-time" type="time" value={weeklyInsightsTime} onChange={(e) => setWeeklyInsightsTime(e.target.value)} />
                                    </div>
                                </div>
                            )}
                        </section>
                    </CardContent>
                </Card>

                <div className="flex justify-end">
                    <Button onClick={handleSave} className="w-full whitespace-nowrap sm:w-auto" data-testid="save-notification-settings">Save notification settings</Button>
                </div>
            </div>
        </TooltipProvider>
    );
}

const _VALID_TABS = ["general", "notifications", "security", "system", "users"] as const;
type SettingsTab = typeof _VALID_TABS[number];

export function SettingsView() {
    const { user, updateUser } = useMonitorStore();
    const { toast } = useToast();
    const { isAdmin, canEdit } = useRole();
    const [isLoading, setIsLoading] = useState(false);
    const [searchParams, setSearchParams] = useSearchParams();
    const { theme, setTheme } = useTheme();
    const isSSOUser = Boolean(user?.ssoProvider);

    const tabParam = searchParams.get("tab") as SettingsTab | null;
    // Enforce role-based tab access: viewers can only see "general"
    const allowedTabs: SettingsTab[] = isAdmin
        ? ["general", "notifications", "security", "system", "users"]
        : canEdit
            ? ["general", "notifications"]
            : ["general"];
    const activeTab = tabParam && allowedTabs.includes(tabParam) ? tabParam : "general";

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
        <div className="min-w-0 space-y-6">
            <div>
                <h3 className="text-lg font-medium">Settings</h3>
                <p className="text-sm text-muted-foreground">
                    Manage your workspace preferences.
                </p>
            </div>

            <Tabs value={activeTab} onValueChange={handleTabChange} className="min-w-0">
                <div className="flex items-center justify-between">
                    <TabsList className="grid h-auto w-full grid-cols-2 gap-1 sm:inline-flex sm:w-auto">
                        <TabsTrigger value="general">General</TabsTrigger>
                        {canEdit && <TabsTrigger value="notifications">Notifications</TabsTrigger>}
                        {isAdmin && <TabsTrigger value="security">Security</TabsTrigger>}
                        {isAdmin && <TabsTrigger value="system">System</TabsTrigger>}
                        {isAdmin && <TabsTrigger value="users">Users</TabsTrigger>}
                    </TabsList>
                </div>

                <TabsContent value="general" className="space-y-6 mt-6">
                    <Card>
                        <CardHeader>
                            <CardTitle>Profile and preferences</CardTitle>
                            <CardDescription>Your identity, timezone, and dashboard appearance.</CardDescription>
                        </CardHeader>
                        <CardContent className="space-y-4">
                            <form onSubmit={handleUpdateProfile} className="space-y-4">
                                <div className="grid gap-4 sm:grid-cols-2">
                                    <div className="grid gap-2">
                                        <Label>Username</Label>
                                        <Input value={user?.username || user?.name || ''} disabled />
                                    </div>
                                    <div className="grid gap-2">
                                        <Label>Timezone</Label>
                                        <input type="hidden" name="timezone" value={selectedTimezone} />
                                        <SelectTimezone value={selectedTimezone} onValueChange={setSelectedTimezone} />
                                    </div>
                                </div>

                                <Separator />

                                <div className="grid gap-2">
                                    <Label>Theme</Label>
                                    <Select value={theme} onValueChange={(value) => setTheme(value as "light" | "dark" | "system")}>
                                        <SelectTrigger className="max-w-[200px]">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="light"><span className="flex items-center gap-2"><Sun className="h-4 w-4" /> Light</span></SelectItem>
                                            <SelectItem value="dark"><span className="flex items-center gap-2"><Moon className="h-4 w-4" /> Dark</span></SelectItem>
                                            <SelectItem value="system"><span className="flex items-center gap-2"><Monitor className="h-4 w-4" /> System</span></SelectItem>
                                        </SelectContent>
                                    </Select>
                                </div>

                                <Separator />

                                {isSSOUser ? (
                                    <div className="rounded-lg border border-border/50 bg-muted/30 p-4">
                                        <Label className="text-sm font-medium">Single sign-on account</Label>
                                        <p className="text-sm text-muted-foreground mt-1">
                                            Your password is managed by your identity provider. Change it there, not in Warden.
                                        </p>
                                    </div>
                                ) : (
                                    <div className="grid max-w-md gap-2">
                                        <Label>Change password</Label>
                                        <Input
                                            name="currentPassword"
                                            type="password"
                                            placeholder="Current Password (Required)"
                                        />
                                        <Input
                                            name="password"
                                            type="password"
                                            placeholder="New Password"
                                            className="mt-2"
                                        />
                                    </div>
                                )}

                                <Button type="submit" disabled={isLoading}>
                                    {isLoading ? "Saving..." : "Save Changes"}
                                </Button>
                            </form>
                        </CardContent>
                    </Card>

                    {isAdmin && <GeneralSettings />}
                </TabsContent>

                {canEdit && (
                    <TabsContent value="notifications" className="space-y-6 mt-6">
                        <NotificationsView />
                        <NotificationIntelligence />
                    </TabsContent>
                )}

                {isAdmin && (
                    <TabsContent value="security" className="space-y-6 mt-6">
                        <APIKeysView />
                        <SSOSettings />
                    </TabsContent>
                )}

                {isAdmin && (
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
                )}

                {isAdmin && (
                    <TabsContent value="users" className="space-y-6 mt-6">
                        <div className="flex items-center justify-between">
                            <div>
                                <h4 className="text-sm font-medium">User Management</h4>
                                <p className="text-sm text-muted-foreground">Create local accounts, review SSO identities, and manage roles.</p>
                            </div>
                            <CreateUserSheet />
                        </div>
                        <UsersView />
                    </TabsContent>
                )}
            </Tabs>
        </div>
    )
}
