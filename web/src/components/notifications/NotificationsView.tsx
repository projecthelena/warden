import { useState, useEffect } from "react";
import { useMonitorStore, NotificationChannel } from "@/lib/store";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { BellOff, MoreHorizontal, Pencil, Trash2 } from "lucide-react";
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
import { channelDisplayValue } from "@/lib/channelConfig";
import { ChannelIcon } from "./ChannelIcon";

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

    const getTypeLabel = (type: string) => {
        switch (type) {
            case 'slack': return 'Slack';
            case 'webhook': return 'Webhook';
            case 'email': return 'Email';
            case 'discord': return 'Discord';
            case 'telegram': return 'Telegram';
            default: return type;
        }
    }

    return (
        <Card>
            <CardHeader>
                <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                        <CardTitle>Channels</CardTitle>
                        <CardDescription>Where Warden sends alerts and summaries.</CardDescription>
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
                                            <ChannelIcon type={channel.type} />
                                            <span>{getTypeLabel(channel.type)}</span>
                                        </div>
                                    </TableCell>
                                    <TableCell className="max-w-48">
                                        <span className="block truncate text-muted-foreground font-mono text-xs">
                                            {channelDisplayValue(channel.config)}
                                        </span>
                                    </TableCell>
                                    <TableCell>
                                        <Badge variant={channel.enabled ? "secondary" : "outline"}>
                                            {channel.enabled ? "Active" : "Disabled"}
                                        </Badge>
                                    </TableCell>
                                    <TableCell>
                                        <DropdownMenu>
                                            <DropdownMenuTrigger asChild onClick={(e) => e.stopPropagation()}>
                                                <Button variant="ghost" size="icon" className="h-8 w-8" aria-label={`Open actions for ${channel.name}`}>
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
                    <div className="flex flex-col items-center justify-center rounded-lg border border-dashed border-border p-8 text-center text-muted-foreground">
                        <BellOff className="mb-3 h-8 w-8 opacity-50" />
                        <h3 className="mb-1 text-sm font-medium text-foreground">No channels yet</h3>
                        <p className="text-sm">Add one to receive alerts.</p>
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
