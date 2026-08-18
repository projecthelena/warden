export function EmailPreview() {
    return (
        <div className="space-y-2">
            <p className="text-[0.8rem] text-muted-foreground">
                Alerts arrive looking like this.
            </p>
            <div className="rounded-md border bg-muted/30 overflow-hidden">
                <div className="border-b px-3 py-2 text-xs">
                    <span className="text-muted-foreground">Subject: </span>
                    <span className="font-medium">[Warden] Monitor Down: Example Monitor</span>
                </div>
                <div className="border-l-2 border-l-destructive px-3 py-3 space-y-2">
                    <p className="text-sm font-medium">Monitor Down</p>
                    <p className="text-xs text-muted-foreground">
                        Connection refused after 10s timeout
                    </p>
                    <div className="text-xs font-mono text-muted-foreground">
                        https://example.com
                    </div>
                </div>
            </div>
        </div>
    );
}
