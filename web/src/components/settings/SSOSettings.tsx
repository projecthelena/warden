import { useCallback, useEffect, useMemo, useState } from "react";
import {
    ArrowDown,
    ArrowUp,
    CheckCircle2,
    Copy,
    MoreHorizontal,
    Plus,
    ShieldCheck,
    Trash2,
} from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
} from "@/components/ui/sheet";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/components/ui/use-toast";

type ProviderTemplate = "google" | "oidc";
type SSOProvider = {
    id: string;
    template: ProviderTemplate;
    name: string;
    issuerUrl: string;
    clientId: string;
    secretConfigured: boolean;
    allowedDomains: string;
    autoProvision: boolean;
    enabled: boolean;
    sortOrder: number;
};

type ProviderForm = {
    template: ProviderTemplate;
    name: string;
    issuerUrl: string;
    clientId: string;
    clientSecret: string;
    allowedDomains: string;
    autoProvision: boolean;
    enabled: boolean;
};

const emptyProvider: ProviderForm = {
    template: "google",
    name: "Google",
    issuerUrl: "https://accounts.google.com",
    clientId: "",
    clientSecret: "",
    allowedDomains: "",
    autoProvision: true,
    enabled: false,
};

async function api<T>(url: string, options?: RequestInit): Promise<T> {
    const response = await fetch(url, {
        credentials: "include",
        ...options,
        headers: { "Content-Type": "application/json", ...options?.headers },
    });
    if (!response.ok) {
        const body = await response.json().catch(() => ({}));
        throw new Error(body.error || "The request failed.");
    }
    return response.status === 204 ? (undefined as T) : response.json();
}

export function SSOSettings() {
    const { toast } = useToast();
    const [providers, setProviders] = useState<SSOProvider[]>([]);
    const [loading, setLoading] = useState(true);
    const [sheetOpen, setSheetOpen] = useState(false);
    const [editing, setEditing] = useState<SSOProvider | null>(null);
    const [form, setForm] = useState<ProviderForm>(emptyProvider);
    const [saving, setSaving] = useState(false);
    const [testing, setTesting] = useState(false);
    const [deleteTarget, setDeleteTarget] = useState<SSOProvider | null>(null);

    const loadProviders = useCallback(async () => {
        try {
            const result = await api<{ providers: SSOProvider[] }>(
                "/api/sso/providers",
            );
            setProviders(result.providers);
        } catch (error) {
            toast({
                title: "Could not load identity providers",
                description: message(error),
                variant: "destructive",
            });
        } finally {
            setLoading(false);
        }
    }, [toast]);

    useEffect(() => {
        loadProviders();
    }, [loadProviders]);

    const callbackUrl = useMemo(
        () =>
            `${window.location.origin}/api/auth/sso/${editing?.id || "provider-id"}/callback`,
        [editing],
    );

    const openCreate = () => {
        setEditing(null);
        setForm(emptyProvider);
        setSheetOpen(true);
    };

    const openEdit = (provider: SSOProvider) => {
        setEditing(provider);
        setForm({
            template: provider.template,
            name: provider.name,
            issuerUrl: provider.issuerUrl,
            clientId: provider.clientId,
            clientSecret: "",
            allowedDomains: provider.allowedDomains,
            autoProvision: provider.autoProvision,
            enabled: provider.enabled,
        });
        setSheetOpen(true);
    };

    const changeTemplate = (template: ProviderTemplate) => {
        setForm((current) => ({
            ...current,
            template,
            name:
                current.name === "Google" || current.name === "OpenID Connect"
                    ? template === "google"
                        ? "Google"
                        : "OpenID Connect"
                    : current.name,
            issuerUrl:
                template === "google" ? "https://accounts.google.com" : "",
        }));
    };

    const save = async () => {
        setSaving(true);
        try {
            const provider = await api<SSOProvider>(
                editing
                    ? `/api/sso/providers/${editing.id}`
                    : "/api/sso/providers",
                {
                    method: editing ? "PUT" : "POST",
                    body: JSON.stringify(form),
                },
            );
            toast({
                title: editing
                    ? "Identity provider updated"
                    : "Identity provider added",
            });
            setSheetOpen(false);
            if (editing) {
                setProviders((items) =>
                    items.map((item) =>
                        item.id === provider.id ? provider : item,
                    ),
                );
            } else {
                await loadProviders();
            }
        } catch (error) {
            toast({
                title: "Could not save identity provider",
                description: message(error),
                variant: "destructive",
            });
        } finally {
            setSaving(false);
        }
    };

    const testConnection = async () => {
        if (!editing) return;
        setTesting(true);
        try {
            const result = await api<{ valid: boolean; message: string }>(
                `/api/sso/providers/${editing.id}/test`,
                { method: "POST" },
            );
            toast({
                title: result.valid
                    ? "Connection successful"
                    : "Connection failed",
                description: result.message,
                variant: result.valid ? "default" : "destructive",
            });
        } catch (error) {
            toast({
                title: "Connection failed",
                description: message(error),
                variant: "destructive",
            });
        } finally {
            setTesting(false);
        }
    };

    const remove = async () => {
        if (!deleteTarget) return;
        try {
            await api<void>(`/api/sso/providers/${deleteTarget.id}`, {
                method: "DELETE",
            });
            setProviders((items) =>
                items.filter((item) => item.id !== deleteTarget.id),
            );
            toast({ title: "Identity provider removed" });
        } catch (error) {
            toast({
                title: "Could not remove identity provider",
                description: message(error),
                variant: "destructive",
            });
        } finally {
            setDeleteTarget(null);
        }
    };

    const move = async (index: number, offset: -1 | 1) => {
        const next = [...providers];
        const target = index + offset;
        if (target < 0 || target >= next.length) return;
        [next[index], next[target]] = [next[target], next[index]];
        setProviders(next);
        try {
            await api<void>("/api/sso/providers/order", {
                method: "PUT",
                body: JSON.stringify({
                    ids: next.map((provider) => provider.id),
                }),
            });
        } catch (error) {
            setProviders(providers);
            toast({
                title: "Could not change provider order",
                description: message(error),
                variant: "destructive",
            });
        }
    };

    return (
        <>
            <Card>
                <CardHeader className="flex-row items-start justify-between gap-4 space-y-0">
                    <div className="space-y-1.5">
                        <CardTitle>Identity providers</CardTitle>
                        <CardDescription>
                            Add one or more OpenID Connect providers and choose
                            their order on the sign-in page.
                        </CardDescription>
                    </div>
                    <Button onClick={openCreate} data-testid="sso-add-provider">
                        <Plus className="mr-2 h-4 w-4" />
                        Add provider
                    </Button>
                </CardHeader>
                <CardContent>
                    {loading ? (
                        <p className="text-sm text-muted-foreground">
                            Loading identity providers…
                        </p>
                    ) : providers.length === 0 ? (
                        <div className="rounded-lg border border-dashed p-8 text-center">
                            <ShieldCheck className="mx-auto mb-3 h-8 w-8 text-muted-foreground" />
                            <p className="font-medium">
                                No identity providers yet
                            </p>
                            <p className="mt-1 text-sm text-muted-foreground">
                                Add Google or any standards-compliant OpenID
                                Connect provider.
                            </p>
                            <Button
                                variant="outline"
                                className="mt-4"
                                onClick={openCreate}
                            >
                                Add your first provider
                            </Button>
                        </div>
                    ) : (
                        <div className="divide-y rounded-lg border">
                            {providers.map((provider, index) => (
                                <div
                                    key={provider.id}
                                    className="flex items-center gap-3 p-4"
                                    data-testid={`sso-provider-${provider.id}`}
                                >
                                    <div className="min-w-0 flex-1">
                                        <div className="flex flex-wrap items-center gap-2">
                                            <Button
                                                variant="link"
                                                className="h-auto truncate p-0 font-medium"
                                                onClick={() =>
                                                    openEdit(provider)
                                                }
                                            >
                                                {provider.name}
                                            </Button>
                                            <Badge
                                                variant={
                                                    provider.enabled
                                                        ? "default"
                                                        : "secondary"
                                                }
                                            >
                                                {provider.enabled
                                                    ? "Enabled"
                                                    : "Disabled"}
                                            </Badge>
                                            <Badge variant="outline">
                                                {provider.template === "google"
                                                    ? "Google"
                                                    : "OpenID Connect"}
                                            </Badge>
                                        </div>
                                        <p className="mt-1 truncate text-sm text-muted-foreground">
                                            {provider.issuerUrl}
                                        </p>
                                    </div>
                                    <div className="flex items-center gap-1">
                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            aria-label={`Move ${provider.name} up`}
                                            disabled={index === 0}
                                            onClick={() => move(index, -1)}
                                        >
                                            <ArrowUp className="h-4 w-4" />
                                        </Button>
                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            aria-label={`Move ${provider.name} down`}
                                            disabled={
                                                index === providers.length - 1
                                            }
                                            onClick={() => move(index, 1)}
                                        >
                                            <ArrowDown className="h-4 w-4" />
                                        </Button>
                                        <DropdownMenu>
                                            <DropdownMenuTrigger asChild>
                                                <Button
                                                    variant="ghost"
                                                    size="icon"
                                                    aria-label={`Actions for ${provider.name}`}
                                                >
                                                    <MoreHorizontal className="h-4 w-4" />
                                                </Button>
                                            </DropdownMenuTrigger>
                                            <DropdownMenuContent align="end">
                                                <DropdownMenuItem
                                                    onClick={() =>
                                                        openEdit(provider)
                                                    }
                                                >
                                                    Edit
                                                </DropdownMenuItem>
                                                <DropdownMenuSeparator />
                                                <DropdownMenuItem
                                                    className="text-destructive focus:text-destructive"
                                                    onClick={() =>
                                                        setDeleteTarget(
                                                            provider,
                                                        )
                                                    }
                                                >
                                                    <Trash2 className="mr-2 h-4 w-4" />
                                                    Remove
                                                </DropdownMenuItem>
                                            </DropdownMenuContent>
                                        </DropdownMenu>
                                    </div>
                                </div>
                            ))}
                        </div>
                    )}
                </CardContent>
            </Card>

            <Sheet open={sheetOpen} onOpenChange={setSheetOpen}>
                <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
                    <SheetHeader>
                        <SheetTitle>
                            {editing
                                ? `Edit ${editing.name}`
                                : "Add identity provider"}
                        </SheetTitle>
                        <SheetDescription>
                            Users authenticate with the provider; Warden stores
                            the account, role, and session.
                        </SheetDescription>
                    </SheetHeader>
                    <div className="space-y-6 py-6">
                        <section className="space-y-4">
                            <SectionTitle
                                title="Identity"
                                description="Choose a template and the label shown on the sign-in page."
                            />
                            <div className="grid gap-2">
                                <Label htmlFor="sso-template">Template</Label>
                                <Select
                                    value={form.template}
                                    onValueChange={(value) =>
                                        changeTemplate(
                                            value as ProviderTemplate,
                                        )
                                    }
                                >
                                    <SelectTrigger
                                        id="sso-template"
                                        data-testid="sso-template"
                                    >
                                        <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
                                        <SelectItem value="google">
                                            Google
                                        </SelectItem>
                                        <SelectItem value="oidc">
                                            OpenID Connect
                                        </SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="sso-name">Display name</Label>
                                <Input
                                    id="sso-name"
                                    data-testid="sso-name"
                                    value={form.name}
                                    onChange={(event) =>
                                        setForm({
                                            ...form,
                                            name: event.target.value,
                                        })
                                    }
                                    placeholder="Company SSO"
                                />
                            </div>
                        </section>
                        <section className="space-y-4 border-t pt-6">
                            <SectionTitle
                                title="Connection"
                                description="Copy the callback URL into the provider, then enter its client credentials."
                            />
                            <div className="grid gap-2">
                                <Label htmlFor="sso-callback">
                                    Callback URL
                                </Label>
                                <div className="flex gap-2">
                                    <Input
                                        id="sso-callback"
                                        readOnly
                                        value={callbackUrl}
                                        className="font-mono text-xs"
                                    />
                                    <Button
                                        variant="outline"
                                        size="icon"
                                        aria-label="Copy callback URL"
                                        onClick={() => {
                                            navigator.clipboard.writeText(
                                                callbackUrl,
                                            );
                                            toast({
                                                title: "Callback URL copied",
                                            });
                                        }}
                                    >
                                        <Copy className="h-4 w-4" />
                                    </Button>
                                </div>
                                {!editing && (
                                    <p className="text-xs text-muted-foreground">
                                        Save once to get the final callback URL.
                                    </p>
                                )}
                            </div>
                            {form.template === "oidc" && (
                                <div className="grid gap-2">
                                    <Label htmlFor="sso-issuer">
                                        Issuer URL
                                    </Label>
                                    <Input
                                        id="sso-issuer"
                                        data-testid="sso-issuer"
                                        type="url"
                                        value={form.issuerUrl}
                                        onChange={(event) =>
                                            setForm({
                                                ...form,
                                                issuerUrl: event.target.value,
                                            })
                                        }
                                        placeholder="https://id.example.com/realms/company"
                                    />
                                    <p className="text-xs text-muted-foreground">
                                        Use the exact issuer from the provider's
                                        OpenID configuration.
                                    </p>
                                </div>
                            )}
                            <div className="grid gap-2">
                                <Label htmlFor="sso-client-id">Client ID</Label>
                                <Input
                                    id="sso-client-id"
                                    data-testid="sso-client-id"
                                    value={form.clientId}
                                    onChange={(event) =>
                                        setForm({
                                            ...form,
                                            clientId: event.target.value,
                                        })
                                    }
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="sso-client-secret">
                                    Client secret
                                </Label>
                                <Input
                                    id="sso-client-secret"
                                    data-testid="sso-client-secret"
                                    type="password"
                                    value={form.clientSecret}
                                    onChange={(event) =>
                                        setForm({
                                            ...form,
                                            clientSecret: event.target.value,
                                        })
                                    }
                                    placeholder={
                                        editing?.secretConfigured
                                            ? "Configured — enter a new value to replace it"
                                            : "Client secret"
                                    }
                                />
                            </div>
                        </section>
                        <section className="space-y-4 border-t pt-6">
                            <SectionTitle
                                title="Access"
                                description="New SSO users start as viewers. An admin can change their role in Users."
                            />
                            <div className="grid gap-2">
                                <Label htmlFor="sso-domains">
                                    Allowed email domains (optional)
                                </Label>
                                <Input
                                    id="sso-domains"
                                    value={form.allowedDomains}
                                    onChange={(event) =>
                                        setForm({
                                            ...form,
                                            allowedDomains: event.target.value,
                                        })
                                    }
                                    placeholder="example.com, company.org"
                                />
                            </div>
                            <Toggle
                                id="sso-auto-provision"
                                label="Auto-provision users"
                                description="Create a viewer after the first verified sign-in."
                                checked={form.autoProvision}
                                onChange={(autoProvision) =>
                                    setForm({ ...form, autoProvision })
                                }
                            />
                            <Toggle
                                id="sso-enabled"
                                label="Enable provider"
                                description="Show this provider on the sign-in page."
                                checked={form.enabled}
                                onChange={(enabled) =>
                                    setForm({ ...form, enabled })
                                }
                            />
                            {form.enabled &&
                                (!form.clientId ||
                                    (!editing?.secretConfigured &&
                                        !form.clientSecret)) && (
                                    <Alert>
                                        <AlertDescription>
                                            Add a client ID and secret before
                                            enabling this provider.
                                        </AlertDescription>
                                    </Alert>
                                )}
                            {editing?.secretConfigured && (
                                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                                    <CheckCircle2 className="h-4 w-4 text-emerald-500" />
                                    A client secret is configured.
                                </p>
                            )}
                        </section>
                    </div>
                    <SheetFooter className="gap-2 sm:justify-between">
                        <Button
                            variant="outline"
                            onClick={testConnection}
                            disabled={!editing || testing}
                        >
                            {testing ? "Testing…" : "Test connection"}
                        </Button>
                        <Button
                            onClick={save}
                            disabled={
                                saving ||
                                !form.name ||
                                (form.template === "oidc" && !form.issuerUrl)
                            }
                            data-testid="sso-save"
                        >
                            {saving ? "Saving…" : "Save provider"}
                        </Button>
                    </SheetFooter>
                </SheetContent>
            </Sheet>

            <AlertDialog
                open={deleteTarget !== null}
                onOpenChange={(open) => !open && setDeleteTarget(null)}
            >
                <AlertDialogContent>
                    <AlertDialogHeader>
                        <AlertDialogTitle>
                            Remove {deleteTarget?.name}?
                        </AlertDialogTitle>
                        <AlertDialogDescription>
                            This removes its configuration. If users are linked
                            to it, Warden will ask you to disable it instead so
                            they can be managed safely.
                        </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                            onClick={remove}
                            className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        >
                            Remove provider
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </>
    );
}

function SectionTitle({
    title,
    description,
}: {
    title: string;
    description: string;
}) {
    return (
        <div>
            <h3 className="text-sm font-medium">{title}</h3>
            <p className="text-sm text-muted-foreground">{description}</p>
        </div>
    );
}
function Toggle({
    id,
    label,
    description,
    checked,
    onChange,
}: {
    id: string;
    label: string;
    description: string;
    checked: boolean;
    onChange: (value: boolean) => void;
}) {
    return (
        <div className="flex items-center justify-between gap-4">
            <div>
                <Label htmlFor={id}>{label}</Label>
                <p className="text-sm text-muted-foreground">{description}</p>
            </div>
            <Switch
                id={id}
                data-testid={id}
                checked={checked}
                onCheckedChange={onChange}
            />
        </div>
    );
}
function message(error: unknown) {
    return error instanceof Error ? error.message : "The request failed.";
}
