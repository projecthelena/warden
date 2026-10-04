import { CheckCircle2, AlertCircle, RotateCcw, ChevronDown, ChevronLeft, ChevronRight } from "lucide-react";
import "./checks.css";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { HTTPDiagnosticDetails } from "@/components/HTTPDiagnosticDetails";
import { Button } from "@/components/ui/button";
import { explainCheck, recoveredByRetry, type CheckEvidence } from "@/lib/checkEvidence";

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
    if (query.isLoading) return <p className="py-8 text-sm text-muted-foreground" role="status">Loading checks…</p>;
    if (query.error) return <div className="space-y-3 rounded-lg border border-border p-5"><p role="alert">Could not load recent checks.</p><Button variant="outline" onClick={() => query.refetch()}>Try again</Button></div>;
    const checks = query.data ?? [];
    return <section className="space-y-5" aria-label="Recent checks">
        <div className="rounded-xl border border-border bg-card px-4 py-3 sm:px-5 sm:py-4">
            <h2 className="mb-3 text-xs font-medium text-muted-foreground">Last 24 hours</h2>
            {totals.data ? <>
                <dl className="grid grid-cols-3 gap-3" aria-label="24-hour totals">
                    <Metric label="Checks" value={totals.data.total} />
                    <Metric label="Failed" value={totals.data.failed} tone="text-rose-700 dark:text-rose-400" />
                    <Metric label="Recovered" value={totals.data.recovered} tone="text-foreground" />
                </dl>
                {totals.data.traced < totals.data.total && <p className="mt-4 text-xs text-muted-foreground">Traces: {totals.data.traced} of {totals.data.total}. Recovery counts may be incomplete.</p>}
            </> : totals.error ? <p role="alert" className="text-sm text-muted-foreground">The 24-hour summary is unavailable. Individual checks are still shown below.</p> : <p role="status" className="text-sm text-muted-foreground">Loading summary…</p>}
        </div>
        <div className="overflow-hidden rounded-xl border border-border bg-card">
            <header className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
                <h2 className="text-sm font-medium">Check history</h2>
                <span className="text-xs text-muted-foreground">{before ? "Older checks" : "Latest checks"} · {checks.length}</span>
            </header>
            <div className="check-row check-column-head border-b border-border text-xs text-muted-foreground" aria-hidden="true"><span>Time</span><span>Result</span><span>Attempts</span><span>Total time</span><span /></div>
            {!checks.length && <p className="p-8 text-center text-sm text-muted-foreground">No checks recorded.</p>}
            {checks.map((check, i) => {
                const explanation = explainCheck(check);
                const recovered = recoveredByRetry(check);
                const Icon = recovered ? RotateCcw : check.status === "up" ? CheckCircle2 : AlertCircle;
                const attempts = check.diagnostics?.attempts;
                const title = recovered ? "Recovered" : check.status === "up" ? "Passed" : check.statusCode ? `HTTP ${check.statusCode}` : explanation.title;
                return <details key={check.id ?? `${check.timestamp}-${i}`} className="check-entry border-b border-border last:border-0">
                    <summary className="check-row cursor-pointer list-none hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring" aria-label={`${new Date(check.timestamp).toLocaleString()} · ${explanation.title}`}>
                        <time className="check-time text-xs tabular-nums text-muted-foreground" dateTime={check.timestamp}><span className="block">{new Date(check.timestamp).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })}</span><span className="block mt-1">{new Date(check.timestamp).toLocaleDateString([], { month: "short", day: "numeric" })}</span></time>
                        <span className={`check-result flex min-w-0 items-center gap-2 text-sm font-semibold ${recovered ? "text-foreground" : check.status === "up" ? "text-foreground" : "text-rose-700 dark:text-rose-400"}`}><Icon className="h-4 w-4 shrink-0" aria-hidden="true" /><span>{title}</span></span>
                        <span className="check-attempts text-xs tabular-nums text-muted-foreground">{attempts ? <>{attempts.length}<span className="sm:hidden"> attempts</span></> : "—"}</span>
                        <span className="check-duration text-right font-mono text-xs tabular-nums">{Math.round(check.diagnostics?.totalMs ?? check.latency).toLocaleString()} ms</span>
                        <ChevronDown className="check-chevron h-4 w-4 text-muted-foreground" aria-hidden="true" />
                    </summary>
                    <div className="border-t border-border bg-muted/20 px-4 py-5 sm:px-5 space-y-4">
                        <div className="space-y-1"><p className="text-sm font-medium">{explanation.title}</p>{explanation.action && <p className="max-w-prose text-sm text-muted-foreground">{explanation.action}</p>}</div>
                        {check.diagnostics ? <HTTPDiagnosticDetails diagnostics={check.diagnostics} /> : <p className="text-sm text-muted-foreground">No HTTP trace was recorded for this check.</p>}
                    </div>
                </details>;
            })}
        </div>
        <footer className="flex flex-wrap items-center justify-between gap-3">
            <p className="text-xs text-muted-foreground">Monitoring requests · within your retention period</p>
            <div className="flex gap-2">
                <Button className="min-h-11" variant="outline" disabled={pages.length === 1} onClick={() => setPages(current => current.slice(0, -1))}><ChevronLeft className="mr-1 h-4 w-4" />Newer</Button>
                <Button className="min-h-11" variant="outline" disabled={checks.length < 20 || !checks.at(-1)?.id} onClick={() => setPages(current => [...current, checks.at(-1)!.id!])}>Older<ChevronRight className="ml-1 h-4 w-4" /></Button>
            </div>
        </footer>
    </section>;
}

function Metric({ label, value, tone = "text-foreground" }: { label: string; value: number; tone?: string }) {
    return <div><dt className="text-xs text-muted-foreground">{label}</dt><dd className={`mt-1 text-2xl font-semibold tabular-nums ${tone}`}>{value.toLocaleString()}</dd></div>;
}
