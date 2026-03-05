import { useState, useEffect } from "react";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";
import { Key, MoreHorizontal, Trash2 } from "lucide-react";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import {
    DropdownMenu,
    DropdownMenuContent,
    DropdownMenuItem,
    DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { CreateAPIKeySheet } from "./CreateAPIKeySheet";

export function APIKeysView() {
    const { apiKeys, fetchAPIKeys, deleteAPIKey } = useMonitorStore();
    const { toast } = useToast();
    const [revokeId, setRevokeId] = useState<number | null>(null);

    useEffect(() => {
        fetchAPIKeys();
    }, [fetchAPIKeys]);

    const handleDelete = async () => {
        if (revokeId === null) return;
        await deleteAPIKey(revokeId);
        await fetchAPIKeys();
        setRevokeId(null);
        toast({ title: "Revoked", description: "API Key revoked successfully." });
    };

    return (
        <Card>
            <CardHeader>
                <div className="flex items-center justify-between">
                    <div>
                        <CardTitle>API Keys</CardTitle>
                        <CardDescription>Manage programmatic access to the API.</CardDescription>
                    </div>
                    <CreateAPIKeySheet />
                </div>
            </CardHeader>
            <CardContent>
                {apiKeys.length > 0 ? (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>Name</TableHead>
                                <TableHead>Key Prefix</TableHead>
                                <TableHead>Created</TableHead>
                                <TableHead className="w-[50px]"></TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {apiKeys.map((key) => (
                                <TableRow key={key.id}>
                                    <TableCell className="font-medium">{key.name}</TableCell>
                                    <TableCell>
                                        <span className="font-mono text-xs text-muted-foreground">
                                            {key.keyPrefix}••••••••
                                        </span>
                                    </TableCell>
                                    <TableCell className="text-muted-foreground">
                                        {new Date(key.createdAt).toLocaleDateString()}
                                    </TableCell>
                                    <TableCell>
                                        <DropdownMenu>
                                            <DropdownMenuTrigger asChild>
                                                <Button variant="ghost" size="icon" className="h-8 w-8">
                                                    <MoreHorizontal className="h-4 w-4" />
                                                </Button>
                                            </DropdownMenuTrigger>
                                            <DropdownMenuContent align="end">
                                                <DropdownMenuItem
                                                    className="text-destructive focus:text-destructive"
                                                    onClick={() => setRevokeId(key.id)}
                                                >
                                                    <Trash2 className="h-4 w-4 mr-2" />
                                                    Revoke
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
                        <Key className="w-12 h-12 mb-4 opacity-50" />
                        <h3 className="text-lg font-medium text-foreground mb-1">No API Keys</h3>
                        <p className="text-sm">Generate a key to access the API programmatically.</p>
                    </div>
                )}
            </CardContent>

            <AlertDialog open={revokeId !== null} onOpenChange={(open) => !open && setRevokeId(null)}>
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
                        <AlertDialogAction onClick={handleDelete} className="bg-red-600 hover:bg-red-700">
                            Revoke Key
                        </AlertDialogAction>
                    </AlertDialogFooter>
                </AlertDialogContent>
            </AlertDialog>
        </Card>
    )
}
