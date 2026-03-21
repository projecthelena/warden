import { useState, useEffect } from "react";
import { useNavigate } from "react-router-dom";
import { useMonitorStore } from "@/lib/store";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { ExternalLink, LogOut } from "lucide-react";

interface MyPage {
    id: number;
    slug: string;
    title: string;
    public: boolean;
}

export function MyPagesView() {
    const navigate = useNavigate();
    const { user, logout, isAuthChecked } = useMonitorStore();
    const [pages, setPages] = useState<MyPage[]>([]);
    const [loading, setLoading] = useState(true);

    useEffect(() => {
        if (isAuthChecked && (!user || !user.isAuthenticated)) {
            navigate("/login", { replace: true });
            return;
        }
        if (isAuthChecked && user && user.role !== "status_viewer") {
            navigate("/dashboard", { replace: true });
            return;
        }
    }, [user, isAuthChecked, navigate]);

    useEffect(() => {
        async function fetchPages() {
            try {
                const res = await fetch("/api/my/status-pages", { credentials: "include" });
                if (res.ok) {
                    const data = await res.json();
                    setPages(data.pages || []);
                }
            } catch (e) {
                console.error("Failed to fetch pages:", e);
            } finally {
                setLoading(false);
            }
        }
        if (isAuthChecked && user?.isAuthenticated) {
            fetchPages();
        }
    }, [isAuthChecked, user]);

    const handleLogout = async () => {
        await logout();
        navigate("/login", { replace: true });
    };

    if (!isAuthChecked || loading) {
        return (
            <div className="min-h-screen bg-background flex items-center justify-center">
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
            </div>
        );
    }

    return (
        <div className="min-h-screen bg-background">
            <header className="border-b border-border/40 bg-background/95 backdrop-blur sticky top-0 z-10">
                <div className="max-w-3xl mx-auto flex items-center justify-between px-4 h-14">
                    <div className="flex items-center gap-2">
                        <span className="text-lg font-display font-bold tracking-tight">
                            Project <span className="font-normal text-muted-foreground">Helena</span>
                        </span>
                        <span className="text-xs font-mono font-medium text-cyan-500 tracking-widest">WARDEN</span>
                    </div>
                    <Button variant="ghost" size="sm" onClick={handleLogout} className="gap-2">
                        <LogOut className="h-4 w-4" />
                        Sign Out
                    </Button>
                </div>
            </header>

            <main className="max-w-3xl mx-auto px-4 py-8">
                <div className="mb-6">
                    <h1 className="text-xl font-semibold tracking-tight text-foreground">My Status Pages</h1>
                    <p className="text-sm text-muted-foreground">
                        {user?.name ? `Welcome, ${user.name}` : "View your assigned status pages"}
                    </p>
                </div>

                {pages.length === 0 ? (
                    <div className="flex flex-col items-center justify-center p-12 border border-dashed border-border rounded-lg text-muted-foreground">
                        <h3 className="text-lg font-medium text-foreground mb-1">No Status Pages Assigned</h3>
                        <p className="text-sm">Contact your administrator to get access to status pages.</p>
                    </div>
                ) : (
                    <div className="grid gap-3">
                        {pages.map((page) => (
                            <Card
                                key={page.id}
                                onClick={() => navigate(`/status/${page.slug}`)}
                                className="group relative flex flex-row items-center justify-between p-4 rounded-xl border-border/50 bg-card/50 hover:bg-accent/50 transition-all duration-300 cursor-pointer overflow-hidden gap-4 shadow-none"
                            >
                                <div className="flex items-center gap-3">
                                    <div className="font-medium text-foreground group-hover:text-foreground transition-colors">
                                        {page.title}
                                    </div>
                                </div>
                                <div className="flex items-center gap-2 text-muted-foreground group-hover:text-foreground transition-colors">
                                    <span className="text-sm">View</span>
                                    <ExternalLink className="h-4 w-4" />
                                </div>
                            </Card>
                        ))}
                    </div>
                )}
            </main>
        </div>
    );
}
