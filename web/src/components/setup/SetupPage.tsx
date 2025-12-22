import { useState } from 'react';
import { useMonitorStore } from '../../lib/store';

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AlertCircle, ArrowRight, Check, Rocket, Globe, Clock, ShieldCheck } from 'lucide-react';
import { motion, AnimatePresence } from "framer-motion";
import { cn } from "@/lib/utils";

import { SelectTimezone } from '@/components/ui/select-timezone';

export function SetupPage() {
    const { performSetup, login } = useMonitorStore();

    // States
    const [step, setStep] = useState(0); // 0: Welcome, 1: Account, 2: Timezone, 3: Defaults
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    // Form Data
    const [formData, setFormData] = useState({
        username: 'admin',
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
            const success = await performSetup(formData);
            if (success) {
                // Login implicitly or explicitly?
                // Try to login, if successful, reload. If setup was success, backend blocked re-setup.
                await login(formData.username, formData.password);

                // Force reload to clear all states and re-run App.tsx logic fresh
                window.location.href = "/";
            } else {
                // If success is false, maybe it failed or maybe it's already set up?
                // Check status
                const isDone = await useMonitorStore.getState().checkSetupStatus();
                if (isDone) {
                    window.location.href = "/";
                } else {
                    setError("Setup failed. Please check logs or try again.");
                    setLoading(false);
                }
            }
        } catch (e) {
            console.error(e);
            setError("An unexpected error occurred.");
            setLoading(false);
        }
    };

    // Animation variants
    // Animation variants

    return (
        <div className="min-h-screen bg-[#020617] text-slate-100 flex flex-col items-center justify-center p-6 relative overflow-hidden font-sans selection:bg-blue-500/30">
            {/* Ambient Background */}
            <div className="absolute top-0 left-0 w-full h-full overflow-hidden pointer-events-none">
                <div className="absolute top-[-10%] left-[-10%] w-[40%] h-[40%] bg-blue-600/10 rounded-full blur-[120px]" />
                <div className="absolute bottom-[-10%] right-[-10%] w-[40%] h-[40%] bg-indigo-600/10 rounded-full blur-[120px]" />
            </div>

            {/* Logo Header - Always visible but subtle */}
            <div className="absolute top-8 left-8 flex items-center gap-2 opacity-50">
                <Rocket className="w-5 h-5 text-blue-400" />
                <span className="font-semibold text-lg tracking-tight">ClusterUptime</span>
            </div>

            {/* Progress Bar (Only visible after welcome) */}
            {step > 0 && (
                <div className="absolute top-0 left-0 w-full h-1 bg-slate-800">
                    <motion.div
                        className="h-full bg-blue-500 shadow-[0_0_10px_rgba(59,130,246,0.5)]"
                        initial={{ width: 0 }}
                        animate={{ width: `${(step / 3) * 100}%` }}
                        transition={{ duration: 0.5, ease: "easeInOut" }}
                    />
                </div>
            )}

            <div className="w-full max-w-lg z-10 relative">
                <AnimatePresence mode='wait'>
                    {step === 0 && (
                        <motion.div
                            key="step0"
                            initial={{ opacity: 0, y: 20 }}
                            animate={{ opacity: 1, y: 0 }}
                            exit={{ opacity: 0, y: -20 }}
                            className="text-center space-y-8"
                            data-testid="setup-welcome"
                        >
                            <div className="space-y-4">
                                <h1 className="text-5xl md:text-6xl font-extrabold tracking-tight bg-clip-text text-transparent bg-gradient-to-b from-white to-slate-400 pb-2">
                                    Welcome.
                                </h1>
                                <p className="text-xl text-slate-400 font-light max-w-sm mx-auto leading-relaxed">
                                    Let's get your monitoring instance configured in just a few seconds.
                                </p>
                            </div>

                            <Button
                                onClick={() => setStep(1)}
                                size="lg"
                                className="h-14 px-8 text-lg rounded-full bg-white text-black hover:bg-slate-200 transition-all hover:scale-105 shadow-[0_0_20px_rgba(255,255,255,0.1)] group"
                                data-testid="setup-start-btn"
                            >
                                Get Started <ArrowRight className="ml-2 w-5 h-5 group-hover:translate-x-1 transition-transform" />
                            </Button>
                        </motion.div>
                    )}

                    {step === 1 && (
                        <motion.div
                            key="step1"
                            initial={{ opacity: 0, x: 20 }}
                            animate={{ opacity: 1, x: 0 }}
                            exit={{ opacity: 0, x: -20 }}
                            className="space-y-8"
                        >
                            <div className="space-y-2">
                                <div className="flex items-center gap-2 text-blue-400 mb-2">
                                    <ShieldCheck className="w-5 h-5" />
                                    <span className="text-sm font-medium uppercase tracking-wider">Step 1 of 3</span>
                                </div>
                                <h2 className="text-4xl font-bold">Admin Access</h2>
                                <p className="text-slate-400 text-lg">Create the owner account for this instance.</p>
                            </div>

                            <div className="space-y-5">
                                <div className="space-y-2">
                                    <Label className="text-slate-300 ml-1">Username</Label>
                                    <Input
                                        autoFocus
                                        value={formData.username}
                                        onChange={(e) => setFormData({ ...formData, username: e.target.value })}
                                        className="h-14 text-lg bg-slate-900/50 border-slate-800 focus:border-blue-500/50 focus:ring-blue-500/20 rounded-xl"
                                        placeholder="e.g. admin"
                                        data-testid="setup-username-input"
                                    />
                                </div>
                                <div className="space-y-2">
                                    <Label className="text-slate-300 ml-1">Password</Label>
                                    <Input
                                        type="password"
                                        value={formData.password}
                                        onChange={(e) => setFormData({ ...formData, password: e.target.value })}
                                        className="h-14 text-lg bg-slate-900/50 border-slate-800 focus:border-blue-500/50 focus:ring-blue-500/20 rounded-xl"
                                        placeholder="Min 8 chars, 1 number, 1 special char"
                                        onKeyDown={(e) => e.key === 'Enter' && nextStep()}
                                        data-testid="setup-password-input"
                                    />
                                </div>
                            </div>

                            {error && (
                                <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="flex items-center gap-2 text-red-400 bg-red-950/20 p-3 rounded-lg border border-red-900/30">
                                    <AlertCircle className="w-5 h-5 shrink-0" />
                                    <span>{error}</span>
                                </motion.div>
                            )}

                            <Button onClick={nextStep} className="w-full h-14 text-lg rounded-xl bg-blue-600 hover:bg-blue-500 transition-all shadow-lg shadow-blue-900/20" data-testid="setup-continue-btn">
                                Continue
                            </Button>
                        </motion.div>
                    )}

                    {step === 2 && (
                        <motion.div
                            key="step2"
                            initial={{ opacity: 0, x: 20 }}
                            animate={{ opacity: 1, x: 0 }}
                            exit={{ opacity: 0, x: -20 }}
                            className="space-y-8"
                        >
                            <div className="space-y-2">
                                <div className="flex items-center gap-2 text-indigo-400 mb-2">
                                    <Clock className="w-5 h-5" />
                                    <span className="text-sm font-medium uppercase tracking-wider">Step 2 of 3</span>
                                </div>
                                <h2 className="text-4xl font-bold">Local Time</h2>
                                <p className="text-slate-400 text-lg">Set the default timezone for your dashboards.</p>
                            </div>

                            <div className="space-y-2 p-1">
                                <div className="space-y-2 p-1">
                                    <SelectTimezone
                                        value={formData.timezone}
                                        onValueChange={(val) => setFormData({ ...formData, timezone: val })}
                                        className="h-16 text-xl px-6 bg-slate-900/50 border-slate-800 rounded-xl focus:ring-indigo-500/20 w-full"
                                    />
                                </div>
                            </div>

                            <Button onClick={nextStep} className="w-full h-14 text-lg rounded-xl bg-indigo-600 hover:bg-indigo-500 transition-all shadow-lg shadow-indigo-900/20" data-testid="setup-continue-btn-2">
                                Continue
                            </Button>
                        </motion.div>
                    )}

                    {step === 3 && (
                        <motion.div
                            key="step3"
                            initial={{ opacity: 0, x: 20 }}
                            animate={{ opacity: 1, x: 0 }}
                            exit={{ opacity: 0, x: -20 }}
                            className="space-y-8"
                        >
                            <div className="space-y-2">
                                <div className="flex items-center gap-2 text-emerald-400 mb-2">
                                    <Globe className="w-5 h-5" />
                                    <span className="text-sm font-medium uppercase tracking-wider">Step 3 of 3</span>
                                </div>
                                <h2 className="text-4xl font-bold">Quick Start</h2>
                                <p className="text-slate-400 text-lg">Bootstrap your instance with example data?</p>
                            </div>

                            <div
                                onClick={() => setFormData(f => ({ ...f, createDefaults: !f.createDefaults }))}
                                className={cn(
                                    "cursor-pointer group relative flex items-center gap-6 p-6 rounded-2xl border-2 transition-all duration-300",
                                    formData.createDefaults
                                        ? "bg-emerald-950/10 border-emerald-500/50 shadow-[0_0_30px_rgba(16,185,129,0.1)]"
                                        : "bg-slate-900/30 border-slate-800 hover:border-slate-700"
                                )}
                            >
                                <div className={cn(
                                    "w-8 h-8 rounded-full border-2 flex items-center justify-center transition-colors",
                                    formData.createDefaults
                                        ? "bg-emerald-500 border-emerald-500 text-black"
                                        : "border-slate-600 group-hover:border-slate-500"
                                )}>
                                    {formData.createDefaults && <Check className="w-5 h-5 font-bold" />}
                                </div>

                                <div className="flex-1">
                                    <h3 className={cn("text-xl font-semibold mb-1", formData.createDefaults ? "text-emerald-400" : "text-slate-300")}>
                                        Create Default Monitors
                                    </h3>
                                    <p className="text-slate-400">
                                        We'll add checks for Google, GitHub, and Cloudflare DNS so you aren't staring at a blank screen.
                                    </p>
                                </div>
                            </div>

                            <Button
                                onClick={handleSubmit}
                                disabled={loading}
                                className="w-full h-16 text-xl font-semibold rounded-xl bg-gradient-to-r from-blue-600 to-indigo-600 hover:from-blue-500 hover:to-indigo-500 transition-all shadow-lg shadow-blue-900/20"
                                data-testid="setup-launch-btn"
                            >
                                {loading ? "Configuring..." : "Launch Dashboard"}
                            </Button>
                        </motion.div>
                    )}
                </AnimatePresence>
            </div>

            {/* Footer / Copyright */}
            <div className="absolute bottom-6 text-center text-slate-600 text-sm">
                © {new Date().getFullYear()} ClusterUptime. Self-hosted & Open Source.
            </div>
        </div>
    );
}

// Helper to calculate direction for slides (optional, simplified to fade/slide for now in variants)
