import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";

import { useState } from "react";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";

export function SettingsView() {
    const { user, updateUser } = useMonitorStore();
    const { toast } = useToast();
    const [isLoading, setIsLoading] = useState(false);

    const handleUpdateProfile = async (e: React.FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        setIsLoading(true);
        const formData = new FormData(e.currentTarget);
        const timezone = formData.get("timezone") as string;
        const password = formData.get("password") as string;

        try {
            await updateUser({
                timezone,
                password: password || undefined
            });
            toast({ title: "Settings updated", description: "Your profile has been updated successfully." });
        } catch (error) {
            toast({ title: "Error", description: "Failed to update settings", variant: "destructive" });
        } finally {
            setIsLoading(false);
        }
    };

    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-medium">Settings</h3>
                <p className="text-sm text-muted-foreground">
                    Manage your workspace preferences.
                </p>
            </div>
            <Separator />
            <div className="grid gap-6">
                <Card className="bg-slate-900/20 border-slate-800">
                    <CardHeader>
                        <CardTitle>Account Settings</CardTitle>
                        <CardDescription>
                            Manage your account preferences and security.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <form onSubmit={handleUpdateProfile} className="space-y-4">
                            <div className="grid gap-2">
                                <Label>Username</Label>
                                <Input value={user?.name || ''} disabled className="bg-slate-900 border-slate-800 max-w-md" />
                            </div>
                            <div className="grid gap-2">
                                <Label>Timezone</Label>
                                <select
                                    name="timezone"
                                    defaultValue={user?.timezone || 'UTC'}
                                    className="flex h-9 w-full max-w-md rounded-md border border-slate-800 bg-slate-900 px-3 py-1 text-sm shadow-sm transition-colors file:border-0 file:bg-transparent file:text-sm file:font-medium placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
                                >
                                    <option value="UTC">UTC</option>
                                    <option value="America/New_York">America/New_York (EST)</option>
                                    <option value="America/Chicago">America/Chicago (CST)</option>
                                    <option value="America/Denver">America/Denver (MST)</option>
                                    <option value="America/Los_Angeles">America/Los_Angeles (PST)</option>
                                    <option value="Europe/London">Europe/London (GMT)</option>
                                    <option value="Europe/Paris">Europe/Paris (CET)</option>
                                    <option value="Asia/Tokyo">Asia/Tokyo (JST)</option>
                                </select>
                            </div>

                            <Separator />

                            <div className="grid gap-2">
                                <Label>New Password</Label>
                                <Input
                                    name="password"
                                    type="password"
                                    placeholder="Leave empty to keep current"
                                    className="bg-slate-900 border-slate-800 max-w-md"
                                />
                            </div>

                            <Button type="submit" disabled={isLoading}>
                                {isLoading ? "Saving..." : "Save Changes"}
                            </Button>
                        </form>
                    </CardContent>
                </Card>

                <Card className="bg-slate-900/20 border-slate-800">
                    <CardHeader>
                        <CardTitle>Appearance</CardTitle>
                        <CardDescription>
                            Customize how the dashboard looks.
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-4">
                        <div className="flex items-center justify-between max-w-md p-4 border border-slate-800 rounded-lg">
                            <div className="space-y-0.5">
                                <Label className="text-base">Dark Mode</Label>
                                <p className="text-sm text-slate-500">Enable dark theme for the interface.</p>
                            </div>
                            <Button variant="outline" size="sm" disabled>Enabled</Button>
                        </div>
                    </CardContent>
                </Card>
            </div>
        </div>
    )
}
