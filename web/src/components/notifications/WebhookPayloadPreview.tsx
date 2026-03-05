const samplePayload = JSON.stringify(
    {
        event: "down",
        monitorId: "mon-abc123",
        monitorName: "Example Monitor",
        monitorUrl: "https://example.com",
        message: "Connection refused after 10s timeout",
        timestamp: new Date().toISOString(),
    },
    null,
    2
);

export function WebhookPayloadPreview() {
    return (
        <div className="space-y-2">
            <p className="text-[0.8rem] text-muted-foreground">
                Your endpoint will receive a POST request with this JSON payload.
            </p>
            <pre className="rounded-md border bg-muted/30 p-3 text-xs font-mono overflow-x-auto">
                {samplePayload}
            </pre>
        </div>
    );
}
