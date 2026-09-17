/* Hallmark · pre-emit critique: P5 H4 E4 S5 R5 V4 · macrostructure: Workbench */

import { useState } from "react";
import { CheckCircle2, ChevronDown, Loader2, Pencil, Plus, RefreshCw, Server, Trash2 } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Textarea } from "@/components/ui/textarea";
import { useToast } from "@/components/ui/use-toast";
import { DockerHost, DockerHostInput, useCreateDockerHost, useDeleteDockerHost, useDockerContainers, useDockerHosts, useTestDockerHost, useUpdateDockerHost } from "@/hooks/useDockerHosts";

interface DockerTargetFieldsProps {
    hostId: string;
    container: string;
    onHostChange: (id: string) => void;
    onContainerChange: (name: string) => void;
}

export function DockerTargetFields({ hostId, container, onHostChange, onContainerChange }: DockerTargetFieldsProps) {
    const hosts = useDockerHosts();
    const containers = useDockerContainers(hostId);

    return <div className="grid gap-5">
        <div className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
                <Label>Docker host</Label>
                <DockerHostsDialog onSaved={onHostChange} />
            </div>
            <Select value={hostId} onValueChange={value => { onHostChange(value); onContainerChange(""); }}>
                <SelectTrigger data-testid="docker-host-select"><SelectValue placeholder={hosts.isLoading ? "Loading hosts…" : "Select a Docker host"} /></SelectTrigger>
                <SelectContent>{(hosts.data ?? []).map(host => <SelectItem key={host.id} value={host.id}>{host.name}</SelectItem>)}</SelectContent>
            </Select>
            {!hosts.isLoading && hosts.data?.length === 0 && <p className="text-xs text-muted-foreground">Connect a Docker host first. You only do this once.</p>}
            {hosts.isError && <div className="flex items-center justify-between gap-3 text-xs text-destructive"><span>Could not load Docker hosts.</span><Button type="button" variant="ghost" size="sm" className="min-h-11 shrink-0 px-3 text-xs" onClick={() => hosts.refetch()}><RefreshCw className="mr-1.5 h-3.5 w-3.5" />Retry</Button></div>}
        </div>

        <div className="grid gap-2">
            <Label>Container</Label>
            <Select value={container} onValueChange={onContainerChange} disabled={!hostId || containers.isLoading || containers.isError}>
                <SelectTrigger data-testid="docker-container-select">
                    <SelectValue placeholder={!hostId ? "Select a host first" : containers.isLoading ? "Finding containers…" : "Select a container"} />
                </SelectTrigger>
                <SelectContent>{(containers.data ?? []).map(item => <SelectItem key={item.id} value={item.name}>
                    <span className="flex min-w-0 items-center gap-2"><span className={`h-2 w-2 shrink-0 rounded-full ${item.state === "running" ? "bg-emerald-500" : "bg-muted-foreground"}`} /><span className="truncate">{item.name}</span><span className="hidden truncate text-xs text-muted-foreground sm:inline">{item.image}</span></span>
                </SelectItem>)}</SelectContent>
            </Select>
            {hostId && !containers.isLoading && !containers.isError && containers.data?.length === 0 && <p className="text-xs text-muted-foreground">No containers were found on this host.</p>}
            {containers.isError && <div className="flex items-center justify-between gap-3 text-xs text-destructive"><span>{containers.error.message}</span><Button type="button" variant="ghost" size="sm" className="min-h-11 shrink-0 px-3 text-xs" onClick={() => containers.refetch()}><RefreshCw className="mr-1.5 h-3.5 w-3.5" />Retry</Button></div>}
        </div>
    </div>;
}

const localHost: DockerHostInput = { name: "Local Docker", endpoint: "unix:///var/run/docker.sock", tlsVerify: false };

function DockerHostsDialog({ onSaved }: { onSaved: (id: string) => void }) {
    const [open, setOpen] = useState(false);
    const [showForm, setShowForm] = useState(false);
    const [advanced, setAdvanced] = useState(false);
    const [editingId, setEditingId] = useState<string>();
    const [form, setForm] = useState<DockerHostInput>(localHost);
    const hosts = useDockerHosts();
    const create = useCreateDockerHost();
    const update = useUpdateDockerHost();
    const remove = useDeleteDockerHost();
    const test = useTestDockerHost();
    const { toast } = useToast();

    const beginCreate = () => {
        setEditingId(undefined);
        setForm(localHost);
        setAdvanced(false);
        setShowForm(true);
    };

    const beginEdit = (host: DockerHost) => {
        setEditingId(host.id);
        setForm({ name: host.name, endpoint: host.endpoint, tlsVerify: host.tlsVerify });
        setAdvanced(host.endpoint.startsWith("https://"));
        setShowForm(true);
    };

    const save = async () => {
        try {
            const input = { ...form, tlsVerify: form.endpoint.startsWith("https://") };
            const host = editingId
                ? await update.mutateAsync({ id: editingId, input })
                : await create.mutateAsync(input);
            onSaved(host.id);
            setShowForm(false);
            try {
                await test.mutateAsync(host.id);
                setOpen(false);
                toast({ title: editingId ? "Docker host updated" : "Docker host connected", description: "Connection verified. Now choose the container to monitor." });
            } catch (error) {
                toast({ title: "Host saved, but Warden cannot connect", description: (error as Error).message, variant: "destructive" });
            }
        } catch (error) {
            toast({ title: "Could not save Docker host", description: (error as Error).message, variant: "destructive" });
        }
    };

    const testHost = async (id: string) => {
        try {
            await test.mutateAsync(id);
            toast({ title: "Connection successful", description: "Warden can reach this Docker host." });
        } catch (error) {
            toast({ title: "Connection failed", description: (error as Error).message, variant: "destructive" });
        }
    };

    return <Dialog open={open} onOpenChange={setOpen}>
        <DialogTrigger asChild><Button type="button" variant="ghost" size="sm" className="min-h-11 px-3 text-xs"><Server className="mr-1.5 h-3.5 w-3.5" />Manage hosts</Button></DialogTrigger>
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-xl">
            <DialogHeader><DialogTitle>Docker hosts</DialogTitle><DialogDescription>Connect each Docker Engine once, then reuse it across monitors.</DialogDescription></DialogHeader>

            {!showForm ? <div className="space-y-3 py-2">
                {(hosts.data ?? []).map(host => <div key={host.id} className="flex flex-col gap-3 rounded-lg border border-border p-3 sm:flex-row sm:items-center">
                    <div className="flex min-w-0 flex-1 items-center gap-3"><Server className="h-4 w-4 shrink-0 text-muted-foreground" /><div className="min-w-0"><p className="text-sm font-medium">{host.name}</p><p className="truncate font-mono text-xs text-muted-foreground">{host.endpoint}</p></div></div>
                    <div className="flex items-center justify-end gap-1">
                        <Button type="button" variant="outline" size="sm" className="min-h-11 whitespace-nowrap" onClick={() => testHost(host.id)} disabled={test.isPending}>{test.isPending && test.variables === host.id ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <CheckCircle2 className="mr-1.5 h-3.5 w-3.5" />}Test</Button>
                        <Button type="button" variant="ghost" size="icon" className="h-11 w-11" aria-label={`Edit ${host.name}`} onClick={() => beginEdit(host)}><Pencil className="h-4 w-4" /></Button>
                        <Button type="button" variant="ghost" size="icon" className="h-11 w-11" aria-label={`Delete ${host.name}`} disabled={remove.isPending} onClick={async () => { try { await remove.mutateAsync(host.id); } catch (error) { toast({ title: "Host is still in use", description: (error as Error).message, variant: "destructive" }); } }}><Trash2 className="h-4 w-4" /></Button>
                    </div>
                </div>)}
                {hosts.data?.length === 0 && <div className="rounded-lg border border-dashed border-border p-6 text-center"><Server className="mx-auto mb-2 h-5 w-5 text-muted-foreground" /><p className="text-sm font-medium">No Docker hosts yet</p><p className="mt-1 text-xs text-muted-foreground">Connect one once, then reuse it for every container monitor.</p></div>}
                <Button type="button" variant="outline" className="min-h-11 w-full" onClick={beginCreate}><Plus className="mr-2 h-4 w-4" />Connect Docker host</Button>
            </div> : <div className="space-y-4 py-2">
                <div className="grid gap-2"><Label htmlFor="docker-host-name">Name</Label><Input id="docker-host-name" value={form.name} onChange={event => setForm({ ...form, name: event.target.value })} placeholder="Local Docker" /></div>
                <div className="grid gap-2"><Label htmlFor="docker-host-endpoint">Endpoint</Label><Input id="docker-host-endpoint" value={form.endpoint} onChange={event => setForm({ ...form, endpoint: event.target.value, tlsVerify: event.target.value.startsWith("https://") })} className="font-mono text-xs" placeholder="unix:///var/run/docker.sock" /><p className="text-xs text-muted-foreground">Use the local socket, a read-only socket proxy, or an HTTPS Docker API endpoint.</p></div>
                <Alert><Server className="h-4 w-4" /><AlertTitle>Prefer a socket proxy</AlertTitle><AlertDescription>Direct Docker socket access is powerful. A proxy limited to read-only container endpoints is safer.</AlertDescription></Alert>
                {form.endpoint.startsWith("https://") && <Button type="button" variant="ghost" className="min-h-11 w-full justify-between px-3 text-sm text-muted-foreground" onClick={() => setAdvanced(value => !value)}><span>{advanced ? "Hide mTLS certificates" : editingId ? "Replace mTLS certificates" : "Add mTLS certificates"}</span><ChevronDown className={`h-4 w-4 transition-transform ${advanced ? "rotate-180" : ""}`} /></Button>}
                {advanced && <div className="space-y-4 rounded-lg border border-border bg-muted/20 p-3">
                    <p className="text-xs text-muted-foreground">HTTPS always verifies the server certificate. Leave these blank while editing to keep the saved certificates.</p>
                    <CertificateField label="CA certificate" value={form.caCert ?? ""} onChange={caCert => setForm({ ...form, caCert })} />
                    <CertificateField label="Client certificate" value={form.clientCert ?? ""} onChange={clientCert => setForm({ ...form, clientCert })} />
                    <CertificateField label="Client private key" value={form.clientKey ?? ""} onChange={clientKey => setForm({ ...form, clientKey })} />
                </div>}
            </div>}

            {showForm && <DialogFooter><Button type="button" variant="outline" className="min-h-11" onClick={() => setShowForm(false)}>Back</Button><Button type="button" className="min-h-11" onClick={save} disabled={create.isPending || update.isPending || test.isPending || !form.name.trim() || !form.endpoint.trim()}>{(create.isPending || update.isPending || test.isPending) && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}{test.isPending ? "Testing connection…" : editingId ? "Save changes" : "Save and test"}</Button></DialogFooter>}
        </DialogContent>
    </Dialog>;
}

function CertificateField({ label, value, onChange }: { label: string; value: string; onChange: (value: string) => void }) {
    const id = `docker-${label.toLowerCase().replaceAll(" ", "-")}`;
    return <div className="grid gap-2"><Label htmlFor={id}>{label}</Label><Textarea id={id} value={value} onChange={event => onChange(event.target.value)} className="min-h-20 resize-y font-mono text-[11px]" placeholder="-----BEGIN CERTIFICATE-----" /></div>;
}
