import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { useMonitorStore } from "@/lib/store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Activity, Lock, AlertCircle } from "lucide-react";

export function LoginPage() {
    const navigate = useNavigate();
    const { login } = useMonitorStore();
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);

    const handleLogin = async (e: React.FormEvent) => {
        e.preventDefault();
        setIsLoading(true);
        setError(null);

        const result = await login(username, password);
        setIsLoading(false);

        if (result.success) {
            navigate('/dashboard');
        } else {
            setError(result.error || "An unexpected error occurred");
        }
    };

    return (
        <div className="min-h-screen bg-background flex items-center justify-center p-4">
            <div className="w-full max-w-sm space-y-8">
                <div className="flex flex-col items-center justify-center text-center space-y-2">
                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-lg">
                        <Activity className="h-6 w-6" />
                    </div>
                    <div className="space-y-1">
                        <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                            Welcome back
                        </h1>
                        <p className="text-sm text-muted-foreground">
                            Enter your credentials to access your dashboard
                        </p>
                    </div>
                </div>

                <Card>
                    <form onSubmit={handleLogin}>
                        <CardHeader className="space-y-1">
                            <CardTitle className="text-xl" data-testid="login-header">Sign in</CardTitle>
                            <CardDescription>
                                Enter your credentials to continue
                            </CardDescription>
                        </CardHeader>
                        <CardContent className="grid gap-4">
                            {error && (
                                <Alert variant="destructive" className="bg-destructive/50 text-destructive-foreground border-destructive/50">
                                    <AlertCircle className="h-4 w-4 text-destructive-foreground" />
                                    <AlertTitle>Error</AlertTitle>
                                    <AlertDescription>
                                        {error}
                                    </AlertDescription>
                                </Alert>
                            )}
                            <div className="grid gap-2">
                                <Label htmlFor="username">Username</Label>
                                <Input
                                    id="username"
                                    type="text"
                                    placeholder="username"
                                    value={username}
                                    onChange={e => setUsername(e.target.value)}
                                    required
                                    data-testid="login-username-input"
                                />
                            </div>
                            <div className="grid gap-2">
                                <Label htmlFor="password">Password</Label>
                                <Input
                                    id="password"
                                    type="password"
                                    value={password}
                                    onChange={e => setPassword(e.target.value)}
                                    required
                                    data-testid="login-password-input"
                                />
                            </div>
                        </CardContent>
                        <CardFooter>
                            <Button className="w-full" type="submit" disabled={isLoading} data-testid="login-submit-btn">
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


            </div>
        </div>
    );
}
