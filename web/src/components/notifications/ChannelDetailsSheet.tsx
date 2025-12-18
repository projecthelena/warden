import { useState, useEffect } from "react";
import { Trash2, Save, Bell, Slack, Mail, Webhook, MessageSquare } from "lucide-react";
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

interface ChannelDetailsSheetProps {
    channel: NotificationChannel;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function ChannelDetailsSheet({ channel, open, onOpenChange }: ChannelDetailsSheetProps) {
    const { updateChannel, deleteChannel } = useMonitorStore();
    const [name, setName] = useState(channel.name);
    const [type, setType] = useState<NotificationChannel['type']>(channel.type);
    const [webhookUrl, setWebhookUrl] = useState(channel.config.webhookUrl || "");
    const [email, setEmail] = useState(channel.config.email || "");

    // Reset state when channel changes
    useEffect(() => {
        setName(channel.name);
        setType(channel.type);
        setWebhookUrl(channel.config.webhookUrl || "");
        setEmail(channel.config.email || "");
    }, [channel, open]);

    const handleSave = () => {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const config: any = {};
        if (type === 'email') {
            config.email = email;
        } else {
            config.webhookUrl = webhookUrl;
        }

        updateChannel(channel.id, {
            name,
            type,
            config
        });
        onOpenChange(false);
    };

    const handleDelete = () => {
        deleteChannel(channel.id);
        onOpenChange(false);
    };

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="text-slate-100 flex items-center gap-2">
                        <Bell className="w-5 h-5 text-blue-500" />
                        Edit Channel
                    </SheetTitle>
                    <SheetDescription className="text-slate-400">
                        Update configuration or remove this channel.
                    </SheetDescription>
                </SheetHeader>

                <div className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label>Channel Type</Label>
                        <Select value={type} onValueChange={(v: NotificationChannel['type']) => setType(v)}>
                            <SelectTrigger className="bg-slate-900 border-slate-800">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                <SelectItem value="slack">
                                    <div className="flex items-center gap-2"><Slack className="w-4 h-4" /> Slack</div>
                                </SelectItem>
                                <SelectItem value="email">
                                    <div className="flex items-center gap-2"><Mail className="w-4 h-4" /> Email</div>
                                </SelectItem>
                                <SelectItem value="discord">
                                    <div className="flex items-center gap-2"><MessageSquare className="w-4 h-4" /> Discord</div>
                                </SelectItem>
                                <SelectItem value="webhook">
                                    <div className="flex items-center gap-2"><Webhook className="w-4 h-4" /> Global Webhook</div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-2">
                        <Label>Friendly Name</Label>
                        <Input value={name} onChange={e => setName(e.target.value)}
                            className="bg-slate-900 border-slate-800" />
                    </div>

                    {type === 'email' ? (
                        <div className="grid gap-2">
                            <Label>Email Address</Label>
                            <Input value={email} onChange={e => setEmail(e.target.value)}
                                className="bg-slate-900 border-slate-800" />
                        </div>
                    ) : (
                        <div className="grid gap-2">
                            <Label>Webhook URL</Label>
                            <Input value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)}
                                className="bg-slate-900 border-slate-800 font-mono text-xs" />
                        </div>
                    )}
                </div>

                <Separator className="bg-slate-800 my-4" />

                <SheetFooter className="flex-col sm:flex-row gap-2">
                    <Button variant="destructive" onClick={handleDelete} className="w-full sm:w-auto">
                        <Trash2 className="w-4 h-4 mr-2" /> Delete Channel
                    </Button>
                    <Button onClick={handleSave} className="w-full sm:w-auto ml-auto">
                        <Save className="w-4 h-4 mr-2" /> Save Changes
                    </Button>
                </SheetFooter>
            </SheetContent>
        </Sheet>
    );
}
