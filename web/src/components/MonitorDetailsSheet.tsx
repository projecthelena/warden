import { useState, useEffect } from "react";
import { Monitor, useMonitorStore } from "@/lib/store";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/monitor-visuals";
import { Trash2, Save, Activity, Clock } from "lucide-react";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from "@/components/ui/alert-dialog";

interface MonitorDetailsSheetProps {
    monitor: Monitor;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function MonitorDetailsSheet({ monitor, open, onOpenChange }: MonitorDetailsSheetProps) {
    const { updateMonitor, deleteMonitor } = useMonitorStore();
    const [name, setName] = useState(monitor.name);
    const [url, setUrl] = useState(monitor.url);
    const [interval, setInterval] = useState(monitor.interval || 60);
    const [stats, setStats] = useState({ uptime24h: 100, uptime7d: 100, uptime30d: 100 });

    useEffect(() => {
        if (open) {
            setName(monitor.name);
            setUrl(monitor.url);
            setInterval(monitor.interval || 60);
        }
    }, [open, monitor]);

    useEffect(() => {
        if (open && monitor.id) {
            fetch(`/api/monitors/${monitor.id}/uptime`)
                .then(res => res.json())
                .then(data => setStats(data))
                .catch(err => console.error("Failed to fetch stats", err));
        }
    }, [open, monitor.id]);

    const handleSave = () => {
        updateMonitor(monitor.id, { name, url, interval });
        onOpenChange(false);
    };

    const formatUptime = (val: number) => {
        if (val === 100) return "100%";
        return val.toFixed(2) + "%";
    }

    return (
        <Sheet open={open} onOpenChange={onOpenChange}>
            <SheetContent className="bg-slate-950 border-slate-800 text-slate-100 sm:max-w-[500px] overflow-y-auto">
                <SheetHeader className="mb-6">
                    <div className="flex items-center justify-between">
                        <SheetTitle className="text-slate-100">{monitor.name}</SheetTitle>
                        <StatusBadge status={monitor.status} />
                    </div>
                    <SheetDescription className="text-slate-400 font-mono text-xs">
                        ID: {monitor.id}
                    </SheetDescription>

                    <div className="grid grid-cols-3 gap-2 mt-4">
                        <div className="bg-slate-900 border border-slate-800 rounded p-2 text-center">
                            <span className="text-xs text-slate-500 block mb-1">24h Uptime</span>
                            <span className="text-sm font-semibold text-green-400">{formatUptime(stats.uptime24h)}</span>
                        </div>
                        <div className="bg-slate-900 border border-slate-800 rounded p-2 text-center">
                            <span className="text-xs text-slate-500 block mb-1">7d Uptime</span>
                            <span className="text-sm font-semibold text-green-400">{formatUptime(stats.uptime7d)}</span>
                        </div>
                        <div className="bg-slate-900 border border-slate-800 rounded p-2 text-center">
                            <span className="text-xs text-slate-500 block mb-1">30d Uptime</span>
                            <span className="text-sm font-semibold text-green-400">{formatUptime(stats.uptime30d)}</span>
                        </div>
                    </div>
                </SheetHeader>

                <Tabs defaultValue="events" className="w-full">
                    <TabsList className="bg-slate-900 border border-slate-800 w-full">
                        <TabsTrigger value="events" className="flex-1">Events</TabsTrigger>
                        <TabsTrigger value="settings" className="flex-1">Settings</TabsTrigger>
                    </TabsList>

                    <TabsContent value="events" className="mt-6 space-y-4">
                        <h3 className="text-sm font-medium text-slate-300 mb-4 flex items-center gap-2">
                            <Activity className="w-4 h-4" /> Activity Log
                        </h3>
                        {monitor.events && monitor.events.length > 0 ? (
                            <div className="relative border-l border-slate-800 ml-2 space-y-6">
                                {monitor.events.map((event) => (
                                    <div key={event.id} className="ml-6 relative">
                                        <div className={`absolute -left-[31px] top-1 w-2.5 h-2.5 rounded-full ring-4 ring-slate-950 ${event.type === 'up' ? 'bg-green-500' :
                                            event.type === 'down' ? 'bg-red-500' : 'bg-yellow-500'
                                            }`} />
                                        <div className="flex flex-col gap-1">
                                            <span className="text-xs text-slate-500 flex items-center gap-1">
                                                <Clock className="w-3 h-3" />
                                                {new Date(event.timestamp).toLocaleString()}
                                            </span>
                                            <p className="text-sm text-slate-200">{event.message}</p>
                                        </div>
                                    </div>
                                ))}
                            </div>
                        ) : (
                            <div className="text-center py-12 text-slate-500 text-sm border border-dashed border-slate-800 rounded-lg">
                                No events recorded yet.
                            </div>
                        )}
                    </TabsContent>

                    <TabsContent value="settings" className="mt-6 space-y-6">
                        <div className="space-y-4">
                            <div className="grid gap-2">
                                <Label>Display Name</Label>
                                <Input value={name} onChange={e => setName(e.target.value)} className="bg-slate-900 border-slate-800" />
                            </div>
                            <div className="grid gap-2">
                                <Label>Target URL</Label>
                                <Input value={url} onChange={e => setUrl(e.target.value)} className="bg-slate-900 border-slate-800 font-mono text-xs" />
                            </div>
                            <div className="grid gap-2">
                                <Label>Check Frequency</Label>
                                <Select onValueChange={(v) => setInterval(Number(v))} value={interval.toString()}>
                                    <SelectTrigger className="bg-slate-900 border-slate-800 text-slate-100">
                                        <SelectValue placeholder="Select frequency" />
                                    </SelectTrigger>
                                    <SelectContent className="bg-slate-950 border-slate-800 text-slate-100">
                                        <SelectItem value="10" className="cursor-pointer">10 Seconds</SelectItem>
                                        <SelectItem value="30" className="cursor-pointer">30 Seconds</SelectItem>
                                        <SelectItem value="60" className="cursor-pointer">1 Minute</SelectItem>
                                        <SelectItem value="300" className="cursor-pointer">5 Minutes</SelectItem>
                                        <SelectItem value="600" className="cursor-pointer">10 Minutes</SelectItem>
                                    </SelectContent>
                                </Select>
                            </div>
                            <Button onClick={handleSave} className="w-full bg-blue-600 hover:bg-blue-500">
                                <Save className="w-4 h-4 mr-2" /> Save Changes
                            </Button>
                        </div>

                        <div className="pt-6 border-t border-slate-800">
                            <h3 className="text-sm font-medium text-red-500 mb-2">Danger Zone</h3>
                            <p className="text-xs text-slate-500 mb-4">
                                Deleting this monitor is irreversible. All history will be lost.
                            </p>
                            <AlertDialog>
                                <AlertDialogTrigger asChild>
                                    <Button variant="destructive" className="w-full bg-red-900/50 hover:bg-red-900 text-red-200 border border-red-900">
                                        <Trash2 className="w-4 h-4 mr-2" /> Delete Monitor
                                    </Button>
                                </AlertDialogTrigger>
                                <AlertDialogContent className="bg-slate-950 border-slate-800 text-slate-100">
                                    <AlertDialogHeader>
                                        <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                                        <AlertDialogDescription className="text-slate-400">
                                            This action cannot be undone. This will permanently delete the monitor
                                            <strong> {monitor.name} </strong> and remove all its data.
                                        </AlertDialogDescription>
                                    </AlertDialogHeader>
                                    <AlertDialogFooter>
                                        <AlertDialogCancel className="bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-200">Cancel</AlertDialogCancel>
                                        <AlertDialogAction
                                            onClick={() => {
                                                deleteMonitor(monitor.id);
                                                onOpenChange(false);
                                            }}
                                            className="bg-red-600 hover:bg-red-700 text-white border-none"
                                        >
                                            Delete Monitor
                                        </AlertDialogAction>
                                    </AlertDialogFooter>
                                </AlertDialogContent>
                            </AlertDialog>
                        </div>
                    </TabsContent>

                </Tabs>
            </SheetContent>
        </Sheet>
    )
}
