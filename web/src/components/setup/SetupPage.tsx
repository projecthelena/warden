import { useState } from 'react';
import { useMonitorStore } from '../../lib/store';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Card, CardContent, CardFooter, CardHeader } from '@/components/ui/card';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Checkbox } from '@/components/ui/checkbox';
import { AlertCircle, Activity } from 'lucide-react';
import { SelectTimezone } from '@/components/ui/select-timezone';

export function SetupPage() {
    const { performSetup, login } = useMonitorStore();

    // States
    const [step, setStep] = useState(0); // 0: Welcome, 1: Account, 2: Timezone, 3: Defaults
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    // Form Data
    const [formData, setFormData] = useState({
        username: '',
        password: '',
        timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC', // Auto-detect default
        createDefaults: true
    });

    const nextStep = () => {
        setError(null);
        if (step === 1) {
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
            if (formData.password.length < 8) {
                setError("Password must be at least 8 characters.");
                return;
            }
            if (!/[0-9]/.test(formData.password)) {
                setError("Password must contain at least one number.");
                return;
            }
            if (!/[^a-zA-Z0-9]/.test(formData.password)) {
                setError("Password must contain at least one special character.");
                return;
            }
        }
        setStep(s => s + 1);
    };

    const handleSubmit = async () => {
        setError(null);
        setLoading(true);

        try {
            const result = await performSetup(formData);
            if (result.success) {
                const loginRes = await login(formData.username, formData.password);
                if (!loginRes.success) {
                    setError("Setup successful but login failed: " + loginRes.error);
                    setLoading(false);
                    return;
                }
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
                            {step === 2 && "Select Timezone"}
                            {step === 3 && "Almost Done"}
                        </h1>
                        <p className="text-muted-foreground text-lg max-w-sm mx-auto leading-relaxed">
                            {step === 0 && "Your self-hosted monitoring solution is just steps away."}
                            {step === 1 && "Secure your instance with a new admin account."}
                            {step === 2 && "Configure the default timezone for your dashboards."}
                            {step === 3 && "Review your settings and launch the platform."}
                        </p>
                    </div>

                    {step > 0 && (
                        <div className="flex items-center justify-center gap-2 mt-2">
                            {[
                                { num: 1, label: "Account" },
                                { num: 2, label: "Timezone" },
                                { num: 3, label: "Finish" },
                            ].map((s, idx) => {
                                const isActive = step === s.num;
                                const isCompleted = step > s.num;
                                return (
                                    <div key={s.num} className="flex items-center">
                                        <div className={`flex items-center gap-2 px-3 py-1 rounded-full transition-colors ${isActive ? "bg-primary/10 text-primary font-medium" :
                                            isCompleted ? "text-primary/70" : "text-muted-foreground/40"
                                            }`}>
                                            <span className="text-sm">{s.label}</span>
                                        </div>
                                        {idx < 2 && (
                                            <div className="w-4 h-[1px] bg-border mx-1" />
                                        )}
                                    </div>
                                )
                            })}
                        </div>
                    )}
                </CardHeader>

                {step > 0 && (
                    <CardContent className="px-8 py-6 space-y-8">
                        {step === 1 && (
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
                                        placeholder="Min 8 chars, 1 number, 1 special char"
                                        onKeyDown={(e) => e.key === 'Enter' && nextStep()}
                                        data-testid="setup-password-input"
                                    />
                                </div>
                            </div>
                        )}

                        {step === 2 && (
                            <div className="space-y-4 animate-in fade-in slide-in-from-bottom-4 duration-500">
                                <div className="space-y-2.5">
                                    <Label className="text-base font-medium ml-1">Timezone</Label>
                                    <SelectTimezone
                                        value={formData.timezone}
                                        onValueChange={(val) => setFormData({ ...formData, timezone: val })}
                                        className="h-12 text-lg w-full bg-background/50"
                                    />
                                    <p className="text-sm text-muted-foreground ml-1">
                                        Current detected: {Intl.DateTimeFormat().resolvedOptions().timeZone}
                                    </p>
                                </div>
                            </div>
                        )}

                        {step === 3 && (
                            <div className="space-y-6 animate-in fade-in slide-in-from-bottom-4 duration-500">
                                <div className="flex items-start space-x-4 p-5 rounded-xl border bg-muted/30 hover:bg-muted/50 transition-colors cursor-pointer" onClick={() => setFormData(f => ({ ...f, createDefaults: !f.createDefaults }))}>
                                    <Checkbox
                                        id="createDefaults"
                                        checked={formData.createDefaults}
                                        onCheckedChange={(checked) => setFormData({ ...formData, createDefaults: checked as boolean })}
                                        className="mt-1"
                                    />
                                    <div className="space-y-1 select-none">
                                        <Label htmlFor="createDefaults" className="text-lg font-semibold cursor-pointer">
                                            Create Default Monitors
                                        </Label>
                                        <p className="text-muted-foreground leading-relaxed">
                                            Bootstrap your experience with example monitors for Google, GitHub, and major DNS providers.
                                        </p>
                                    </div>
                                </div>
                            </div>
                        )}

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

                        {(step === 1 || step === 2) && (
                            <Button
                                onClick={nextStep}
                                className="w-full h-12 text-lg font-medium rounded-lg"
                                data-testid={step === 2 ? "setup-continue-btn-2" : "setup-continue-btn"}
                            >
                                Continue
                            </Button>
                        )}

                        {step === 3 && (
                            <Button
                                onClick={handleSubmit}
                                disabled={loading}
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
                <p>© {new Date().getFullYear()} ClusterUptime. Self-hosted & Open Source.</p>
            </div>
        </div>
    );
}
