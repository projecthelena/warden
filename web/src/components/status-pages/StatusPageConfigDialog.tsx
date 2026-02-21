import { useState, useEffect } from "react";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { useToggleStatusPageMutation } from "@/hooks/useStatusPages";
import { useToast } from "@/components/ui/use-toast";
import { StatusPage } from "@/lib/store";
import { Loader2, Image } from "lucide-react";

interface StatusPageConfigDialogProps {
    page: StatusPage | null;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function StatusPageConfigDialog({ page, open, onOpenChange }: StatusPageConfigDialogProps) {
    const { toast } = useToast();
    const toggleMutation = useToggleStatusPageMutation();

    // Form state
    const [description, setDescription] = useState("");
    const [logoUrl, setLogoUrl] = useState("");
    const [accentColor, setAccentColor] = useState("");
    const [theme, setTheme] = useState<'light' | 'dark' | 'system'>("system");
    const [showUptimeBars, setShowUptimeBars] = useState(true);
    const [showUptimePercentage, setShowUptimePercentage] = useState(true);
    const [showIncidentHistory, setShowIncidentHistory] = useState(true);

    // Logo preview state
    const [logoError, setLogoError] = useState(false);

    // Reset form when page changes
    useEffect(() => {
        if (page) {
            setDescription(page.description || "");
            setLogoUrl(page.logoUrl || "");
            setAccentColor(page.accentColor || "");
            setTheme(page.theme || "system");
            setShowUptimeBars(page.showUptimeBars ?? true);
            setShowUptimePercentage(page.showUptimePercentage ?? true);
            setShowIncidentHistory(page.showIncidentHistory ?? true);
            setLogoError(false);
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
            const targetSlug = resolveSlug(page.slug, page.title);
            await toggleMutation.mutateAsync({
                slug: targetSlug,
                public: page.public,
                enabled: page.enabled,
                title: page.title,
                groupId: page.groupId || undefined,
                description,
                logoUrl: logoUrl || undefined,
                accentColor: accentColor || undefined,
                theme,
                showUptimeBars,
                showUptimePercentage,
                showIncidentHistory,
            });
            toast({
                title: "Configuration Saved",
                description: `${page.title} settings updated successfully.`,
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

    const isValidHexColor = (color: string) => {
        if (!color) return true;
        return /^#[0-9A-Fa-f]{6}$/.test(color);
    };

    const isValidLogoUrl = (url: string) => {
        if (!url) return true;
        return url.startsWith("http://") || url.startsWith("https://") || url.startsWith("data:image/");
    };

    return (
        <Dialog open={open} onOpenChange={onOpenChange}>
            <DialogContent className="w-[calc(100vw-2rem)] max-w-[500px] max-h-[90vh] overflow-y-auto">
                <DialogHeader>
                    <DialogTitle>Configure Status Page</DialogTitle>
                </DialogHeader>

                <div className="space-y-6 py-4">
                    {/* Branding Section */}
                    <div className="space-y-4">
                        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
                            Branding
                        </h3>

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

                        <div className="space-y-2">
                            <Label htmlFor="logoUrl">Logo URL</Label>
                            <Input
                                id="logoUrl"
                                placeholder="https://example.com/logo.png or data:image/..."
                                value={logoUrl}
                                onChange={(e) => {
                                    setLogoUrl(e.target.value);
                                    setLogoError(false);
                                }}
                                className={!isValidLogoUrl(logoUrl) ? "border-destructive" : ""}
                            />
                            {!isValidLogoUrl(logoUrl) && (
                                <p className="text-xs text-destructive">Must be http/https URL or data:image URI</p>
                            )}
                            {logoUrl && isValidLogoUrl(logoUrl) && (
                                <div className="flex items-center gap-2 mt-2 p-2 bg-muted/50 rounded-md">
                                    {logoError ? (
                                        <div className="flex items-center justify-center w-8 h-8 bg-muted rounded border border-border">
                                            <Image className="w-4 h-4 text-muted-foreground" />
                                        </div>
                                    ) : (
                                        <img
                                            src={logoUrl}
                                            alt="Logo preview"
                                            className="w-8 h-8 object-contain rounded"
                                            onError={() => setLogoError(true)}
                                        />
                                    )}
                                    <span className="text-xs text-muted-foreground">Preview</span>
                                </div>
                            )}
                        </div>

                        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                            <div className="space-y-2">
                                <Label htmlFor="accentColor">Accent Color</Label>
                                <div className="flex gap-2">
                                    <Input
                                        id="accentColor"
                                        placeholder="#00A3FF"
                                        value={accentColor}
                                        onChange={(e) => setAccentColor(e.target.value)}
                                        className={`flex-1 ${!isValidHexColor(accentColor) ? "border-destructive" : ""}`}
                                    />
                                    {accentColor && isValidHexColor(accentColor) && (
                                        <div
                                            className="w-10 h-10 rounded-md border border-border shrink-0"
                                            style={{ backgroundColor: accentColor }}
                                        />
                                    )}
                                </div>
                                {!isValidHexColor(accentColor) && (
                                    <p className="text-xs text-destructive">Must be #RRGGBB format</p>
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
                        </div>
                    </div>

                    {/* Display Options Section */}
                    <div className="space-y-4">
                        <h3 className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
                            Display Options
                        </h3>

                        <div className="space-y-3">
                            <div className="flex items-center justify-between gap-4">
                                <div className="min-w-0">
                                    <Label htmlFor="showUptimeBars" className="cursor-pointer">Show Uptime Bars</Label>
                                    <p className="text-xs text-muted-foreground">Display 90-day uptime history bars</p>
                                </div>
                                <Switch
                                    id="showUptimeBars"
                                    checked={showUptimeBars}
                                    onCheckedChange={setShowUptimeBars}
                                    className="shrink-0"
                                />
                            </div>

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

                <DialogFooter className="flex-col sm:flex-row gap-2">
                    <Button variant="outline" onClick={() => onOpenChange(false)} className="w-full sm:w-auto">
                        Cancel
                    </Button>
                    <Button
                        onClick={handleSave}
                        disabled={toggleMutation.isPending || !isValidHexColor(accentColor) || !isValidLogoUrl(logoUrl)}
                        className="w-full sm:w-auto"
                    >
                        {toggleMutation.isPending && <Loader2 className="w-4 h-4 mr-2 animate-spin" />}
                        Save Changes
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}
