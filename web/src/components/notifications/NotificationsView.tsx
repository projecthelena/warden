import { useState, useEffect } from "react";
import { useMonitorStore, NotificationChannel } from "@/lib/store";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Slack, Webhook, BellOff, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";
import { ChannelDetailsSheet } from "./ChannelDetailsSheet";
import { CreateChannelSheet } from "./CreateChannelSheet";

export function NotificationsView() {
    const { channels, fetchChannels, deleteChannel } = useMonitorStore();
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
            case 'slack': return <Slack className="h-4 w-4" />;
            case 'webhook': return <Webhook className="h-4 w-4" />;
            default: return <Webhook className="h-4 w-4" />;
        }
    }

    const getTypeLabel = (type: string) => {
        switch (type) {
            case 'slack': return 'Slack';
            case 'webhook': return 'Webhook';
            default: return type;
        }
    }

    const getDisplayValue = (config: NotificationChannel['config']) => {
        if (config.webhookUrl) {
            const url = config.webhookUrl.replace('https://', '').replace('http://', '');
            return url.length > 35 ? url.substring(0, 35) + '...' : url;
        }
        return 'Configured';
    }

    return (
        <Card>
            <CardHeader>
                <div className="flex items-center justify-between">
                    <div>
                        <CardTitle>Notification Channels</CardTitle>
                        <CardDescription>Manage where alerts are delivered.</CardDescription>
                    </div>
                    <CreateChannelSheet />
                </div>
            </CardHeader>
            <CardContent>
                {channels.length > 0 ? (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Type</TableHead>
                                <TableHead>Destination</TableHead>
                                <TableHead>Status</TableHead>
                                <TableHead className="w-[50px]"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {channels.map((channel) => (
                                <TableRow
                                    key={channel.id}
                                    className="cursor-pointer"
                                    onClick={() => handleChannelClick(channel)}
                                >
                                    <TableCell className="font-medium">{channel.name}</TableCell>
                                    <TableCell>
                                        <div className="flex items-center gap-2 text-muted-foreground">
                                            {getIcon(channel.type)}
                                            <span>{getTypeLabel(channel.type)}</span>
                                        </div>
                                    </TableCell>
                                    <TableCell>
                                        <span className="text-muted-foreground font-mono text-xs">
                                            {getDisplayValue(channel.config)}
                                        </span>
                                    </TableCell>
                                    <TableCell>
                                        <Badge variant="secondary">Active</Badge>
                                    </TableCell>
                                    <TableCell>
                                        <DropdownMenu>
                                            <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                                                <Button variant="ghost" size="icon" className="h-8 w-8">
                                                    <MoreHorizontal className="h-4 w-4" />
                                                </Button>
                                            </DropdownMenuTrigger>
                                            <DropdownMenuContent align="end">
                                                <DropdownMenuItem onClick={(e) => {
                                                    e.stopPropagation();
                                                    handleChannelClick(channel);
                                                }}>
                                                    <Pencil className="h-4 w-4 mr-2" />
                                                    Edit
                                                </DropdownMenuItem>
                                                <DropdownMenuItem
                                                    className="text-destructive focus:text-destructive"
                                                    onClick={(e) => {
                                                        e.stopPropagation();
                                                        deleteChannel(channel.id);
                                                    }}
                                                >
                                                    <Trash2 className="h-4 w-4 mr-2" />
                                                    Delete
                                                </DropdownMenuItem>
                                            </DropdownMenuContent>
                                        </DropdownMenu>
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                ) : (
                    <div className="flex flex-col items-center justify-center p-12 border border-dashed border-border rounded-lg text-muted-foreground">
                        <BellOff className="w-12 h-12 mb-4 opacity-50" />
                        <h3 className="text-lg font-medium text-foreground mb-1">No Notification Channels</h3>
                        <p className="text-sm">Add a channel to receive alerts when monitors go down.</p>
                    </div>
                )}
            </CardContent>

            {selectedChannel && (
                <ChannelDetailsSheet
                    channel={selectedChannel}
                    open={detailsOpen}
                    onOpenChange={setDetailsOpen}
                />
            )}
        </Card>
    )
}
