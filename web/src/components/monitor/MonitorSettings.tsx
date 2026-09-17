import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { DNS_RECORD_TYPES, MONITOR_TYPES, Monitor, MonitorType, RequestConfig, useMonitorStore } from "@/lib/store";
import { MONITOR_TYPE_INFO, isValidTarget } from "@/lib/monitorTypes";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/use-toast";
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle, AlertDialogTrigger } from "@/components/ui/alert-dialog";
import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from "@/components/ui/accordion";
import { BellRing, Globe2, Save, TimerReset, Trash2, X } from "lucide-react";

export function MonitorSettings({ monitor, groupId }: { monitor: Monitor; groupId: string }) {
    const navigate = useNavigate();
    const { toast } = useToast();
    const { groups, updateMonitor, moveMonitor, deleteMonitor } = useMonitorStore();
    const [name, setName] = useState(monitor.name);
    const [type, setType] = useState<MonitorType>(monitor.type ?? "http");
    const [target, setTarget] = useState(monitor.url);
    const [interval, setInterval] = useState(monitor.interval || 60);
    const [group, setGroup] = useState(groupId);
    const [pendingGroup, setPendingGroup] = useState<string | null>(null);
    const [confirmation, setConfirmation] = useState(monitor.confirmationThreshold?.toString() ?? "");
    const [cooldown, setCooldown] = useState(monitor.notificationCooldownMinutes?.toString() ?? "");
    const [latencyThreshold, setLatencyThreshold] = useState(monitor.latencyThreshold?.toString() ?? "");
    const [timeout, setTimeoutValue] = useState(monitor.requestConfig?.timeoutSeconds?.toString() ?? "");
    const [retries, setRetries] = useState(monitor.requestConfig?.retryCount?.toString() ?? "0");
    const [method, setMethod] = useState(monitor.requestConfig?.method || "GET");
    const [acceptedCodes, setAcceptedCodes] = useState(monitor.requestConfig?.acceptedStatusCodes ?? "");
    const [followRedirects, setFollowRedirects] = useState(monitor.requestConfig?.followRedirects !== false);
    const [headers, setHeaders] = useState(Object.entries(monitor.requestConfig?.headers ?? {}).map(([key, value]) => ({ key, value })));
    const [body, setBody] = useState(monitor.requestConfig?.body ?? "");
    const [recordType, setRecordType] = useState(monitor.requestConfig?.dnsRecordType ?? "A");
    const [resolver, setResolver] = useState(monitor.requestConfig?.dnsResolver ?? "");
    const [saving, setSaving] = useState(false);

    useEffect(() => {
        setName(monitor.name);
        setType(monitor.type ?? "http");
        setTarget(monitor.url);
        setInterval(monitor.interval || 60);
        setGroup(groupId);
    }, [monitor, groupId]);

    const save = async () => {
        const normalizedTarget = target.trim();
        if (!name.trim()) {
            toast({ title: "Name required", description: "Give this monitor a name before saving.", variant: "destructive" });
            return;
        }
        if (!isValidTarget(type, normalizedTarget)) {
            toast({ title: "Invalid target", description: `Use a target such as ${MONITOR_TYPE_INFO[type].placeholder}.`, variant: "destructive" });
            return;
        }

        const requestConfig: RequestConfig = {};
        if (timeout) requestConfig.timeoutSeconds = Number(timeout);
        if (Number(retries) > 0) requestConfig.retryCount = Number(retries);
        if (type === "http") {
            const cleanHeaders = Object.fromEntries(headers.filter(item => item.key.trim()).map(item => [item.key.trim(), item.value.trim()]));
            if (method !== "GET") requestConfig.method = method;
            if (acceptedCodes) requestConfig.acceptedStatusCodes = acceptedCodes;
            if (!followRedirects) requestConfig.followRedirects = false;
            if (Object.keys(cleanHeaders).length) requestConfig.headers = cleanHeaders;
            if (body) requestConfig.body = body;
        }
        if (type === "dns") {
            if (recordType !== "A") requestConfig.dnsRecordType = recordType;
            if (resolver) requestConfig.dnsResolver = resolver;
        }

        setSaving(true);
        await updateMonitor(monitor.id, {
            name: name.trim(), type, url: normalizedTarget, interval,
            confirmationThreshold: confirmation ? Number(confirmation) : undefined,
            notificationCooldownMinutes: cooldown ? Number(cooldown) : undefined,
            latencyThreshold: latencyThreshold ? Number(latencyThreshold) : undefined,
            requestConfig: Object.keys(requestConfig).length || monitor.requestConfig ? requestConfig : undefined,
        });
        setSaving(false);
    };

    return (
        <div className="space-y-5" data-testid="monitor-settings">
            <Card className="border-border bg-card shadow-none">
                <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
                    <div><CardTitle className="text-base">General</CardTitle><CardDescription className="mt-1">What Warden checks and how often.</CardDescription></div>
                    <Button onClick={save} disabled={saving} data-testid="monitor-edit-save-btn" className="shrink-0 whitespace-nowrap"><Save className="mr-2 h-4 w-4" />{saving ? "Saving…" : "Save changes"}</Button>
                </CardHeader>
                <CardContent className="grid gap-5 sm:grid-cols-2">
                    <Field label="Display name" className="sm:col-span-2"><Input value={name} onChange={event => setName(event.target.value)} data-testid="monitor-edit-name-input" /></Field>
                    <Field label="Check type">
                        <Select value={type} onValueChange={value => setType(value as MonitorType)}><SelectTrigger data-testid="monitor-edit-type-select"><SelectValue /></SelectTrigger><SelectContent>{MONITOR_TYPES.map(item => <SelectItem key={item} value={item}>{MONITOR_TYPE_INFO[item].label}</SelectItem>)}</SelectContent></Select>
                    </Field>
                    <Field label="Check frequency">
                        <Select value={interval.toString()} onValueChange={value => setInterval(Number(value))}><SelectTrigger data-testid="monitor-edit-interval-select"><SelectValue /></SelectTrigger><SelectContent>{[30, 60, 300, 600, 1800, 3600].map(seconds => <SelectItem key={seconds} value={seconds.toString()}>{seconds < 60 ? `${seconds} seconds` : seconds === 60 ? "1 minute" : `${seconds / 60} minutes`}</SelectItem>)}</SelectContent></Select>
                    </Field>
                    <Field label={MONITOR_TYPE_INFO[type].targetLabel} className="sm:col-span-2"><Input value={target} onChange={event => setTarget(event.target.value)} placeholder={MONITOR_TYPE_INFO[type].placeholder} className="font-mono text-xs" data-testid="monitor-edit-url-input" /></Field>
                    <Field label="Group" className="sm:col-span-2">
                        <Select value={group} onValueChange={setPendingGroup}><SelectTrigger data-testid="monitor-edit-group-select"><SelectValue /></SelectTrigger><SelectContent>{groups.map(item => <SelectItem key={item.id} value={item.id}>{item.name}</SelectItem>)}</SelectContent></Select>
                        <p className="text-xs text-muted-foreground">Moving is applied separately because it changes which status pages include this monitor.</p>
                    </Field>
                </CardContent>
            </Card>

            <Card className="border-border bg-card shadow-none">
                <CardHeader className="pb-2"><CardTitle className="text-base">Advanced settings</CardTitle><CardDescription>Open only the section you need. Defaults work well for most monitors.</CardDescription></CardHeader>
                <CardContent>
                    <Accordion type="multiple" className="w-full">
                        <AccordionItem value="alerting">
                            <AccordionTrigger className="hover:no-underline">
                                <SectionLabel icon={<BellRing />} title="Alerting" summary="Confirmation, cooldown and latency threshold" />
                            </AccordionTrigger>
                            <AccordionContent className="grid gap-5 px-1 pt-2 sm:grid-cols-2">
                                <Field label="Confirmation checks"><Input aria-label="Confirmation checks" type="number" min={1} placeholder="Global default" value={confirmation} onChange={event => setConfirmation(event.target.value)} /></Field>
                                <Field label="Cooldown (minutes)"><Input aria-label="Cooldown (minutes)" type="number" min={0} placeholder="Global default" value={cooldown} onChange={event => setCooldown(event.target.value)} /></Field>
                                <Field label="Latency threshold (ms)" className="sm:col-span-2"><Input aria-label="Latency threshold (ms)" type="number" min={1} placeholder="Global default" value={latencyThreshold} onChange={event => setLatencyThreshold(event.target.value)} /></Field>
                            </AccordionContent>
                        </AccordionItem>

                        <AccordionItem value="behavior">
                            <AccordionTrigger className="hover:no-underline">
                                <SectionLabel icon={<TimerReset />} title="Failure handling" summary={`${timeout || 5}s timeout · ${Number(retries) === 0 ? "no retries" : `${retries} retries`}`} />
                            </AccordionTrigger>
                            <AccordionContent className="grid gap-5 px-1 pt-2 sm:grid-cols-2">
                                <Field label="Timeout (seconds)"><Input aria-label="Timeout (seconds)" type="number" min={1} max={120} placeholder="5" value={timeout} onChange={event => setTimeoutValue(event.target.value)} /></Field>
                                <Field label="Retry on failure"><Select value={retries} onValueChange={setRetries}><SelectTrigger data-testid="request-retry-select"><SelectValue /></SelectTrigger><SelectContent>{[0, 1, 2, 3, 4, 5].map(value => <SelectItem key={value} value={value.toString()}>{value === 0 ? "No retry" : `${value} ${value === 1 ? "retry" : "retries"}`}</SelectItem>)}</SelectContent></Select></Field>
                            </AccordionContent>
                        </AccordionItem>

                        {type === "http" && <AccordionItem value="request">
                            <AccordionTrigger className="hover:no-underline">
                                <SectionLabel icon={<Globe2 />} title="HTTP request" summary={`${method} · ${acceptedCodes || "200–399"} · ${followRedirects ? "follows redirects" : "does not follow redirects"}`} />
                            </AccordionTrigger>
                            <AccordionContent className="space-y-5 px-1 pt-2">
                                <div className="grid gap-5 sm:grid-cols-2">
                                    <Field label="Method"><Select value={method} onValueChange={setMethod}><SelectTrigger data-testid="request-method-select"><SelectValue /></SelectTrigger><SelectContent>{["GET", "HEAD", "POST", "PUT", "DELETE"].map(item => <SelectItem key={item} value={item}>{item}</SelectItem>)}</SelectContent></Select></Field>
                                    <Field label="Accepted status codes"><Input placeholder="200-399" value={acceptedCodes} onChange={event => setAcceptedCodes(event.target.value)} /></Field>
                                </div>
                                <div className="flex min-h-11 items-center justify-between rounded-lg border border-border px-3"><Label htmlFor="follow-redirects">Follow redirects</Label><Switch id="follow-redirects" checked={followRedirects} onCheckedChange={setFollowRedirects} /></div>
                                <Field label="Custom headers">
                                    <div className="space-y-2">{headers.map((header, index) => <div key={index} className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2.75rem] gap-2"><Input aria-label={`Header ${index + 1} name`} placeholder="Header name" value={header.key} onChange={event => setHeaders(current => current.map((item, itemIndex) => itemIndex === index ? { ...item, key: event.target.value } : item))} /><Input aria-label={`Header ${index + 1} value`} placeholder="Value" value={header.value} onChange={event => setHeaders(current => current.map((item, itemIndex) => itemIndex === index ? { ...item, value: event.target.value } : item))} /><Button type="button" variant="ghost" size="icon" aria-label="Remove header" onClick={() => setHeaders(current => current.filter((_, itemIndex) => itemIndex !== index))}><X className="h-4 w-4" /></Button></div>)}</div>
                                    <Button type="button" variant="outline" size="sm" onClick={() => setHeaders(current => [...current, { key: "", value: "" }])}>Add header</Button>
                                </Field>
                                {(method === "POST" || method === "PUT") && <Field label="Request body"><Textarea value={body} onChange={event => setBody(event.target.value)} placeholder='{"status":"ok"}' className="min-h-28 resize-y font-mono text-xs" /></Field>}
                            </AccordionContent>
                        </AccordionItem>}

                        {type === "dns" && <AccordionItem value="request">
                            <AccordionTrigger className="hover:no-underline"><SectionLabel icon={<Globe2 />} title="DNS query" summary={`${recordType} record · ${resolver || "system resolver"}`} /></AccordionTrigger>
                            <AccordionContent className="grid gap-5 px-1 pt-2 sm:grid-cols-2"><Field label="Record type"><Select value={recordType} onValueChange={setRecordType}><SelectTrigger data-testid="dns-record-type-select"><SelectValue /></SelectTrigger><SelectContent>{DNS_RECORD_TYPES.map(item => <SelectItem key={item} value={item}>{item}</SelectItem>)}</SelectContent></Select></Field><Field label="Resolver"><Input value={resolver} onChange={event => setResolver(event.target.value)} placeholder="System default" data-testid="dns-resolver-input" /></Field></AccordionContent>
                        </AccordionItem>}
                    </Accordion>
                </CardContent>
            </Card>

            <Accordion type="single" collapsible className="rounded-xl border border-destructive/30 bg-destructive/5 px-5">
                <AccordionItem value="danger" className="border-0">
                    <AccordionTrigger className="text-destructive hover:no-underline"><SectionLabel icon={<Trash2 />} title="Danger zone" summary="Permanently delete this monitor and its history" /></AccordionTrigger>
                    <AccordionContent className="pt-2"><AlertDialog><AlertDialogTrigger asChild><Button variant="destructive" data-testid="delete-monitor-trigger"><Trash2 className="mr-2 h-4 w-4" />Delete monitor</Button></AlertDialogTrigger><AlertDialogContent><AlertDialogHeader><AlertDialogTitle>Delete {monitor.name}?</AlertDialogTitle><AlertDialogDescription>This action cannot be undone. All history for this monitor will be lost.</AlertDialogDescription></AlertDialogHeader><AlertDialogFooter><AlertDialogCancel>Cancel</AlertDialogCancel><AlertDialogAction data-testid="delete-monitor-confirm" className="bg-destructive text-destructive-foreground hover:bg-destructive/90" onClick={async () => { await deleteMonitor(monitor.id); navigate(`/groups/${groupId}`); }}>Delete monitor</AlertDialogAction></AlertDialogFooter></AlertDialogContent></AlertDialog></AccordionContent>
                </AccordionItem>
            </Accordion>

            <AlertDialog open={Boolean(pendingGroup)} onOpenChange={open => { if (!open) setPendingGroup(null); }}>
                <AlertDialogContent>
                    <AlertDialogHeader><AlertDialogTitle>Move this monitor?</AlertDialogTitle><AlertDialogDescription>Its history and incidents will move with it. Status pages scoped to the current group will stop including it.</AlertDialogDescription></AlertDialogHeader>
                    <AlertDialogFooter><AlertDialogCancel data-testid="monitor-move-cancel">Cancel</AlertDialogCancel><AlertDialogAction data-testid="monitor-move-confirm" onClick={async () => { const destination = pendingGroup; setPendingGroup(null); if (destination && await moveMonitor(monitor.id, destination)) setGroup(destination); }}>Move monitor</AlertDialogAction></AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </div>
    );
}

function Field({ label, children, className = "" }: { label: string; children: React.ReactNode; className?: string }) {
    return <div className={`grid min-w-0 gap-2 ${className}`}><Label>{label}</Label>{children}</div>;
}

function SectionLabel({ icon, title, summary }: { icon: React.ReactElement; title: string; summary: string }) {
    return <span className="flex min-w-0 items-center gap-3 text-left"><span className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-muted text-muted-foreground [&>svg]:h-4 [&>svg]:w-4" aria-hidden="true">{icon}</span><span className="min-w-0"><span className="block text-sm font-medium text-foreground">{title}</span><span className="block truncate text-xs font-normal text-muted-foreground">{summary}</span></span></span>;
}
