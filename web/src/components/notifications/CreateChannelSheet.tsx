import { useState } from "react";
import { Plus, Bell, Mail, Slack, Webhook, MessageSquare } from "lucide-react";
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

export function CreateChannelSheet({ onCreate }: { onCreate?: (c: any) => void }) {
    const { addChannel } = useMonitorStore();
    const [name, setName] = useState("");
    const [type, setType] = useState<NotificationChannel['type']>("slack");
    const [webhookUrl, setWebhookUrl] = useState("");
    const [email, setEmail] = useState("");
    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

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
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle className="text-slate-100 flex items-center gap-2">
                        <Bell className="w-5 h-5 text-blue-500" />
                        Add Notification Channel
                    </SheetTitle>
                    <SheetDescription className="text-slate-400">
                        Connect a new destination to receive alerts.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label>Channel Type</Label>
                        <Select value={type} onValueChange={(v: any) => setType(v)}>
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
                                    <div className="flex items-center gap-2"><Webhook className="w-4 h-4" /> Generic Webhook</div>
                                </SelectItem>
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-2">
                        <Label>Friendly Name</Label>
                        <Input value={name} onChange={e => setName(e.target.value)} required
                            className="bg-slate-900 border-slate-800" placeholder="e.g. DevOps Team" />
                    </div>

                    {type === 'email' ? (
                        <div className="grid gap-2">
                            <Label>Email Address</Label>
                            <Input value={email} onChange={e => setEmail(e.target.value)} required
                                type="email"
                                className="bg-slate-900 border-slate-800"
                                placeholder="team@example.com" />
                            <p className="text-[0.8rem] text-slate-500">
                                We will send alert summaries to this address.
                            </p>
                        </div>
                    ) : (
                        <div className="grid gap-2">
                            <Label>Webhook URL</Label>
                            <Input value={webhookUrl} onChange={e => setWebhookUrl(e.target.value)} required
                                type="url"
                                className="bg-slate-900 border-slate-800 font-mono text-xs"
                                placeholder="https://..." />
                            <p className="text-[0.8rem] text-slate-500">
                                {type === 'slack' ? 'Incoming Webhook URL from Slack App.' :
                                    type === 'discord' ? 'Discord Webhook URL.' : 'POST request URL.'}
                            </p>
                        </div>
                    )}

                    <SheetFooter className="mt-4">
                        <Button type="submit">Add Integration</Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
