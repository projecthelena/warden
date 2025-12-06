import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useMonitorStore } from "@/lib/store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { Activity, Lock } from "lucide-react";

export function LoginPage() {
    const navigate = useNavigate();
    const { login } = useMonitorStore();
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [isLoading, setIsLoading] = useState(false);

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);

        const success = await login(username, password);
        setIsLoading(false);

        if (success) {
            navigate('/dashboard');
        } else {
            // Show error (would be better with a toast, but this is fine for now)
            alert("Login failed. Default is admin / password");
        }
    };

    return (
        <div className="min-h-screen bg-[#020617] flex items-center justify-center p-4">
            <div className="w-full max-w-sm space-y-8">
                <div className="flex flex-col items-center justify-center text-center space-y-2">
                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-blue-600 text-white shadow-lg shadow-blue-900/20">
                        <Activity className="h-6 w-6" />
                    </div>
                    <div className="space-y-1">
                        <h1 className="text-2xl font-semibold tracking-tight text-white">
                            Welcome back
                        </h1>
                        <p className="text-sm text-slate-400">
                            Enter your credentials to access your dashboard
                        </p>
                    </div>
                </div>

                <Card className="bg-slate-900/50 border-slate-800">
                    <form onSubmit={handleLogin}>
                        <CardHeader className="space-y-1">
                            <CardTitle className="text-xl">Sign in</CardTitle>
                            <CardDescription>
                                Enter your credentials to continue
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="grid gap-4">
                            <div className="grid gap-2">
                                <Label htmlFor="username">Username</Label>
                                <Input
                                    id="username"
                                    type="text"
                                    placeholder="admin"
                                    className="bg-slate-950 border-slate-800"
                                    value={username}
                                    onChange={e => setUsername(e.target.value)}
                                    required
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="password">Password</Label>
                                <Input
                                    id="password"
                                    type="password"
                                    className="bg-slate-950 border-slate-800"
                                    value={password}
                                    onChange={e => setPassword(e.target.value)}
                                    required
                                />
                            </div>
                        </CardContent>
                        <CardFooter>
                            <Button className="w-full" type="submit" disabled={isLoading}>
                                {isLoading ? (
                                    <div className="flex items-center gap-2">
                                        <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
                                        Signing in...
                                    </div>
                                ) : (
                                    <div className="flex items-center gap-2">
                                        <Lock className="w-4 h-4" /> Sign In
                                    </div>
                                )}
                            </Button>
                        </CardFooter>
                    </form>
                </Card>

                <p className="px-8 text-center text-sm text-slate-500">
                    <a href="/status" className="hover:text-slate-400 underline underline-offset-4">
                        View Public Status Page
                    </a>
                </p>
            </div>
        </div>
    );
}
