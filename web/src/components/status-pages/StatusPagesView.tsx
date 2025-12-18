import { useEffect, useState, useCallback } from "react";
import { useMonitorStore, StatusPage } from "@/lib/store";
import { Badge } from "@/components/ui/badge";
import { Switch } from "@/components/ui/switch";
import { ExternalLink } from "lucide-react";
import { useToast } from "@/components/ui/use-toast";

export function StatusPagesView() {
    const { fetchStatusPages, toggleStatusPage } = useMonitorStore();
    const [pages, setPages] = useState<StatusPage[]>([]);
    const { toast } = useToast();

    // Map existing groups to potential status pages + Global 'all'
    // Actually, backend returns the *configured* pages. 
    // If a group doesn't have a configured page yet, we might want to show it as "Disabled" default?
    // Or we rely on backend having seeded them? 
    // Current backend implementation only seeded "all". 
    const load = useCallback(async () => {
        const data = await fetchStatusPages();
        setPages(data);
    }, [fetchStatusPages]);

    useEffect(() => {
        load();
    }, [load]);

    const handleToggle = async (slug: string, currentStatus: boolean, title: string, groupId?: string | null) => {
        try {
            // If slug starts with 'g-' and looks like an ID default, we might want to generate a prettier slug
            // But for now, backend handles upsert.

            // Generate a prettier slug if it's currently a raw ID-based default and we are enabling it
            let targetSlug = slug;
            if (!currentStatus && (slug.startsWith('g-') || slug === 'all')) {
                if (slug !== 'all') {
                    // Simple clean slug from title
                    targetSlug = title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || slug;
                }
            }

            await toggleStatusPage(targetSlug, !currentStatus, title, groupId || undefined);
            toast({ title: "Status Page Updated", description: `${title} is now ${!currentStatus ? 'Public' : 'Private'}` });
            load();
        } catch (e) {
            toast({ title: "Error", description: "Failed to update status page", variant: "destructive" });
        }
    };

    const allPages = pages;

    return (
        <div className="space-y-6">
            <div>
                <h2 className="text-xl font-semibold tracking-tight text-foreground">Status Pages</h2>
                <p className="text-sm text-muted-foreground">
                    Enable status pages to share uptime history with your users.
                </p>
            </div>

            <div className="grid gap-3">
                {allPages.map(page => (
                    <div
                        key={page.slug}
                        className="flex items-center justify-between p-4 rounded-xl border border-border bg-card hover:bg-accent/50 transition-all duration-200"
                    >
                        <div className="space-y-1">
                            <div className="flex items-center gap-3">
                                <span className="text-base font-medium text-foreground">{page.title}</span>
                                {page.public ? (
                                    <Badge variant="default" className="shadow-none font-normal text-xs px-2 py-0.5 h-auto">
                                        Active
                                    </Badge>
                                ) : (
                                    <Badge variant="secondary" className="shadow-none font-normal text-xs px-2 py-0.5 h-auto">
                                        Disabled
                                    </Badge>
                                )}
                            </div>
                            <div className="text-sm text-muted-foreground flex items-center gap-2">
                                <span className="font-mono text-xs opacity-50">/{page.slug}</span>
                                {page.public && (
                                    <a
                                        href={`/status/${page.slug}`}
                                        target="_blank"
                                        rel="noreferrer"
                                        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                                    >
                                        Visit Page <ExternalLink className="w-3 h-3" />
                                    </a>
                                )}
                            </div>
                        </div>

                        <div className="flex items-center gap-6">
                            <div className="flex items-center gap-2">
                                <span className={`text-sm font-medium transition-colors ${page.public ? 'text-foreground' : 'text-muted-foreground'}`}>
                                    {page.public ? 'On' : 'Off'}
                                </span>
                                <Switch
                                    checked={page.public}
                                    onCheckedChange={() => handleToggle(page.slug, page.public, page.title, page.groupId)}
                                />
                            </div>
                        </div>
                    </div>
                ))}
            </div>
        </div>
    )
}
