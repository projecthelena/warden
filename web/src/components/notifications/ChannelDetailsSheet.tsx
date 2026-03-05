import { useState, useEffect } from "react";
import { Trash2, Save, Bell, Slack, Webhook, Loader2, Send } from "lucide-react";
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
} from "@/components/ui/sheet";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { useMonitorStore, NotificationChannel } from "@/lib/store";
import { SlackPreview } from "./SlackPreview";
import { WebhookPayloadPreview } from "./WebhookPayloadPreview";

interface ChannelDetailsSheetProps {
    channel: NotificationChannel;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function ChannelDetailsSheet({ channel, open, onOpenChange }: ChannelDetailsSheetProps) {
    const { updateChannel, deleteChannel, testChannel } = useMonitorStore();
    const [name, setName] = useState(channel.name);
    const [type, setType] = useState<NotificationChannel['type']>(channel.type);
    const [webhookUrl, setWebhookUrl] = useState(channel.config.webhookUrl || "");
    const [testing, setTesting] = useState(false);

    // Reset state when channel changes
    useEffect(() => {
        setName(channel.name);
        setType(channel.type);
        setWebhookUrl(channel.config.webhookUrl || "");
    }, [channel, open]);

    const handleSave = () => {
        updateChannel(channel.id, {
            name,
            type,
            config: { webhookUrl },
        });
        onOpenChange(false);
    };

    const handleDelete = () => {
        deleteChannel(channel.id);
        onOpenChange(false);
    };

    const handleTest = async () => {
        setTesting(true);
        await testChannel(type, { webhookUrl });
        setTesting(false);
    };

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="flex items-center gap-2">
                        <Bell className="w-5 h-5 text-primary" />
                        Edit Channel
                    </SheetTitle>
                    <SheetDescription>
                        Update configuration or remove this channel.
                    </SheetDescription>
                </SheetHeader>

                <div className="grid gap-6 py-6">
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
                                <SelectItem value="webhook">
                                    <div className="flex items-center gap-2"><Webhook className="w-4 h-4" /> Webhook</div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-2">
                        <Label>Friendly Name</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} />
                    </div>

                    <div className="grid gap-2">
                        <Label>Webhook URL</Label>
                        <Input
                            value={webhookUrl}
                            onChange={e => setWebhookUrl(e.target.value)}
                            className="font-mono text-xs"
                            placeholder={type === 'slack' ? "https://hooks.slack.com/services/..." : "https://your-endpoint.com/webhook"}
                        />
                        <p className="text-[0.8rem] text-muted-foreground">
                            {type === 'slack'
                                ? "Incoming Webhook URL from your Slack App."
                                : "Any HTTP endpoint that accepts POST requests with JSON."}
                        </p>
                    </div>

                    {type === 'slack' ? <SlackPreview /> : <WebhookPayloadPreview />}
                </div>

                <Separator className="my-4" />

                <SheetFooter className="flex-col sm:flex-row gap-2">
                    <Button variant="destructive" onClick={handleDelete} className="w-full sm:w-auto" data-testid="delete-channel-btn">
                        <Trash2 className="w-4 h-4 mr-2" /> Delete Channel
                    </Button>
                    <div className="flex gap-2 w-full sm:w-auto sm:ml-auto">
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
                        <Button onClick={handleSave}>
                            <Save className="w-4 h-4 mr-2" /> Save Changes
                        </Button>
                    </div>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    );
}
