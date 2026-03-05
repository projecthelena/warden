export function SlackPreview() {
    return (
        <div className="space-y-2">
            <p className="text-[0.8rem] text-muted-foreground">
                This is how alerts will appear in your Slack channel.
            </p>
            <div className="rounded-md border bg-muted/30 p-3">
                <div className="flex items-start gap-2">
                    <div className="w-1 self-stretch rounded-full bg-destructive shrink-0" />
                    <div className="space-y-2 text-sm min-w-0">
                        <p className="font-bold">Monitor Down</p>
                        <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-xs">
                            <div>
                                <span className="text-muted-foreground">Monitor</span>
                                <p className="font-medium">Example Monitor</p>
                            </div>
                            <div>
                                <span className="text-muted-foreground">URL</span>
                                <p className="font-medium truncate">https://example.com</p>
                            </div>
                            <div className="col-span-2">
                                <span className="text-muted-foreground">Message</span>
                                <p className="font-medium">Connection refused after 10s timeout</p>
                            </div>
                            <div>
                                <span className="text-muted-foreground">Time</span>
                                <p className="font-medium">{new Date().toLocaleString()}</p>
                            </div>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    );
}
