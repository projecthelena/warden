import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
} from "@/components/ui/sheet";
import { Separator } from "@/components/ui/separator";
import { Info } from "lucide-react";
import { useToggleStatusPageMutation } from "@/hooks/useStatusPages";
import { useToast } from "@/components/ui/use-toast";
import { StatusPage } from "@/lib/store";
import { sanitizeImageUrl } from "@/lib/utils";
import { Loader2, Image, X } from "lucide-react";

interface StatusPageConfigSheetProps {
    page: StatusPage | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function StatusPageConfigSheet({ page, open, onOpenChange }: StatusPageConfigSheetProps) {
    const { toast } = useToast();
    const toggleMutation = useToggleStatusPageMutation();

    // Form state
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [logoUrl, setLogoUrl] = useState("");
    const [faviconUrl, setFaviconUrl] = useState("");
    const [accentColor] = useState("");
    const [theme, setTheme] = useState<'light' | 'dark' | 'system'>("system");
    const [showUptimeBars, setShowUptimeBars] = useState(true);
    const [showUptimePercentage, setShowUptimePercentage] = useState(true);
    const [showIncidentHistory, setShowIncidentHistory] = useState(true);
    const [uptimeDaysRange, setUptimeDaysRange] = useState(90);
    const [headerContent, setHeaderContent] = useState<'logo-title' | 'logo-only' | 'title-only'>("logo-title");
    const [headerAlignment, setHeaderAlignment] = useState<'left' | 'center' | 'right'>("center");
    const [headerArrangement, setHeaderArrangement] = useState<'stacked' | 'inline'>("inline");

    // Preview state
    const [logoError, setLogoError] = useState(false);
    const [faviconError, setFaviconError] = useState(false);

    // Reset form when page changes
    useEffect(() => {
        if (page) {
            setTitle(page.title || "");
            setDescription(page.description || "");
            setLogoUrl(page.logoUrl || "");
            setFaviconUrl(page.faviconUrl || "");
            setTheme(page.theme || "system");
            setShowUptimeBars(page.showUptimeBars ?? true);
            setShowUptimePercentage(page.showUptimePercentage ?? true);
            setShowIncidentHistory(page.showIncidentHistory ?? true);
            setUptimeDaysRange(page.uptimeDaysRange ?? 90);
            setHeaderContent(page.headerContent || "logo-title");
            setHeaderAlignment(page.headerAlignment || "center");
            setHeaderArrangement(page.headerArrangement || "inline");
            setLogoError(false);
            setFaviconError(false);
        }
    }, [page]);

    const resolveSlug = (slug: string, title: string) => {
        if (slug.startsWith('g-')) {
            return title.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '') || slug;
        }
        return slug;
    };

    const handleSave = async () => {
        if (!page) return;

        try {
            const targetSlug = resolveSlug(page.slug, title);
            await toggleMutation.mutateAsync({
                slug: targetSlug,
                public: page.public,
                enabled: page.enabled,
                title,
                groupId: page.groupId || undefined,
                description,
                logoUrl,
                faviconUrl,
                accentColor,
                theme,
                showUptimeBars,
                showUptimePercentage,
                showIncidentHistory,
                uptimeDaysRange,
                headerContent,
                headerAlignment,
                headerArrangement,
            });
            toast({
                title: "Configuration Saved",
                description: `${title} settings updated successfully.`,
            });
            onOpenChange(false);
        } catch (_e) {
            toast({
                title: "Error",
                description: "Failed to save configuration",
                variant: "destructive",
            });
        }
    };

    const isValidImageUrl = (url: string) => {
        if (!url) return true;
        return url.startsWith("http://") || url.startsWith("https://") || url.startsWith("data:image/");
    };

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="sm:max-w-[500px] overflow-y-auto">
                <SheetHeader>
                    <SheetTitle>Configure Status Page</SheetTitle>
                    <SheetDescription>Customize branding, layout, and display options for your status page.</SheetDescription>
                </SheetHeader>

                <div className="space-y-6 py-4">
                    {/* General Section */}
                    <div className="space-y-4">
                        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
                            General
                        </h3>

                        <div className="space-y-2">
                            <Label htmlFor="title">Title</Label>
                            <Input
                                id="title"
                                placeholder="My Status Page"
                                value={title}
                                onChange={(e) => setTitle(e.target.value)}
                            />
                            <p className="text-xs text-muted-foreground">The name displayed at the top of your status page</p>
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="description">Description</Label>
                            <Textarea
                                id="description"
                                placeholder="A short tagline or description for your status page"
                                value={description}
                                onChange={(e) => setDescription(e.target.value)}
                                className="resize-none"
                                rows={2}
                            />
                        </div>
                    </div>

                    <Separator />

                    {/* Branding Section */}
                    <div className="space-y-4">
                        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
                            Branding
                        </h3>

                        {/* Logo */}
                        <div className="space-y-2">
                            <Label htmlFor="logoUrl">Logo</Label>
                            <div className="flex gap-2">
                                <Input
                                    id="logoUrl"
                                    placeholder="https://example.com/logo.png"
                                    value={logoUrl}
                                    onChange={(e) => {
                                        setLogoUrl(e.target.value);
                                        setLogoError(false);
                                    }}
                                    className={!isValidImageUrl(logoUrl) ? "border-destructive" : ""}
                                />
                                {logoUrl && (
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="icon"
                                        title="Remove logo"
                                        onClick={() => { setLogoUrl(""); setLogoError(false); }}
                                    >
                                        <X className="w-4 h-4" />
                                    </Button>
                                )}
                            </div>
                            {!isValidImageUrl(logoUrl) && (
                                <p className="text-xs text-destructive">Must be an http/https URL</p>
                            )}
                            {logoUrl && isValidImageUrl(logoUrl) && (
                                <div className="flex items-center gap-2 mt-2 p-2 bg-muted/50 rounded-md">
                                    {logoError ? (
                                        <div className="flex items-center justify-center w-8 h-8 bg-muted rounded border border-border">
                                            <Image className="w-4 h-4 text-muted-foreground" />
                                        </div>
                                    ) : (
                                        <img
                                            src={sanitizeImageUrl(logoUrl)}
                                            alt="Logo preview"
                                            className="w-8 h-8 object-contain rounded"
                                            onError={() => setLogoError(true)}
                                        />
                                    )}
                                    <span className="text-xs text-muted-foreground">Logo preview</span>
                                </div>
                            )}
                        </div>

                        {/* Favicon */}
                        <div className="space-y-2">
                            <Label htmlFor="faviconUrl">Favicon</Label>
                            <div className="flex gap-2">
                                <Input
                                    id="faviconUrl"
                                    placeholder="https://example.com/favicon.ico"
                                    value={faviconUrl}
                                    onChange={(e) => {
                                        setFaviconUrl(e.target.value);
                                        setFaviconError(false);
                                    }}
                                    className={!isValidImageUrl(faviconUrl) ? "border-destructive" : ""}
                                />
                                {faviconUrl && (
                                    <Button
                                        type="button"
                                        variant="outline"
                                        size="icon"
                                        title="Remove favicon"
                                        onClick={() => { setFaviconUrl(""); setFaviconError(false); }}
                                    >
                                        <X className="w-4 h-4" />
                                    </Button>
                                )}
                            </div>
                            {!isValidImageUrl(faviconUrl) && (
                                <p className="text-xs text-destructive">Must be an http/https URL</p>
                            )}
                            {faviconUrl && isValidImageUrl(faviconUrl) && (
                                <div className="flex items-center gap-2 mt-2 p-2 bg-muted/50 rounded-md">
                                    {faviconError ? (
                                        <div className="flex items-center justify-center w-4 h-4 bg-muted rounded border border-border">
                                            <Image className="w-3 h-3 text-muted-foreground" />
                                        </div>
                                    ) : (
                                        <img
                                            src={sanitizeImageUrl(faviconUrl)}
                                            alt="Favicon preview"
                                            className="w-4 h-4 object-contain"
                                            onError={() => setFaviconError(true)}
                                        />
                                    )}
                                    <span className="text-xs text-muted-foreground">Favicon preview (browser tab icon)</span>
                                </div>
                            )}
                        </div>

                        <div className="space-y-2">
                            <Label htmlFor="theme">Theme</Label>
                            <Select value={theme} onValueChange={(v) => setTheme(v as 'light' | 'dark' | 'system')}>
                                <SelectTrigger id="theme">
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value="system">System</SelectItem>
                                    <SelectItem value="light">Light</SelectItem>
                                    <SelectItem value="dark">Dark</SelectItem>
                                </SelectContent>
                            </Select>
                        </div>

                        {/* Header Layout */}
                        <div className="space-y-3">
                            <Label>Header Layout</Label>

                            <div className="space-y-1.5">
                                <span className="text-xs text-muted-foreground">Content</span>
                                <div className="flex gap-1">
                                    {([['logo-title', 'Logo & Title'], ['logo-only', 'Logo Only'], ['title-only', 'Title Only']] as const).map(([value, label]) => (
                                        <Button
                                            key={value}
                                            type="button"
                                            size="sm"
                                            variant={headerContent === value ? "default" : "outline"}
                                            className="flex-1 text-xs"
                                            onClick={() => setHeaderContent(value)}
                                        >
                                            {label}
                                        </Button>
                                    ))}
                                </div>
                            </div>

                            {headerContent !== 'title-only' && !logoUrl && (
                                <p className="text-xs text-muted-foreground flex items-center gap-1">
                                    <Info className="w-3 h-3 shrink-0" />
                                    Set a logo URL above for the logo to appear
                                </p>
                            )}

                            <div className="space-y-1.5">
                                <span className="text-xs text-muted-foreground">Alignment</span>
                                <div className="flex gap-1">
                                    {([['left', 'Left'], ['center', 'Center'], ['right', 'Right']] as const).map(([value, label]) => (
                                        <Button
                                            key={value}
                                            type="button"
                                            size="sm"
                                            variant={headerAlignment === value ? "default" : "outline"}
                                            className="flex-1 text-xs"
                                            onClick={() => setHeaderAlignment(value)}
                                        >
                                            {label}
                                        </Button>
                                    ))}
                                </div>
                            </div>

                            {headerContent === 'logo-title' && (
                                <div className="space-y-1.5">
                                    <span className="text-xs text-muted-foreground">Arrangement</span>
                                    <div className="flex gap-1">
                                        {([['stacked', 'Stacked'], ['inline', 'Inline']] as const).map(([value, label]) => (
                                            <Button
                                                key={value}
                                                type="button"
                                                size="sm"
                                                variant={headerArrangement === value ? "default" : "outline"}
                                                className="flex-1 text-xs"
                                                onClick={() => setHeaderArrangement(value)}
                                            >
                                                {label}
                                            </Button>
                                        ))}
                                    </div>
                                </div>
                            )}
                        </div>
                    </div>

                    <Separator />

                    {/* Display Options Section */}
                    <div className="space-y-4">
                        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
                            Display
                        </h3>

                        <div className="space-y-3">
                            <div className="flex items-center justify-between gap-4">
                                <div className="min-w-0">
                                    <Label htmlFor="showUptimeBars" className="cursor-pointer">Show Uptime Bars</Label>
                                    <p className="text-xs text-muted-foreground">Display uptime history bars</p>
                                </div>
                                <Switch
                                    id="showUptimeBars"
                                    checked={showUptimeBars}
                                    onCheckedChange={setShowUptimeBars}
                                    className="shrink-0"
                                />
                            </div>

                            {showUptimeBars && (
                                <div className="ml-4 pl-3 border-l-2 border-border space-y-2">
                                    <Label htmlFor="uptimeDaysRange">Uptime Range</Label>
                                    <Select
                                        value={String(uptimeDaysRange)}
                                        onValueChange={(v) => setUptimeDaysRange(Number(v))}
                                    >
                                        <SelectTrigger id="uptimeDaysRange">
                                            <SelectValue />
                                        </SelectTrigger>
                                        <SelectContent>
                                            <SelectItem value="7">7 days</SelectItem>
                                            <SelectItem value="30">30 days</SelectItem>
                                            <SelectItem value="60">60 days</SelectItem>
                                            <SelectItem value="90">90 days</SelectItem>
                                            <SelectItem value="180">180 days</SelectItem>
                                            <SelectItem value="365">365 days</SelectItem>
                                        </SelectContent>
                                    </Select>
                                    <p className="text-xs text-muted-foreground">How many days of uptime history to display</p>
                                </div>
                            )}

                            <div className="flex items-center justify-between gap-4">
                                <div className="min-w-0">
                                    <Label htmlFor="showUptimePercentage" className="cursor-pointer">Show Uptime Percentage</Label>
                                    <p className="text-xs text-muted-foreground">Display overall uptime percentage</p>
                                </div>
                                <Switch
                                    id="showUptimePercentage"
                                    checked={showUptimePercentage}
                                    onCheckedChange={setShowUptimePercentage}
                                    className="shrink-0"
                                />
                            </div>

                            <div className="flex items-center justify-between gap-4">
                                <div className="min-w-0">
                                    <Label htmlFor="showIncidentHistory" className="cursor-pointer">Show Incident History</Label>
                                    <p className="text-xs text-muted-foreground">Display past incidents section</p>
                                </div>
                                <Switch
                                    id="showIncidentHistory"
                                    checked={showIncidentHistory}
                                    onCheckedChange={setShowIncidentHistory}
                                    className="shrink-0"
                                />
                            </div>
                        </div>
                    </div>
                </div>

                <SheetFooter className="flex-col sm:flex-row gap-2">
                    <Button variant="outline" onClick={() => onOpenChange(false)} className="w-full sm:w-auto">
                        Cancel
                    </Button>
                    <Button
                        onClick={handleSave}
                        disabled={toggleMutation.isPending || !title.trim() || !isValidImageUrl(logoUrl) || !isValidImageUrl(faviconUrl)}
                        className="w-full sm:w-auto"
                    >
                        {toggleMutation.isPending && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
                        Save Changes
                    </Button>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    );
}
