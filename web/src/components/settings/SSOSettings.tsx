import { useState, useEffect, useMemo } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { useToast } from "@/components/ui/use-toast";
import { useMonitorStore } from "@/lib/store";
import { ExternalLink, CheckCircle2, XCircle, Loader2, Copy, Info } from "lucide-react";
import { Alert, AlertDescription } from "@/components/ui/alert";

export function SSOSettings() {
    const { settings, fetchSettings, updateSettings } = useMonitorStore();
    const { toast } = useToast();

    const [enabled, setEnabled] = useState(false);
    const [clientId, setClientId] = useState("");
    const [clientSecret, setClientSecret] = useState("");
    const [redirectUrl, setRedirectUrl] = useState("");
    const [allowedDomains, setAllowedDomains] = useState("");
    const [autoProvision, setAutoProvision] = useState(true);
    const [secretConfigured, setSecretConfigured] = useState(false);
    const [isTesting, setIsTesting] = useState(false);
    const [testResult, setTestResult] = useState<{ valid: boolean; message: string } | null>(null);
    const [isSaving, setIsSaving] = useState(false);

    useEffect(() => {
        fetchSettings();
    }, [fetchSettings]);

    useEffect(() => {
        if (settings) {
            setEnabled(settings["sso.google.enabled"] === "true");
            setClientId(settings["sso.google.client_id"] || "");
            setRedirectUrl(settings["sso.google.redirect_url"] || "");
            setAllowedDomains(settings["sso.google.allowed_domains"] || "");
            setAutoProvision(settings["sso.google.auto_provision"] !== "false");
            setSecretConfigured(settings["sso.google.secret_configured"] === "true");
        }
    }, [settings]);

    const handleTestConnection = async () => {
        setIsTesting(true);
        setTestResult(null);

        try {
            const res = await fetch("/api/settings/sso/test", {
                method: "POST",
                credentials: "include",
            });
            const data = await res.json();
            setTestResult(data);
        } catch {
            setTestResult({ valid: false, message: "Failed to test connection" });
        } finally {
            setIsTesting(false);
        }
    };

    const handleSave = async () => {
        setIsSaving(true);

        try {
            const settingsToUpdate: Record<string, string> = {
                "sso.google.enabled": enabled ? "true" : "false",
                "sso.google.client_id": clientId,
                "sso.google.redirect_url": redirectUrl,
                "sso.google.allowed_domains": allowedDomains,
                "sso.google.auto_provision": autoProvision ? "true" : "false",
            };

            // Only update secret if a new value was entered
            if (clientSecret) {
                settingsToUpdate["sso.google.client_secret"] = clientSecret;
            }

            await updateSettings(settingsToUpdate);
            toast({ title: "SSO Settings Saved", description: "Your Google SSO configuration has been updated." });

            // Clear the password field and update secretConfigured state
            if (clientSecret) {
                setClientSecret("");
                setSecretConfigured(true);
            }

            // Refetch to get updated state
            await fetchSettings();
        } catch {
            toast({ title: "Error", description: "Failed to save SSO settings", variant: "destructive" });
        } finally {
            setIsSaving(false);
        }
    };

    const canEnable = clientId && (secretConfigured || clientSecret);

    // Generate the callback URL for easy copy-paste
    const callbackUrl = useMemo(() => {
        const baseUrl = window.location.origin;
        return `${baseUrl}/api/auth/sso/google/callback`;
    }, []);

    const copyCallbackUrl = () => {
        navigator.clipboard.writeText(callbackUrl);
        toast({ title: "Copied", description: "Callback URL copied to clipboard" });
    };

    return (
        <Card>
            <CardHeader>
                <CardTitle>Single Sign-On (SSO)</CardTitle>
                <CardDescription>
                    Allow users to sign in with their Google account.
                </CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
                <div className="flex items-center justify-between">
                    <div className="space-y-0.5">
                        <Label>Enable Google SSO</Label>
                        <p className="text-sm text-muted-foreground">
                            Show "Sign in with Google" button on the login page
                        </p>
                    </div>
                    <Switch
                        checked={enabled}
                        onCheckedChange={setEnabled}
                        disabled={!canEnable}
                    />
                </div>

                {!canEnable && (
                    <p className="text-sm text-amber-500">
                        Configure Client ID and Client Secret to enable SSO
                    </p>
                )}

                {enabled && (
                    <Alert className="border-green-500/50 bg-green-500/10">
                        <CheckCircle2 className="h-4 w-4 text-green-500" />
                        <AlertDescription className="text-sm">
                            SSO is enabled. Users will see "Sign in with Google" on the login page.
                            Make sure to save your settings.
                        </AlertDescription>
                    </Alert>
                )}

                <Alert className="bg-muted/50">
                    <Info className="h-4 w-4" />
                    <AlertDescription className="text-sm">
                        <strong>Setup Instructions:</strong>
                        <ol className="list-decimal ml-4 mt-2 space-y-1">
                            <li>Go to <a href="https://console.cloud.google.com/apis/credentials" target="_blank" rel="noopener noreferrer" className="text-primary hover:underline">Google Cloud Console</a></li>
                            <li>Create a new OAuth 2.0 Client ID (Web application)</li>
                            <li>Add this as an authorized redirect URI:</li>
                        </ol>
                        <div className="flex items-center gap-2 mt-2 p-2 bg-background rounded border">
                            <code className="text-xs flex-1 break-all">{callbackUrl}</code>
                            <Button variant="ghost" size="sm" onClick={copyCallbackUrl} className="h-6 w-6 p-0">
                                <Copy className="h-3 w-3" />
                            </Button>
                        </div>
                    </AlertDescription>
                </Alert>

                <div className="space-y-4 pt-4 border-t">
                    <div className="flex items-center justify-between">
                        <h4 className="text-sm font-medium">Google OAuth Credentials</h4>
                        <a
                            href="https://console.cloud.google.com/apis/credentials"
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-sm text-primary hover:underline flex items-center gap-1"
                        >
                            Google Cloud Console <ExternalLink className="h-3 w-3" />
                        </a>
                    </div>

                    <div className="grid gap-2">
                        <Label htmlFor="client-id">Client ID</Label>
                        <Input
                            id="client-id"
                            type="text"
                            value={clientId}
                            onChange={(e) => setClientId(e.target.value)}
                            placeholder="xxxx.apps.googleusercontent.com"
                        />
                    </div>

                    <div className="grid gap-2">
                        <Label htmlFor="client-secret">Client Secret</Label>
                        <Input
                            id="client-secret"
                            type="password"
                            value={clientSecret}
                            onChange={(e) => setClientSecret(e.target.value)}
                            placeholder={secretConfigured ? "(configured - enter new value to change)" : "Enter client secret"}
                        />
                        {secretConfigured && !clientSecret && (
                            <p className="text-xs text-muted-foreground">
                                Client secret is configured. Enter a new value to change it.
                            </p>
                        )}
                    </div>

                    <div className="grid gap-2">
                        <Label htmlFor="redirect-url">Redirect URL (Optional)</Label>
                        <Input
                            id="redirect-url"
                            type="text"
                            value={redirectUrl}
                            onChange={(e) => setRedirectUrl(e.target.value)}
                            placeholder="/api/auth/sso/google/callback (default)"
                        />
                        <p className="text-xs text-muted-foreground">
                            Leave empty to use the default. Must match the authorized redirect URI in Google Cloud Console.
                        </p>
                    </div>
                </div>

                <div className="space-y-4 pt-4 border-t">
                    <h4 className="text-sm font-medium">Access Control</h4>

                    <div className="grid gap-2">
                        <Label htmlFor="allowed-domains">Allowed Email Domains (Optional)</Label>
                        <Input
                            id="allowed-domains"
                            type="text"
                            value={allowedDomains}
                            onChange={(e) => setAllowedDomains(e.target.value)}
                            placeholder="example.com, company.org"
                        />
                        <p className="text-xs text-muted-foreground">
                            Comma-separated list of domains. Leave empty to allow all domains.
                        </p>
                    </div>

                    <div className="flex items-center justify-between">
                        <div className="space-y-0.5">
                            <Label>Auto-provision Users</Label>
                            <p className="text-sm text-muted-foreground">
                                Automatically create accounts for new SSO users
                            </p>
                        </div>
                        <Switch
                            checked={autoProvision}
                            onCheckedChange={setAutoProvision}
                        />
                    </div>
                </div>

                <div className="flex items-center gap-4 pt-4 border-t">
                    <Button onClick={handleSave} disabled={isSaving}>
                        {isSaving ? (
                            <>
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                Saving...
                            </>
                        ) : (
                            "Save SSO Settings"
                        )}
                    </Button>

                    <Button
                        variant="outline"
                        onClick={handleTestConnection}
                        disabled={isTesting || !clientId}
                    >
                        {isTesting ? (
                            <>
                                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                                Testing...
                            </>
                        ) : (
                            "Test Configuration"
                        )}
                    </Button>

                    {testResult && (
                        <div className={`flex items-center gap-2 text-sm ${testResult.valid ? "text-green-500" : "text-red-500"}`}>
                            {testResult.valid ? (
                                <CheckCircle2 className="h-4 w-4" />
                            ) : (
                                <XCircle className="h-4 w-4" />
                            )}
                            {testResult.message}
                        </div>
                    )}
                </div>
            </CardContent>
        </Card>
    );
}
