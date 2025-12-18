import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";
import { Trash2, Key } from "lucide-react";
import { Badge } from "@/components/ui/badge";

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
} from "@/components/ui/alert-dialog"

export function APIKeysView() {
    const { apiKeys, fetchAPIKeys, deleteAPIKey } = useMonitorStore();
    const [loading, setLoading] = useState(true);
    const { toast } = useToast();

    useEffect(() => {
        setLoading(true);
        fetchAPIKeys().finally(() => setLoading(false));
    }, [fetchAPIKeys]);

    const handleDelete = async (id: number) => {
        await deleteAPIKey(id);
        await fetchAPIKeys();
        toast({ title: "Revoked", description: "API Key revoked successfully." });
    };

    return (
        <div className="space-y-6">
            {loading ? (
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                    {[1, 2].map(i => (
                        <Card key={i} className="animate-pulse bg-muted/20 h-32" />
                    ))}
                </div>
            ) : apiKeys.length === 0 ? (
                <div className="flex flex-col items-center justify-center p-12 border border-dashed border-border rounded-lg text-muted-foreground">
                    <Key className="w-12 h-12 mb-4 opacity-50" />
                    <h3 className="text-lg font-medium text-foreground mb-1">No API Keys</h3>
                    <p className="text-sm">Generate a key to access the API programmatically.</p>
                </div>
            ) : (
                <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
                    {apiKeys.map((key) => (
                        <Card key={key.id} className="group hover:border-foreground/20 transition-all">
                            <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-2">
                                <div className="space-y-1">
                                    <CardTitle className="text-sm font-medium flex items-center gap-2">
                                        <Key className="w-4 h-4 text-muted-foreground" />
                                        {key.name}
                                    </CardTitle>
                                    <CardDescription className="font-mono text-xs">
                                        {key.keyPrefix}••••••••
                                    </CardDescription>
                                </div>
                                <AlertDialog>
                                    <AlertDialogTrigger asChild>
                                        <Button
                                            variant="ghost"
                                            size="icon"
                                            className="h-8 w-8 text-muted-foreground hover:text-red-500 hover:bg-red-500/10 opacity-0 group-hover:opacity-100 transition-opacity"
                                        >
                                            <Trash2 className="w-4 h-4" />
                                        </Button>
                                    </AlertDialogTrigger>
                                    <AlertDialogContent>
                                        <AlertDialogHeader>
                                            <AlertDialogTitle>Are you absolutely sure?</AlertDialogTitle>
                                            <AlertDialogDescription>
                                                This action cannot be undone. This will permanently revoke the API key
                                                and any applications using it will lose access immediately.
                                            </AlertDialogDescription>
                                        </AlertDialogHeader>
                                        <AlertDialogFooter>
                                            <AlertDialogCancel>Cancel</AlertDialogCancel>
                                            <AlertDialogAction onClick={() => handleDelete(key.id)} className="bg-red-600 hover:bg-red-700">
                                                Revoke Key
                                            </AlertDialogAction>
                                        </AlertDialogFooter>
                                    </AlertDialogContent>
                                </AlertDialog>
                            </CardHeader>
                            <CardContent>
                                <div className="flex items-center justify-between mt-4">
                                    <Badge variant="outline" className="text-[10px] font-normal text-muted-foreground">
                                        Created {new Date(key.createdAt).toLocaleDateString()}
                                    </Badge>
                                </div>
                            </CardContent>
                        </Card>
                    ))}
                </div>
            )}
        </div>
    )
}
