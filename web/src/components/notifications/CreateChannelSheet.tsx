import { useState } from "react";
import { Plus, Bell, Slack, Webhook, Loader2, Send } from "lucide-react";
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
import { SlackPreview } from "./SlackPreview";
import { WebhookPayloadPreview } from "./WebhookPayloadPreview";

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function CreateChannelSheet({ onCreate }: { onCreate?: (c: any) => void }) {
    const { addChannel, testChannel } = useMonitorStore();
    const [name, setName] = useState("");
    const [type, setType] = useState<NotificationChannel['type']>("slack");
    const [webhookUrl, setWebhookUrl] = useState("");
    const [open, setOpen] = useState(false);
    const [testing, setTesting] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        const channelData = {
            type,
            name,
            config: { webhookUrl },
            enabled: true,
        };

        if (onCreate) {
            onCreate(channelData);
        } else {
            addChannel(channelData);
        }

        setOpen(false);
        setName("");
        setWebhookUrl("");
    };

    const handleTest = async () => {
        setTesting(true);
        await testChannel(type, { webhookUrl });
        setTesting(false);
    };

    return (
        <Sheet open={open} onOpenChange={setOpen}>
            <SheetTrigger asChild>
                <Button size="sm" className="gap-2" data-testid="create-channel-trigger">
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
                            <SelectTrigger data-testid="channel-type-select">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="slack" data-testid="channel-type-slack">
                                    <div className="flex items-center gap-2"><Slack className="w-4 h-4" /> Slack</div>
                                </SelectItem>
                                <SelectItem value="webhook" data-testid="channel-type-webhook">
                                    <div className="flex items-center gap-2"><Webhook className="w-4 h-4" /> Webhook</div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-2">
                        <Label>Friendly Name</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} required
                            placeholder="e.g. DevOps Team"
                            data-testid="channel-name-input" />
                    </div>

                    <div className="grid gap-2">
                        <Label>Webhook URL</Label>
                        <Input value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} required
                            type="url"
                            className="font-mono text-xs"
                            placeholder={type === 'slack' ? "https://hooks.slack.com/services/..." : "https://your-endpoint.com/webhook"}
                            data-testid="channel-webhook-input" />
                        <p className="text-[0.8rem] text-muted-foreground">
                            {type === 'slack'
                                ? "Incoming Webhook URL from your Slack App."
                                : "Any HTTP endpoint that accepts POST requests with JSON."}
                        </p>
                    </div>

                    {type === 'slack' ? <SlackPreview /> : <WebhookPayloadPreview />}

                    <SheetFooter className="mt-4 gap-2">
                        <Button
                            type="button"
                            variant="outline"
                            disabled={!webhookUrl || testing}
                            onClick={handleTest}
                            data-testid="test-channel-btn"
                        >
                            {testing ? <Loader2 className="w-4 h-4 mr-2 animate-spin" /> : <Send className="w-4 h-4 mr-2" />}
                            Send Test
                        </Button>
                        <Button type="submit" data-testid="create-channel-submit">Add Integration</Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
