import { useState, useEffect } from "react";
import { Trash2, Save, Bell, Loader2, Send } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
    Sheet,
    SheetContent,
    SheetDescription,
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
import { Switch } from "@/components/ui/switch";
import { useMonitorStore, NotificationChannel } from "@/lib/store";
import { SlackPreview } from "./SlackPreview";
import { WebhookPayloadPreview } from "./WebhookPayloadPreview";
import { EmailPreview } from "./EmailPreview";
import { EmailChannelFields } from "./EmailChannelFields";
import { EmailConfig, emailConfigFromChannel, isEmailConfigured } from "@/lib/emailChannel";
import { ChannelIcon } from "./ChannelIcon";

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
    const [botToken, setBotToken] = useState(channel.config.botToken || "");
    const [chatId, setChatId] = useState(channel.config.chatId || "");
    const [email, setEmail] = useState<EmailConfig>(emailConfigFromChannel(channel.config));
    const [enabled, setEnabled] = useState(channel.enabled);
    const [testing, setTesting] = useState(false);

    const isEmail = type === "email";
    const isTelegram = type === "telegram";
    const config: Record<string, string | boolean> = isEmail ? { ...email } : isTelegram ? { botToken, chatId } : { webhookUrl };
    const canTest = isEmail ? isEmailConfigured(email) : isTelegram ? botToken !== "" && chatId !== "" : webhookUrl !== "";

    // Reset state when channel changes
    useEffect(() => {
        setName(channel.name);
        setType(channel.type);
        setWebhookUrl(channel.config.webhookUrl || "");
        setBotToken(channel.config.botToken || "");
        setChatId(channel.config.chatId || "");
        setEmail(emailConfigFromChannel(channel.config));
        setEnabled(channel.enabled);
    }, [channel, open]);

    const handleSave = () => {
        updateChannel(channel.id, {
            name,
            type,
            config,
            enabled,
        });
        onOpenChange(false);
    };

    const handleDelete = () => {
        deleteChannel(channel.id);
        onOpenChange(false);
    };

    const handleTest = async () => {
        setTesting(true);
        await testChannel(type, config);
        setTesting(false);
    };

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="sm:max-w-[500px] overflow-y-auto">
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
                                    <div className="flex items-center gap-2"><ChannelIcon type="slack" /> Slack</div>
                                </SelectItem>
                                <SelectItem value="webhook">
                                    <div className="flex items-center gap-2"><ChannelIcon type="webhook" /> Webhook</div>
                                </SelectItem>
                                <SelectItem value="email">
                                    <div className="flex items-center gap-2"><ChannelIcon type="email" /> Email</div>
                                </SelectItem>
                                <SelectItem value="discord">
                                    <div className="flex items-center gap-2"><ChannelIcon type="discord" /> Discord</div>
                                </SelectItem>
                                <SelectItem value="telegram">
                                    <div className="flex items-center gap-2"><ChannelIcon type="telegram" /> Telegram</div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="flex items-center justify-between rounded-md border border-border p-3">
                        <div className="grid gap-1">
                            <Label htmlFor="channel-enabled">Channel enabled</Label>
                            <p className="text-[0.8rem] text-muted-foreground">
                                Disabled channels do not receive alerts or daily digests.
                            </p>
                        </div>
                        <Switch
                            id="channel-enabled"
                            checked={enabled}
                            onCheckedChange={setEnabled}
                            data-testid="channel-enabled-switch"
                            aria-label="Channel enabled"
                        />
                    </div>

                    <div className="grid gap-2">
                        <Label>Friendly Name</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} data-testid="channel-details-name-input" />
                    </div>

                    {isEmail ? (
                        <EmailChannelFields config={email} onChange={setEmail} />
                    ) : isTelegram ? (
                        <div className="grid gap-4">
                            <div className="grid gap-2">
                                <Label>Bot Token</Label>
                                <Input value={botToken} onChange={e => setBotToken(e.target.value)} type="password" />
                            </div>
                            <div className="grid gap-2">
                                <Label>Chat ID</Label>
                                <Input value={chatId} onChange={e => setChatId(e.target.value)} />
                            </div>
                        </div>
                    ) : (
                        <div className="grid gap-2">
                            <Label>Webhook URL</Label>
                            <Input
                                value={webhookUrl}
                                onChange={e => setWebhookUrl(e.target.value)}
                                className="font-mono text-xs"
                                placeholder={type === 'slack' ? "https://hooks.slack.com/services/..." : type === 'discord' ? "https://discord.com/api/webhooks/..." : "https://your-endpoint.com/webhook"}
                            />
                            <p className="text-[0.8rem] text-muted-foreground">
                                {type === 'slack' || type === 'discord'
                                    ? `Incoming Webhook URL from your ${type === 'slack' ? 'Slack App' : 'Discord channel'}.`
                                    : "Any HTTP endpoint that accepts POST requests with JSON."}
                            </p>
                        </div>
                    )}

                    {isEmail ? <EmailPreview /> : type === 'slack' ? <SlackPreview /> : type === 'webhook' ? <WebhookPayloadPreview /> : null}
                </div>

                <Separator className="my-4" />

                <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
                    <Button
                        variant="ghost"
                        size="sm"
                        onClick={handleDelete}
                        className="self-start text-destructive hover:bg-destructive/10 hover:text-destructive"
                        data-testid="delete-channel-btn"
                    >
                        <Trash2 /> Delete
                    </Button>
                    <div className="grid grid-cols-2 gap-2 sm:flex">
                        <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            disabled={!canTest || testing}
                            onClick={handleTest}
                            data-testid="test-channel-btn"
                        >
                            {testing ? <Loader2 className="animate-spin" /> : <Send />}
                            Send Test
                        </Button>
                        <Button size="sm" onClick={handleSave} data-testid="save-channel-btn">
                            <Save /> Save
                        </Button>
                    </div>
                </div>
            </SheetContent>
        </Sheet>
    );
}
