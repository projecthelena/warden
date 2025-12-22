
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { useMonitorStore } from "@/lib/store";
import { useToast } from "@/components/ui/use-toast";
import { Plus, Copy } from "lucide-react";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
    SheetFooter,
} from "@/components/ui/sheet"

export function CreateAPIKeySheet() {
    const { createAPIKey, fetchAPIKeys } = useMonitorStore();
    const { toast } = useToast();
    const [newKeyName, setNewKeyName] = useState("");
    const [createdKey, setCreatedKey] = useState<string | null>(null);
    const [isOpen, setIsOpen] = useState(false);

    const handleCreate = async () => {
        if (!newKeyName) return;
        const key = await createAPIKey(newKeyName);
        if (key) {
            setCreatedKey(key);
            setNewKeyName("");
            fetchAPIKeys(); // Refresh list
        } else {
            toast({ title: "Error", description: "Failed to create API Key", variant: "destructive" });
        }
    };

    const copyToClipboard = (text: string) => {
        navigator.clipboard.writeText(text);
        toast({ title: "Copied", description: "API Key copied to clipboard" });
    };

    const handleOpenChange = (open: boolean) => {
        setIsOpen(open);
        if (!open) {
            setCreatedKey(null); // Reset on close
        }
    }

    const closeSheet = () => {
        setIsOpen(false);
    };

    return (
        <Sheet open={isOpen} onOpenChange={handleOpenChange}>
            <SheetTrigger asChild>
                <Button size="sm" data-testid="create-apikey-trigger">
                    <Plus className="w-4 h-4 mr-2" />
                    Create API Key
                </Button>
            </SheetTrigger>
            <SheetContent>
                <SheetHeader>
                    <SheetTitle>Generate New API Key</SheetTitle>
                    <SheetDescription>
                        Give your key a name to identify it later.
                    </SheetDescription>
                </SheetHeader>

                {!createdKey ? (
                    <div className="grid gap-4 py-6">
                        <div className="grid gap-2">
                            <Label>Key Name</Label>
                            <Input
                                value={newKeyName}
                                onChange={(e) => setNewKeyName(e.target.value)}
                                placeholder="e.g. CI/CD Pipeline"
                                data-testid="apikey-name-input"
                            />
                        </div>
                        <SheetFooter>
                            <Button onClick={handleCreate} disabled={!newKeyName} data-testid="apikey-create-submit">Generate Key</Button>
                        </SheetFooter>
                    </div>
                ) : (
                    <div className="py-6 space-y-4">
                        <div className="p-4 bg-green-500/10 border border-green-500/50 rounded-lg text-sm text-green-500">
                            <strong>Success!</strong> Your API Key has been generated.
                        </div>
                        <div className="space-y-2">
                            <Label>API Key</Label>
                            <div className="text-xs text-muted-foreground mb-1">
                                Copy this now. It will not be shown again.
                            </div>
                            <div className="flex items-center gap-2">
                                <code className="flex-1 p-3 bg-muted rounded border font-mono text-sm break-all">
                                    {createdKey}
                                </code>
                                <Button size="icon" variant="outline" onClick={() => copyToClipboard(createdKey)}>
                                    <Copy className="w-4 h-4" />
                                </Button>
                            </div>
                        </div>
                        <SheetFooter className="mt-4">
                            <Button onClick={closeSheet} className="w-full">Done</Button>
                        </SheetFooter>
                    </div>
                )}
            </SheetContent>
        </Sheet>
    );
}
