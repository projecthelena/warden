import { useState } from "react";
import { Plus, Bell, Slack } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
} from "@/components/ui/sheet";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { useMonitorStore, NotificationChannel } from "@/lib/store";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function CreateChannelSheet({ onCreate }: { onCreate?: (c: any) => void }) {
    const { addChannel } = useMonitorStore();
    const [name, setName] = useState("");
    const [type, setType] = useState<NotificationChannel['type']>("slack");
    const [webhookUrl, setWebhookUrl] = useState("");
    const [email, setEmail] = useState("");
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const config: any = {};
        if (type === 'email') {
            config.email = email;
        } else {
            config.webhookUrl = webhookUrl;
        }

        const channelData = {
            type,
            name,
            config,
            enabled: true
        };

        if (onCreate) {
            onCreate(channelData);
        } else {
            addChannel(channelData);
        }

        setOpen(false);
        setName("");
        setWebhookUrl("");
        setEmail("");
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button size="sm" className="gap-2">
                    <Plus className="w-4 h-4" /> Add Channel
                </Button>
            </SheetTrigger>
            <SheetContent className="sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="flex items-center gap-2">
                        <Bell className="w-5 h-5" />
                        Add Notification Channel
                    </SheetTitle>
                    <SheetDescription>
                        Connect a new destination to receive alerts.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label>Channel Type</Label>
                        <Select value={type} onValueChange={(v: NotificationChannel['type']) => setType(v)}>
                            <SelectTrigger>
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="slack">
                                    <div className="flex items-center gap-2"><Slack className="w-4 h-4" /> Slack</div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-2">
                        <Label>Friendly Name</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} required
                            placeholder="e.g. DevOps Team" />
                    </div>

                    <div className="grid gap-2">
                        <Label>Webhook URL</Label>
                        <Input value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} required
                            type="url"
                            className="font-mono text-xs"
                            placeholder="https://hooks.slack.com/services/..." />
                        <p className="text-[0.8rem] text-muted-foreground">
                            Incoming Webhook URL from Slack App.
                        </p>
                    </div>

                    <SheetFooter className="mt-4">
                        <Button type="submit">Add Integration</Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
