import { useState, useEffect } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { useMonitorStore } from "@/lib/store";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Card, CardContent, CardDescription, CardHeader, CardTitle, CardFooter } from "@/components/ui/card";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Lock, AlertCircle } from "lucide-react";

function GoogleIcon({ className }: { className?: string }) {
    return (
        <svg className={className} viewBox="0 0 24 24" fill="currentColor">
            <path d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z" fill="#4285F4"/>
            <path d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z" fill="#34A853"/>
            <path d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l2.85-2.22.81-.62z" fill="#FBBC05"/>
            <path d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z" fill="#EA4335"/>
        </svg>
    );
}

const SSO_ERROR_MESSAGES: Record<string, string> = {
    sso_not_configured: "Google SSO is not configured. Please contact your administrator.",
    invalid_state: "Invalid session state. Please try again.",
    oauth_denied: "Google sign-in was cancelled or denied.",
    missing_code: "Authorization code missing. Please try again.",
    token_exchange_failed: "Failed to complete sign-in. Please try again.",
    userinfo_failed: "Failed to retrieve user information from Google.",
    userinfo_read_failed: "Failed to read user information.",
    userinfo_parse_failed: "Failed to process user information.",
    invalid_user_data: "Invalid user data received from Google.",
    email_not_verified: "Your Google email address is not verified. Please verify your email and try again.",
    domain_not_allowed: "Your email domain is not allowed. Please contact your administrator.",
    user_not_found: "No account found for this email. Please contact your administrator.",
    user_creation_failed: "Failed to create account. Please try again.",
    session_error: "Failed to create session. Please try again.",
    internal_error: "An internal error occurred. Please try again.",
};

export function LoginPage() {
    const navigate = useNavigate();
    const [searchParams] = useSearchParams();
    const { login } = useMonitorStore();
    const [username, setUsername] = useState("");
    const [password, setPassword] = useState("");
    const [error, setError] = useState<string | null>(null);
    const [isLoading, setIsLoading] = useState(false);
    const [googleSSOEnabled, setGoogleSSOEnabled] = useState(false);

    // Check for SSO errors in URL params
    useEffect(() => {
        const errorParam = searchParams.get("error");
        if (errorParam) {
            const errorMessage = SSO_ERROR_MESSAGES[errorParam] || "An error occurred during sign-in.";
            setError(errorMessage);
        }
    }, [searchParams]);

    // Check if Google SSO is enabled
    useEffect(() => {
        fetch("/api/auth/sso/status")
            .then(res => res.json())
            .then(data => {
                setGoogleSSOEnabled(data.google === true);
            })
            .catch(() => {
                setGoogleSSOEnabled(false);
            });
    }, []);

    const handleGoogleLogin = () => {
        window.location.href = "/api/auth/sso/google";
    };

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
                    <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-cyan-500 shadow-lg">
                        <span className="text-lg font-bold text-[#09090b]">H</span>
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
                                    onChange={e => {
                                        setUsername(e.target.value);
                                        if (/[A-Z]/.test(e.target.value)) {
                                            setError("Username must be lowercase.");
                                        } else {
                                            setError(null);
                                        }
                                    }}
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
                        <CardFooter className="flex flex-col gap-4">
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

                            {googleSSOEnabled && (
                                <>
                                    <div className="relative w-full">
                                        <div className="absolute inset-0 flex items-center">
                                            <span className="w-full border-t" />
                                        </div>
                                        <div className="relative flex justify-center text-xs uppercase">
                                            <span className="bg-card px-2 text-muted-foreground">
                                                Or continue with
                                            </span>
                                        </div>
                                    </div>

                                    <Button
                                        type="button"
                                        variant="outline"
                                        className="w-full"
                                        onClick={handleGoogleLogin}
                                        data-testid="google-sso-btn"
                                    >
                                        <GoogleIcon className="mr-2 h-4 w-4" />
                                        Sign in with Google
                                    </Button>
                                </>
                            )}
                        </CardFooter>
                    </form>
                </Card>


            </div>
        </div>
    );
}
