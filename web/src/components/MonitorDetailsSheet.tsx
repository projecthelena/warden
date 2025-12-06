import { useState } from "react";
import { Monitor, useMonitorStore } from "@/lib/store";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Button } from "@/components/ui/button";
import { StatusBadge } from "@/components/ui/monitor-visuals";
import { Trash2, Save, Activity, Clock } from "lucide-react";

interface MonitorDetailsSheetProps {
    monitor: Monitor;
    open: boolean;
    onOpenChange: (open: boolean) => void;
}

export function MonitorDetailsSheet({ monitor, open, onOpenChange }: MonitorDetailsSheetProps) {
    const { updateMonitor, deleteMonitor } = useMonitorStore();
    const [name, setName] = useState(monitor.name);
    const [url, setUrl] = useState(monitor.url);

    const handleSave = () => {
        updateMonitor(monitor.id, { name, url });
        onOpenChange(false);
    };

    const handleDelete = () => {
        if (confirm("Are you sure you want to delete this monitor?")) {
            deleteMonitor(monitor.id);
            onOpenChange(false);
        }
    };

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
                            <Button onClick={handleSave} className="w-full bg-blue-600 hover:bg-blue-500">
                                <Save className="w-4 h-4 mr-2" /> Save Changes
                            </Button>
                        </div>

                        <div className="pt-6 border-t border-slate-800">
                            <h3 className="text-sm font-medium text-red-500 mb-2">Danger Zone</h3>
                            <p className="text-xs text-slate-500 mb-4">
                                Deleting this monitor is irreversible. All history will be lost.
                            </p>
                            <Button variant="destructive" onClick={handleDelete} className="w-full bg-red-900/50 hover:bg-red-900 text-red-200 border border-red-900">
                                <Trash2 className="w-4 h-4 mr-2" /> Delete Monitor
                            </Button>
                        </div>
                    </TabsContent>
                </Tabs>
            </SheetContent>
        </Sheet>
    )
}
