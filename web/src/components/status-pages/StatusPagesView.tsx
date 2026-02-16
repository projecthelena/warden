import { useState } from "react";
import { useStatusPagesQuery, useToggleStatusPageMutation } from "@/hooks/useStatusPages";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Switch } from "@/components/ui/switch";
import { ExternalLink, Settings } from "lucide-react";
import { useToast } from "@/components/ui/use-toast";
import { StatusPage } from "@/lib/store";
import { StatusPageConfigDialog } from "./StatusPageConfigDialog";

export function StatusPagesView() {
    const { data: pages = [], isLoading } = useStatusPagesQuery();
    const toggleMutation = useToggleStatusPageMutation();
    const { toast } = useToast();
    const [configDialogOpen, setConfigDialogOpen] = useState(false);
    const [selectedPage, setSelectedPage] = useState<StatusPage | null>(null);

    const openConfig = (page: StatusPage) => {
        setSelectedPage(page);
        setConfigDialogOpen(true);
    };

    const resolveSlug = (slug: string, title: string) => {
        if (slug.startsWith('g-')) {
            return title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || slug;
        }
        return slug;
    };

    const handleToggleEnabled = async (page: typeof pages[0]) => {
        try {
            const newEnabled = !page.enabled;
            const targetSlug = resolveSlug(page.slug, page.title);
            await toggleMutation.mutateAsync({
                slug: targetSlug,
                public: page.public,
                enabled: newEnabled,
                title: page.title,
                groupId: page.groupId || undefined,
            });
            toast({
                title: "Status Page Updated",
                description: `${page.title} is now ${newEnabled ? 'enabled' : 'disabled'}`,
            });
        } catch (_e) {
            toast({ title: "Error", description: "Failed to update status page", variant: "destructive" });
        }
    };

    const handleTogglePublic = async (page: typeof pages[0]) => {
        try {
            const newPublic = !page.public;
            const targetSlug = resolveSlug(page.slug, page.title);
            await toggleMutation.mutateAsync({
                slug: targetSlug,
                public: newPublic,
                enabled: page.enabled,
                title: page.title,
                groupId: page.groupId || undefined,
            });
            toast({
                title: "Status Page Updated",
                description: `${page.title} is now ${newPublic ? 'public' : 'private'}`,
            });
        } catch (_e) {
            toast({ title: "Error", description: "Failed to update status page", variant: "destructive" });
        }
    };

    const allPages = Array.isArray(pages) ? pages : [];

    if (isLoading) return <div>Loading...</div>;

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
                        data-testid={`status-page-row-${page.slug}`}
                        className="flex items-center justify-between p-4 rounded-xl border border-border bg-card hover:bg-accent/50 transition-all duration-200"
                    >
                        <div className="space-y-1">
                            <div className="flex items-center gap-3">
                                <span className="text-base font-medium text-foreground">{page.title}</span>
                                {!page.enabled ? (
                                    <Badge data-testid={`status-page-badge-${page.slug}`} variant="secondary" className="shadow-none font-normal text-xs px-2 py-0.5 h-auto">
                                        Disabled
                                    </Badge>
                                ) : page.public ? (
                                    <Badge data-testid={`status-page-badge-${page.slug}`} variant="default" className="shadow-none font-normal text-xs px-2 py-0.5 h-auto">
                                        Public
                                    </Badge>
                                ) : (
                                    <Badge data-testid={`status-page-badge-${page.slug}`} variant="outline" className="shadow-none font-normal text-xs px-2 py-0.5 h-auto">
                                        Private
                                    </Badge>
                                )}
                            </div>
                            <div className="text-sm text-muted-foreground flex items-center gap-2">
                                <span className="font-mono text-xs opacity-50">/{page.slug}</span>
                                {page.enabled && (
                                    <a
                                        href={`/status/${page.slug}`}
                                        target="_blank"
                                        rel="noreferrer"
                                        data-testid={`status-page-visit-${page.slug}`}
                                        className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
                                    >
                                        Visit Page <ExternalLink className="w-3 h-3" />
                                    </a>
                                )}
                            </div>
                        </div>

                        <div className="flex items-center gap-4">
                            <Button
                                variant="ghost"
                                size="icon"
                                className="h-8 w-8"
                                onClick={() => openConfig(page)}
                                data-testid={`status-page-config-${page.slug}`}
                            >
                                <Settings className="h-4 w-4" />
                            </Button>
                            <div className="flex items-center gap-2">
                                <span className={`text-xs transition-colors ${page.enabled ? 'text-foreground' : 'text-muted-foreground'}`}>
                                    {page.enabled ? 'Enabled' : 'Disabled'}
                                </span>
                                <Switch
                                    data-testid={`status-page-enabled-toggle-${page.slug}`}
                                    checked={page.enabled}
                                    onCheckedChange={() => handleToggleEnabled(page)}
                                />
                            </div>
                            <div className="flex items-center gap-2">
                                <span className={`text-xs transition-colors ${page.enabled && page.public ? 'text-foreground' : 'text-muted-foreground'}`}>
                                    {page.public ? 'Public' : 'Private'}
                                </span>
                                <Switch
                                    data-testid={`status-page-public-toggle-${page.slug}`}
                                    checked={page.public}
                                    disabled={!page.enabled}
                                    onCheckedChange={() => handleTogglePublic(page)}
                                />
                            </div>
                        </div>
                    </div>
                ))}
            </div>

            <StatusPageConfigDialog
                page={selectedPage}
                open={configDialogOpen}
                onOpenChange={setConfigDialogOpen}
            />
        </div>
    )
}
