import { useEffect, useMemo, useState } from "react";
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Button } from "@/components/ui/button";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";

export function OIDCSettings() {
    const { settings, fetchSettings, updateSettings } = useMonitorStore();
    const { toast } = useToast();
    const [enabled, setEnabled] = useState(false);
    const [issuerUrl, setIssuerUrl] = useState("");
    const [clientId, setClientId] = useState("");
    const [clientSecret, setClientSecret] = useState("");
    const [providerName, setProviderName] = useState("SSO");
    const [allowedDomains, setAllowedDomains] = useState("");
    const [autoProvision, setAutoProvision] = useState(true);
    const [secretConfigured, setSecretConfigured] = useState(false);
    const [saving, setSaving] = useState(false);
    const callbackUrl = useMemo(
        () => `${window.location.origin}/api/auth/sso/oidc/callback`,
        [],
    );

    useEffect(() => {
        fetchSettings();
    }, [fetchSettings]);

    useEffect(() => {
        if (!settings) return;

        setEnabled(settings["sso.oidc.enabled"] === "true");
        setIssuerUrl(settings["sso.oidc.issuer_url"] || "");
        setClientId(settings["sso.oidc.client_id"] || "");
        setProviderName(settings["sso.oidc.provider_name"] || "SSO");
        setAllowedDomains(settings["sso.oidc.allowed_domains"] || "");
        setAutoProvision(settings["sso.oidc.auto_provision"] !== "false");
        setSecretConfigured(settings["sso.oidc.secret_configured"] === "true");
    }, [settings]);

    const save = async () => {
        const values: Record<string, string> = {
            "sso.oidc.enabled": enabled ? "true" : "false",
            "sso.oidc.issuer_url": issuerUrl,
            "sso.oidc.client_id": clientId,
            "sso.oidc.provider_name": providerName,
            "sso.oidc.allowed_domains": allowedDomains,
            "sso.oidc.auto_provision": autoProvision ? "true" : "false",
        };
        if (clientSecret) values["sso.oidc.client_secret"] = clientSecret;
        setSaving(true);
        try {
            await updateSettings(values);
            if (clientSecret) {
                setClientSecret("");
                setSecretConfigured(true);
            }
            toast({ title: "OIDC settings saved" });
            await fetchSettings();
        } catch (error) {
            toast({
                title: "Could not save OIDC settings",
                description: error instanceof Error ? error.message : "The settings were rejected.",
                variant: "destructive",
            });
        } finally {
            setSaving(false);
        }
    };

    const canEnable =
        issuerUrl && clientId && (secretConfigured || clientSecret);
    return (
        <Card>
            <CardHeader>
                <CardTitle>Generic OpenID Connect</CardTitle>
                <CardDescription>
                    Connect Keycloak, Authentik, Authelia, Dex, Okta, Entra ID,
                    or any standards-compliant OIDC provider.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-5">
                <div className="flex items-center justify-between">
                    <div>
                        <Label>Enable OIDC</Label>
                        <p className="text-sm text-muted-foreground">
                            Show a generic SSO button on the login page.
                        </p>
                    </div>
                    <Switch
                        aria-label="Enable OIDC"
                        checked={enabled}
                        onCheckedChange={setEnabled}
                        disabled={!canEnable}
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Provider name</Label>
                    <Input
                        data-testid="oidc-provider-name"
                        value={providerName}
                        onChange={(e) => setProviderName(e.target.value)}
                        placeholder="Company SSO"
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Issuer URL</Label>
                    <Input
                        data-testid="oidc-issuer-url"
                        type="url"
                        value={issuerUrl}
                        onChange={(e) => setIssuerUrl(e.target.value)}
                        placeholder="https://auth.example.com/application/o/warden/"
                    />
                    <p className="text-xs text-muted-foreground">
                        Warden discovers the authorization, token, and
                        signing-key endpoints from this issuer.
                    </p>
                </div>
                <div className="grid gap-2">
                    <Label>Client ID</Label>
                    <Input
                        data-testid="oidc-client-id"
                        value={clientId}
                        onChange={(e) => setClientId(e.target.value)}
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Client Secret</Label>
                    <Input
                        data-testid="oidc-client-secret"
                        type="password"
                        value={clientSecret}
                        onChange={(e) => setClientSecret(e.target.value)}
                        placeholder={
                            secretConfigured
                                ? "Configured — enter a new value to replace"
                                : "Client secret"
                        }
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Redirect URI</Label>
                    <Input
                        data-testid="oidc-allowed-domains"
                        readOnly
                        value={callbackUrl}
                        className="font-mono text-xs"
                    />
                </div>
                <div className="grid gap-2">
                    <Label>Allowed email domains (optional)</Label>
                    <Input
                        value={allowedDomains}
                        onChange={(e) => setAllowedDomains(e.target.value)}
                        placeholder="example.com, company.org"
                    />
                </div>
                <div className="flex items-center justify-between">
                    <div>
                        <Label>Auto-provision users</Label>
                        <p className="text-sm text-muted-foreground">
                            New verified identities become viewers. A Warden admin can promote them after their first login.
                        </p>
                    </div>
                    <Switch
                        aria-label="Auto-provision users"
                        checked={autoProvision}
                        onCheckedChange={setAutoProvision}
                    />
                </div>
                <p className="text-xs text-muted-foreground">
                    LDAP is supported through an OIDC identity provider such as
                    Keycloak or Authentik; Warden does not bind directly to
                    LDAP.
                </p>
                <Button onClick={save} disabled={saving} data-testid="oidc-save">
                    {saving ? "Saving…" : "Save OIDC Settings"}
                </Button>
            </CardContent>
        </Card>
    );
}
