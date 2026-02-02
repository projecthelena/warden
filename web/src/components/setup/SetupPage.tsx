import { useState, useMemo } from 'react';
import { useMonitorStore } from '../../lib/store';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { AlertCircle, Activity, Check, X } from 'lucide-react';

// Password requirement checker
function usePasswordValidation(password: string) {
    return useMemo(() => ({
        minLength: password.length >= 8,
        hasNumber: /[0-9]/.test(password),
        hasSpecial: /[^a-zA-Z0-9]/.test(password),
        isValid: password.length >= 8 && /[0-9]/.test(password) && /[^a-zA-Z0-9]/.test(password)
    }), [password]);
}

// Password requirement indicator component
function PasswordRequirement({ met, label }: { met: boolean; label: string }) {
    return (
        <div className={`flex items-center gap-2 text-sm transition-colors ${met ? 'text-green-600 dark:text-green-400' : 'text-muted-foreground'}`}>
            {met ? (
                <Check className="h-4 w-4" />
            ) : (
                <X className="h-4 w-4 opacity-40" />
            )}
            <span>{label}</span>
        </div>
    );
}

export function SetupPage() {
    const { performSetup } = useMonitorStore();

    // States - simplified to 2 steps: Welcome (0) and Account (1)
    const [step, setStep] = useState(0);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    // Form Data - simplified (timezone auto-detected, no admin secret needed)
    const [formData, setFormData] = useState({
        username: 'admin', // Pre-filled for convenience
        password: '',
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
    });

    // Real-time password validation
    const passwordCheck = usePasswordValidation(formData.password);

    const handleSubmit = async () => {
        setError(null);

        // Validation
        if (!formData.username || !formData.password) {
            setError("Please fill in all fields.");
            return;
        }
        if (/[A-Z]/.test(formData.username)) {
            setError("Username must be lowercase.");
            return;
        }
        if (!/^[a-z0-9._-]+$/.test(formData.username)) {
            setError("Username contains invalid characters. Allowed: lowercase letters, numbers, ., -, _");
            return;
        }
        if (!passwordCheck.isValid) {
            setError("Please meet all password requirements.");
            return;
        }

        setLoading(true);

        try {
            const result = await performSetup(formData);
            if (result.success) {
                // Auto-login cookie is set by backend, just redirect
                window.location.href = "/";
            } else {
                const isDone = await useMonitorStore.getState().checkSetupStatus();
                if (isDone) {
                    window.location.href = "/";
                } else {
                    setError(result.error || "Setup failed. Please check logs or try again.");
                    setLoading(false);
                }
            }
        } catch (e) {
            console.error(e);
            setError("An unexpected error occurred.");
            setLoading(false);
        }
    };

    return (
        <div className="min-h-screen flex flex-col items-center justify-center bg-background p-4 sm:p-8">
            <Card className="w-full max-w-[550px] border-0 sm:border bg-card/50 sm:bg-card shadow-none sm:shadow-xl sm:ring-1 ring-border/5">
                <CardHeader className="space-y-6 pt-10 px-8 text-center">
                    <div className="mx-auto bg-primary/10 rounded-2xl w-14 h-14 flex items-center justify-center mb-2">
                        <Activity className="w-7 h-7 text-primary" />
                    </div>

                    <div className="space-y-2">
                        <h1 className="text-3xl font-bold tracking-tight" data-testid={step === 0 ? "setup-welcome" : undefined}>
                            {step === 0 && "Welcome to ClusterUptime"}
                            {step === 1 && "Create Admin Account"}
                        </h1>
                        <p className="text-muted-foreground text-lg max-w-sm mx-auto leading-relaxed">
                            {step === 0 && "Your self-hosted monitoring solution is ready in seconds."}
                            {step === 1 && "Set up your admin account to get started."}
                        </p>
                    </div>
                </CardHeader>

                {step === 1 && (
                    <CardContent className="px-8 py-6 space-y-8">
                        <div className="space-y-5 animate-in fade-in slide-in-from-bottom-4 duration-500">
                            <div className="space-y-2.5">
                                <Label htmlFor="username" className="text-base font-medium ml-1">Username</Label>
                                <Input
                                    id="username"
                                    autoFocus
                                    className="h-12 text-lg bg-background/50"
                                    value={formData.username}
                                    onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                                    placeholder="e.g. admin"
                                    maxLength={32}
                                    data-testid="setup-username-input"
                                />
                            </div>
                            <div className="space-y-2.5">
                                <Label htmlFor="password" className="text-base font-medium ml-1">Password</Label>
                                <Input
                                    id="password"
                                    type="password"
                                    className="h-12 text-lg bg-background/50"
                                    value={formData.password}
                                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                                    placeholder="Create a secure password"
                                    onKeyDown={(e) => e.key === 'Enter' && passwordCheck.isValid && handleSubmit()}
                                    data-testid="setup-password-input"
                                />

                                {/* Password requirements checklist */}
                                <div className="mt-3 p-3 rounded-lg bg-muted/50 space-y-1.5">
                                    <PasswordRequirement met={passwordCheck.minLength} label="At least 8 characters" />
                                    <PasswordRequirement met={passwordCheck.hasNumber} label="Contains a number" />
                                    <PasswordRequirement met={passwordCheck.hasSpecial} label="Contains a special character (!@#$%...)" />
                                </div>
                            </div>
                        </div>

                        {error && (
                            <Alert variant="destructive" className="animate-in fade-in zoom-in-95">
                                <AlertCircle className="h-5 w-5" />
                                <AlertDescription className="ml-2 text-base font-medium">{error}</AlertDescription>
                            </Alert>
                        )}
                    </CardContent>
                )}

                <CardFooter className="px-8 pb-10">
                    <div className="w-full space-y-4">
                        {step === 0 && (
                            <Button
                                onClick={() => setStep(1)}
                                className="w-full h-12 text-lg font-semibold rounded-lg shadow-lg shadow-primary/20"
                                data-testid="setup-start-btn"
                            >
                                Get Started
                            </Button>
                        )}

                        {step === 1 && (
                            <Button
                                onClick={handleSubmit}
                                disabled={loading || !passwordCheck.isValid}
                                className="w-full h-14 text-xl font-bold rounded-lg shadow-xl shadow-primary/20"
                                data-testid="setup-launch-btn"
                            >
                                {loading ? "Launching..." : "Launch Dashboard"}
                            </Button>
                        )}
                    </div>
                </CardFooter>
            </Card>

            <div className="fixed bottom-6 text-center text-xs text-muted-foreground animate-in fade-in duration-1000">
                <p>ClusterUptime. Self-hosted & Open Source.</p>
            </div>
        </div>
    );
}
