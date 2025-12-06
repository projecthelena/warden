import { useEffect, useState } from "react";
import { useMonitorStore, StatusPage } from "@/lib/store";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { ExternalLink, Globe, Lock } from "lucide-react";
import { useToast } from "@/components/ui/use-toast";

export function StatusPagesView() {
    const { fetchStatusPages, toggleStatusPage, groups } = useMonitorStore();
    const [pages, setPages] = useState<StatusPage[]>([]);
    const { toast } = useToast();

    // Map existing groups to potential status pages + Global 'all'
    // Actually, backend returns the *configured* pages. 
    // If a group doesn't have a configured page yet, we might want to show it as "Disabled" default?
    // Or we rely on backend having seeded them? 
    // Current backend implementation only seeded "all". 
    // We should probably auto-generate the list in UI based on Groups + All, 
    // and matching them with backend config.

    const load = async () => {
        const data = await fetchStatusPages();
        setPages(data);
    };

    useEffect(() => {
        load();
    }, []);

    const handleToggle = async (slug: string, currentStatus: boolean, title: string, groupId?: string | null) => {
        // Optimistic update? Or wait?
        try {
            // We now pass title and groupId so backend can UPSERT if missing.
            await toggleStatusPage(slug, !currentStatus, title, groupId || undefined);
            toast({ title: "Status Page Updated", description: `${title} is now ${!currentStatus ? 'Public' : 'Private'}` });
            load();
        } catch (e) {
            toast({ title: "Error", description: "Failed to update status page", variant: "destructive" });
        }
    };

    // Merge pages config with Groups
    const allPages = [
        {
            slug: 'all',
            title: 'Global Status',
            groupId: null,
            public: pages.find(p => p.slug === 'all')?.public || false
        },
        ...groups.map(g => ({
            slug: g.name.toLowerCase().replace(/\s+/g, '-'), // Naive slug gen
            title: g.name,
            groupId: g.id,
            public: pages.find(p => p.groupId === g.id)?.public || false // Matching by GroupID is safer
        }))
    ];

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-medium">Status Pages</h3>
                <p className="text-sm text-muted-foreground">
                    Manage public status pages for your monitors.
                </p>
            </div>

            <div className="grid gap-4">
                {allPages.map(page => (
                    <Card key={page.slug} className="bg-slate-900/20 border-slate-800">
                        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-4">
                            <div className="space-y-1">
                                <CardTitle className="text-base flex items-center gap-2">
                                    {page.title}
                                    {page.public ? (
                                        <Badge variant="default" className="bg-green-500/10 text-green-500 hover:bg-green-500/20 border-green-500/20">Public</Badge>
                                    ) : (
                                        <Badge variant="secondary" className="text-slate-500">Private</Badge>
                                    )}
                                </CardTitle>
                                <CardDescription>
                                    {page.public
                                        ? (
                                            <a
                                                href={`/status/${page.slug}`}
                                                target="_blank"
                                                rel="noreferrer"
                                                className="ml-auto flex items-center gap-1.5 text-xs text-blue-400 hover:text-blue-300 transition-colors"
                                            >
                                                Open Public Page <ExternalLink className="w-3 h-3" />
                                            </a>
                                        )
                                        : "Only visible to administrators"}
                                </CardDescription>
                            </div>
                            <div className="flex items-center gap-4">
                                {page.public && (
                                    <Button variant="ghost" size="sm" className="gap-2" asChild>
                                        <a href={`/status/${page.slug}`} target="_blank" rel="noreferrer">
                                            <ExternalLink className="w-4 h-4" />
                                            View
                                        </a>
                                    </Button>
                                )}
                                <Switch
                                    checked={page.public}
                                    onCheckedChange={() => handleToggle(page.slug, page.public, page.title, page.groupId)}
                                />
                            </div>
                        </CardHeader>
                    </Card>
                ))}
            </div>
        </div>
    )
}
