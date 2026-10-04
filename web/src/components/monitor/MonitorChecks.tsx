import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { HTTPDiagnosticDetails } from "@/components/HTTPDiagnosticDetails";
import { Button } from "@/components/ui/button";
import { explainCheck, summarizeChecks, phaseLabels, type CheckEvidence } from "@/lib/checkEvidence";

export function MonitorChecks({ monitorId }: { monitorId: string }) {
    const [pages, setPages] = useState<number[]>([0]);
    const before = pages.at(-1) ?? 0;
    const query = useQuery({
        queryKey: ["monitor-checks", monitorId, before],
        queryFn: async (): Promise<CheckEvidence[]> => {
            const base = import.meta.env.VITE_API_URL || "";
            const response = await fetch(`${base}/api/monitors/${encodeURIComponent(monitorId)}/checks?limit=20${before ? `&beforeId=${before}` : ""}`, { credentials: "include" });
            if (!response.ok) throw new Error("Could not load checks");
            return response.json();
        },
        refetchInterval: before ? false : 30_000,
    });
    const totals = useQuery({
        queryKey: ["monitor-check-summary", monitorId],
        queryFn: async (): Promise<{ total: number; failed: number; recovered: number; traced: number; failurePhases: Record<string, number> }> => {
            const response = await fetch(`${import.meta.env.VITE_API_URL || ""}/api/monitors/${encodeURIComponent(monitorId)}/checks/summary`, { credentials: "include" });
            if (!response.ok) throw new Error("Could not load summary");
            return response.json();
        },
        refetchInterval: 30_000,
    });
    if (query.isLoading) return <p>Loading checks…</p>;
    if (query.error) return <p role="alert">Could not load recent checks.</p>;
    const checks = query.data ?? [];
    const summary = summarizeChecks(checks);
    return <section className="space-y-3" aria-label="Recent checks">
        <div className="rounded-lg border border-border bg-card p-4 space-y-2">
            {totals.data && <div className="space-y-1">
                <p className="font-medium">Last 24 hours</p>
                <p className="text-sm">{totals.data.total} checks · {totals.data.failed} failed · {totals.data.recovered} recovered after retry</p>
                <p className="text-sm text-muted-foreground">First failed phase: {Object.entries(totals.data.failurePhases).map(([phase, count]) => `${phaseLabels[phase] ?? phase}: ${count}`).join(" · ") || "none recorded"}</p>
                {totals.data.traced < totals.data.total && <p className="text-xs text-muted-foreground">Detailed evidence covers {totals.data.traced} of {totals.data.total} checks; recovery counts may be incomplete.</p>}
            </div>}
            {totals.error && <p role="alert">The 24-hour summary is unavailable. Individual checks are still shown below.</p>}
            <p className="text-sm font-medium">{summary.total} checks on this page · {summary.failed} failed · {summary.recovered} recovered after retry</p>
            <p className="text-sm text-muted-foreground">These are Warden's monitoring requests, not traffic from your application's users. Earlier failed attempts remain visible even when a retry succeeds.</p>
            {summary.phases.length > 0 && <p className="text-sm">Checks with recorded failures: {summary.phases.map(([phase, count]) => `${phaseLabels[phase] ?? phase}: ${count}`).join(" · ")}</p>}
            {summary.traced < summary.total && <p className="text-xs text-muted-foreground">{summary.total - summary.traced} checks have no detailed trace; retry and phase totals may be incomplete.</p>}
            <p className="text-xs text-muted-foreground">Counts cover this page only. Browse older checks within the configured retention period.</p>
        </div>
        {!checks.length && <p>No checks recorded.</p>}
        {checks.map((check, i) => {
            const explanation = explainCheck(check);
            return <details key={check.id ?? `${check.timestamp}-${i}`} className="border border-border rounded-lg bg-card p-3">
                <summary className="cursor-pointer text-sm">{new Date(check.timestamp).toLocaleString()} · {explanation.title} · {Math.round(check.diagnostics?.totalMs ?? check.latency)} ms{check.diagnostics?.totalMs !== undefined ? " total" : ""}</summary>
                <div className="mt-3 space-y-3">
                    <p className="text-sm">{explanation.detail}</p>
                    {explanation.action && <p className="text-sm text-muted-foreground">{explanation.action}</p>}
                    {check.diagnostics ? <details><summary className="cursor-pointer text-sm font-medium">Technical details</summary><div className="mt-3"><HTTPDiagnosticDetails diagnostics={check.diagnostics} /></div></details> : <p className="text-sm text-muted-foreground">No HTTP trace was recorded for this check.</p>}
                </div>
            </details>;
        })}
        <div className="flex gap-2">
            <Button variant="outline" disabled={pages.length === 1} onClick={() => setPages(current => current.slice(0, -1))}>Newer</Button>
            <Button variant="outline" disabled={checks.length < 20 || !checks.at(-1)?.id} onClick={() => setPages(current => [...current, checks.at(-1)!.id!])}>Older</Button>
        </div>
    </section>;
}
