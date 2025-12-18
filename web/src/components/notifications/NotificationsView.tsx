import { useState, useEffect } from "react";
import { useMonitorStore, NotificationChannel } from "@/lib/store";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Slack, Mail, Webhook, MessageSquare, BellOff } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { ChannelDetailsSheet } from "./ChannelDetailsSheet";

export function NotificationsView() {
    const { channels, fetchChannels } = useMonitorStore();
    const [selectedChannel, setSelectedChannel] = useState<NotificationChannel | null>(null);
    const [detailsOpen, setDetailsOpen] = useState(false);

    useEffect(() => {
        fetchChannels();
    }, [fetchChannels]);

    const handleChannelClick = (channel: NotificationChannel) => {
        setSelectedChannel(channel);
        setDetailsOpen(true);
    };

    const getIcon = (type: string) => {
        switch (type) {
            case 'slack': return <Slack className="h-4 w-4 text-muted-foreground" />;
            case 'email': return <Mail className="h-4 w-4 text-muted-foreground" />;
            case 'discord': return <MessageSquare className="h-4 w-4 text-muted-foreground" />;
            case 'webhook': return <Webhook className="h-4 w-4 text-muted-foreground" />;
            default: return <Webhook className="h-4 w-4 text-muted-foreground" />;
        }
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const getDisplayValue = (config: any) => {
        if (config.email) return config.email;
        if (config.webhookUrl) return config.webhookUrl.replace('https://', '').substring(0, 20) + '...';
        return 'Configured';
    }

    return (
        <div className="space-y-6">
            <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                {channels.map((channel) => (
                    <Card
                        key={channel.id}
                        className="hover:bg-accent/50 cursor-pointer transition-colors group"
                        onClick={() => handleChannelClick(channel)}
                    >
                        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                            <CardTitle className="text-sm font-medium">
                                {channel.name}
                            </CardTitle>
                            {getIcon(channel.type)}
                        </CardHeader>
                        <CardContent>
                            <div className="text-2xl font-bold flex items-center gap-2">
                                <span className="text-sm font-normal text-muted-foreground truncate max-w-[200px] font-mono">
                                    {getDisplayValue(channel.config)}
                                </span>
                            </div>
                            <div className="flex items-center justify-between mt-4">
                                <Badge variant="secondary">Active</Badge>
                                <span className="text-xs text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity">
                                    Click to edit
                                </span>
                            </div>
                        </CardContent>
                    </Card>
                ))}
            </div>

            {
                selectedChannel && (
                    <ChannelDetailsSheet
                        channel={selectedChannel}
                        open={detailsOpen}
                        onOpenChange={setDetailsOpen}
                    />
                )
            }

            {
                channels.length === 0 && (
                    <div className="flex flex-col items-center justify-center p-12 border border-dashed border-slate-800 rounded-lg text-slate-500">
                        <BellOff className="w-12 h-12 mb-4 opacity-50" />
                        <h3 className="text-lg font-medium text-slate-300 mb-1">No Notification Channels</h3>
                        <p className="text-sm">Add a channel to receive alerts when monitors go down.</p>
                    </div>
                )
            }
        </div >
    )
}
