import { APIKeysView } from "./APIKeysView";
import { Separator } from "@/components/ui/separator";

export function APIKeysPage() {
    return (
        <div className="space-y-6">
            <div>
                <h3 className="text-lg font-medium">API Keys</h3>
                <p className="text-sm text-muted-foreground">
                    Manage API keys for programmatic access to the ClusterUptime API.
                </p>
            </div>
            <Separator />
            <APIKeysView />
        </div>
    );
}
