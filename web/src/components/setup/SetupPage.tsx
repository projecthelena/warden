import { useState, useMemo } from 'react';
import { useMonitorStore } from '../../lib/store';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { AlertCircle, Check, X } from 'lucide-react';

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
        <div className="min-h-screen bg-background flex flex-col items-center justify-center p-4">
            <div className="w-full max-w-sm">
                <div className="grid gap-6">
                    {/* Branding — same as login */}
                    <div className="flex flex-col items-center gap-2 text-center">
                        <div className="flex flex-col items-center">
                            <span className="text-xl font-display font-bold tracking-tight">
                                Project <span className="font-normal text-muted-foreground">Helena</span>
                            </span>
                            <span className="text-sm font-mono font-medium text-cyan-500 tracking-widest">WARDEN</span>
                        </div>
                        <p className="text-sm text-muted-foreground" data-testid={step === 0 ? "setup-welcome" : undefined}>
                            {step === 0 && "Welcome to Warden"}
                            {step === 1 && "Create your admin account"}
                        </p>
                    </div>

                    {/* Step 1: form fields */}
                    {step === 1 && (
                        <div className="grid gap-4 animate-in fade-in slide-in-from-bottom-4 duration-500">
                            <div className="grid gap-2">
                                <Label htmlFor="username">Username</Label>
                                <Input
                                    id="username"
                                    autoFocus
                                    value={formData.username}
                                    onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                                    placeholder="e.g. admin"
                                    maxLength={32}
                                    data-testid="setup-username-input"
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="password">Password</Label>
                                <Input
                                    id="password"
                                    type="password"
                                    value={formData.password}
                                    onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                                    placeholder="Create a secure password"
                                    onKeyDown={(e) => e.key === 'Enter' && passwordCheck.isValid && handleSubmit()}
                                    data-testid="setup-password-input"
                                />

                                {/* Password requirements checklist */}
                                <div className="mt-1 p-3 rounded-lg bg-muted/50 space-y-1.5">
                                    <PasswordRequirement met={passwordCheck.minLength} label="At least 8 characters" />
                                    <PasswordRequirement met={passwordCheck.hasNumber} label="Contains a number" />
                                    <PasswordRequirement met={passwordCheck.hasSpecial} label="Contains a special character (!@#$%...)" />
                                </div>
                            </div>

                            {error && (
                                <Alert variant="destructive" className="animate-in fade-in zoom-in-95">
                                    <AlertCircle className="h-5 w-5" />
                                    <AlertDescription className="ml-2">{error}</AlertDescription>
                                </Alert>
                            )}
                        </div>
                    )}

                    {/* Actions */}
                    <div className="grid gap-4">
                        {step === 0 && (
                            <Button
                                onClick={() => setStep(1)}
                                className="w-full"
                                data-testid="setup-start-btn"
                            >
                                Get Started
                            </Button>
                        )}

                        {step === 1 && (
                            <Button
                                onClick={handleSubmit}
                                disabled={loading || !passwordCheck.isValid}
                                className="w-full"
                                data-testid="setup-launch-btn"
                            >
                                {loading ? "Launching..." : "Launch Dashboard"}
                            </Button>
                        )}
                    </div>
                </div>
            </div>

            {/* Footer — relative, not fixed */}
            <div className="mt-8 text-center text-xs text-muted-foreground">
                <p>Warden by Project Helena. Open Source.</p>
            </div>
        </div>
    );
}
