import { MonitorInsight, useMonitorInsights } from "@/hooks/useInsights";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import {
  TrendingUp,
  TrendingDown,
  Clock,
  Repeat,
  Link2,
  Waves,
} from "lucide-react";

const KIND_LABEL: Record<MonitorInsight["kind"], string> = {
  latency_sawtooth: "Climbs and resets",
  periodic_reset: "On a schedule",
  time_of_day: "Time of day",
  co_failure: "Fails with another monitor",
  latency_drift: "Getting slower",
  latency_improved: "Getting faster",
};

const NEXT_STEP: Record<MonitorInsight["kind"], string> = {
  latency_sawtooth: "Compare slow checks with deployment or restart history.",
  periodic_reset:
    "Check scheduled jobs and restart history against this cadence.",
  time_of_day: "Compare traffic and scheduled jobs during this time window.",
  co_failure:
    "Compare failed checks at the same time. Check the monitoring host and shared network before blaming the services.",
  latency_drift: "Compare slow and normal checks with recent changes.",
  latency_improved:
    "Check whether the improvement follows a deployment or configuration change.",
};

function observation(insight: MonitorInsight): string {
  const d = insight.detail;
  if (!d) return insight.summary;
  if (
    insight.kind === "latency_sawtooth" &&
    typeof d.ramps === "number" &&
    typeof d.baselineMs === "number" &&
    typeof d.worstPeakMs === "number"
  ) {
    return `${d.ramps} rises followed by a return to normal. Baseline ${d.baselineMs} ms; highest hourly average ${d.worstPeakMs} ms.`;
  }
  if (
    (insight.kind === "latency_drift" || insight.kind === "latency_improved") &&
    typeof d.previousMedianMs === "number" &&
    typeof d.recentMedianMs === "number"
  ) {
    return `Typical hourly latency changed from ${d.previousMedianMs} ms to ${d.recentMedianMs} ms between last week and this week.`;
  }
  if (insight.kind === "periodic_reset" && typeof d.periodHours === "number") {
    return `Latency returns to normal roughly every ${d.periodHours} hours. The cause is not confirmed.`;
  }
  if (
    insight.kind === "time_of_day" &&
    typeof d.events === "number" &&
    typeof d.share === "number" &&
    typeof d.startHourUTC === "number" &&
    typeof d.endHourUTC === "number"
  ) {
    return `${Math.round(d.share * 100)}% of ${d.events} problems occurred between ${String(d.startHourUTC).padStart(2, "0")}:00 and ${String(d.endHourUTC).padStart(2, "0")}:00 UTC.`;
  }
  if (insight.kind === "co_failure" && typeof d.overlap === "number") {
    const repeated =
      typeof d.matchedOutages === "number"
        ? `${d.matchedOutages} distinct outages started within 5 minutes of each other. `
        : "Simultaneous starts were not recorded for this finding. ";
    return `${repeated}${d.overlap}% of this monitor's downtime overlapped${typeof d.reverseOverlap === "number" ? `; ${d.reverseOverlap}% in the other direction` : ""}.`;
  }
  return insight.summary;
}

function exampleCheck(
  value: unknown,
): { id: number; timestamp?: string; hasTrace?: boolean } | null {
  if (!value || typeof value !== "object") return null;
  const ref = value as Record<string, unknown>;
  if (
    typeof ref.id !== "number" ||
    !Number.isSafeInteger(ref.id) ||
    ref.id <= 0
  )
    return null;
  return {
    id: ref.id,
    timestamp: typeof ref.timestamp === "string" ? ref.timestamp : undefined,
    hasTrace: typeof ref.hasTrace === "boolean" ? ref.hasTrace : undefined,
  };
}

function CheckLink({
  monitorId,
  value,
  children,
  label,
}: {
  monitorId: string;
  value: unknown;
  children: React.ReactNode;
  label?: string;
}) {
  const ref = exampleCheck(value);
  if (!ref) return null;
  return (
    <a
      aria-label={label}
      className="inline-block rounded py-2 text-sm font-medium text-primary underline underline-offset-4 hover:text-primary/80 active:text-primary/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      href={`/monitors/${encodeURIComponent(monitorId)}?tab=checks&checkId=${ref.id}`}
    >
      {children}
    </a>
  );
}

function TraceStatus({ insight }: { insight: MonitorInsight }) {
  const evidence = insight.detail?.checkEvidence;
  const e =
    evidence && typeof evidence === "object"
      ? (evidence as Record<string, unknown>)
      : undefined;
  if (insight.traceCapture === "not_applicable")
    return (
      <p className="text-xs text-muted-foreground">
        HTTP traces do not apply to this monitor.
      </p>
    );
  const capture =
    insight.traceCapture === "enabled"
      ? "HTTP trace capture is on."
      : insight.traceCapture === "disabled"
        ? "HTTP trace capture is off."
        : "Current HTTP trace capture state is unavailable.";
  const coverage =
    !e || typeof e.total !== "number" || typeof e.traced !== "number"
      ? " Trace coverage was not recorded for this finding."
      : e.traced === 0
        ? " No traces were recorded in this window; patterns use timing and outage history."
        : e.traced < e.total
          ? ` Partial history: ${e.traced.toLocaleString()} of ${e.total.toLocaleString()} checks have traces.`
          : ` All ${e.total.toLocaleString()} checks in this window have traces.`;
  return (
    <p className="text-xs text-muted-foreground">
      {capture}
      {coverage}
    </p>
  );
}

function CheckEvidence({ insight }: { insight: MonitorInsight }) {
  const evidence = insight.detail?.checkEvidence;
  const e =
    evidence && typeof evidence === "object"
      ? (evidence as Record<string, unknown>)
      : undefined;
  const phases =
    e?.failurePhases && typeof e.failurePhases === "object"
      ? Object.entries(e.failurePhases).filter(
          ([, count]) => typeof count === "number" && count > 0,
        )
      : [];
  return (
    <div className="space-y-2 text-xs text-muted-foreground">
      {e && typeof e.recovered === "number" && e.recovered > 0 && (
        <p>{e.recovered} checks recovered on retry.</p>
      )}
      {phases.length > 0 && (
        <p>
          Recorded first-attempt failures:{" "}
          {phases
            .map(([phase, count]) => `${phase.replaceAll("_", " ")}: ${count}`)
            .join(" · ")}
          . Partial trace coverage may omit other failures.
        </p>
      )}
      {exampleCheck(insight.detail?.evidenceCheck) ? (
        <CheckLink
          monitorId={insight.monitorId}
          value={insight.detail?.evidenceCheck}
        >
          View example check
        </CheckLink>
      ) : (
        <p>No retained example check is available for this finding.</p>
      )}
      <CheckLink
        monitorId={insight.monitorId}
        value={insight.detail?.comparisonCheck}
      >
        View previous-period check
      </CheckLink>
    </div>
  );
}

function range(values: number[]): string | null {
  if (!values.length) return null;
  const min = Math.min(...values),
    max = Math.max(...values);
  return min === max ? String(min) : `${min}–${max}`;
}

function RelatedPatterns({ related }: { related: MonitorInsight[] }) {
  const [search, setSearch] = useState("");
  const [page, setPage] = useState(0);
  const name = (insight: MonitorInsight) =>
    String(
      insight.detail?.withMonitorName ??
        insight.detail?.withMonitorId ??
        "Related monitor",
    );
  const matches = related
    .filter((insight) =>
      name(insight).toLowerCase().includes(search.trim().toLowerCase()),
    )
    .sort((a, b) => name(a).localeCompare(name(b)));
  const visible = matches.slice(page * 5, (page + 1) * 5);
  const values = (key: string) =>
    related
      .map((insight) => insight.detail?.[key])
      .filter(
        (value): value is number =>
          typeof value === "number" && Number.isFinite(value),
      );
  const outages = range(values("matchedOutages")),
    overlap = range(values("overlap"));
  return (
    <div className="rounded-lg border border-border bg-muted/20 p-4 space-y-3">
      <h3 className="text-sm font-medium">
        Overlapping outages with {related.length}{" "}
        {related.length === 1 ? "monitor" : "monitors"}
      </h3>
      {(outages || overlap) && (
        <p className="text-sm text-foreground/90">
          {outages && `${outages} shared outages per match`}
          {outages && overlap && " · "}
          {overlap && `${overlap}% downtime overlap`}
        </p>
      )}
      <p className="text-xs text-muted-foreground">
        Timing matches do not confirm a shared cause.
      </p>
      <CheckEvidence insight={related[0]} />
      <details>
        <summary className="cursor-pointer rounded py-2 text-sm hover:text-foreground/80 active:text-foreground/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          View related monitors
        </summary>
        <div className="space-y-3 pt-3">
          <Input
            type="search"
            aria-label="Find a related monitor"
            placeholder="Find a monitor"
            value={search}
            onChange={(event) => {
              setSearch(event.target.value);
              setPage(0);
            }}
          />
          <ul className="divide-y divide-border">
            {visible.map((insight) => (
              <li
                key={insight.id}
                className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1 py-2 text-sm"
                data-testid="related-monitor-row"
              >
                <span className="min-w-0 break-words">{name(insight)}</span>
                {typeof insight.detail?.withMonitorId === "string" &&
                exampleCheck(insight.detail?.withCheck) ? (
                  <CheckLink
                    monitorId={insight.detail.withMonitorId}
                    value={insight.detail.withCheck}
                    label={`View check for ${name(insight)}`}
                  >
                    View check
                  </CheckLink>
                ) : (
                  <span className="text-xs text-muted-foreground">
                    Example check unavailable
                  </span>
                )}
              </li>
            ))}
          </ul>
          {!matches.length && (
            <p className="text-sm text-muted-foreground">
              No matching monitors.
            </p>
          )}
          {matches.length > 0 && (
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className="text-xs text-muted-foreground" aria-live="polite">
                {page * 5 + 1}–{Math.min((page + 1) * 5, matches.length)} of{" "}
                {matches.length}
              </p>
              {matches.length > 5 && (
                <div className="flex gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="min-h-11"
                    disabled={page === 0}
                    onClick={() => setPage((current) => current - 1)}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="min-h-11"
                    disabled={(page + 1) * 5 >= matches.length}
                    onClick={() => setPage((current) => current + 1)}
                  >
                    Next
                  </Button>
                </div>
              )}
            </div>
          )}
        </div>
      </details>
    </div>
  );
}

function KindIcon({ kind }: { kind: MonitorInsight["kind"] }) {
  const cls = "w-4 h-4 text-muted-foreground shrink-0 mt-0.5";
  switch (kind) {
    case "latency_sawtooth":
      return <Waves className={cls} />;
    case "periodic_reset":
      return <Repeat className={cls} />;
    case "time_of_day":
      return <Clock className={cls} />;
    case "co_failure":
      return <Link2 className={cls} />;
    case "latency_improved":
      return <TrendingDown className={cls} />;
    default:
      return <TrendingUp className={cls} />;
  }
}

export function InsightsCard({ monitorId }: { monitorId: string }) {
  const { data, isLoading, error } = useMonitorInsights(monitorId);

  if (isLoading) {
    return (
      <div role="status" aria-label="Loading patterns">
        <Skeleton className="h-20 w-full" />
      </div>
    );
  }
  if (error) {
    return (
      <div
        role="alert"
        className="rounded-lg border border-rose-500/30 bg-rose-500/5 p-4 text-sm text-rose-400"
      >
        Could not load patterns. Please try again later.
      </div>
    );
  }
  if (!data || data.length === 0) {
    return (
      <div className="grid min-h-44 place-content-center gap-2 rounded-xl border border-dashed border-border bg-card/30 px-4 text-center text-sm text-muted-foreground">
        <p>No patterns detected yet.</p>
        <p>
          Patterns are based on the last 14 days of checks and refreshed daily.
        </p>
      </div>
    );
  }

  const related = data.filter((insight) => insight.kind === "co_failure");
  const individual = data.filter((insight) => insight.kind !== "co_failure");
  const patternCount = individual.length + (related.length > 0 ? 1 : 0);

  return (
    <section data-testid="monitor-insights">
      <h2 className="text-sm font-medium text-foreground mb-3 flex items-center gap-2">
        <Waves className="w-4 h-4 text-muted-foreground" />
        Patterns ({patternCount})
      </h2>
      <div className="mb-3">
        <TraceStatus insight={data[0]} />
      </div>
      <div className="space-y-2">
        {individual.map((insight) => (
          <div
            key={insight.id}
            className="border border-border rounded-lg p-3 bg-muted/20 flex gap-3"
          >
            <KindIcon kind={insight.kind} />
            <div className="min-w-0 space-y-1">
              <div className="flex items-center gap-2 flex-wrap">
                <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                  {KIND_LABEL[insight.kind] ?? insight.kind}
                </span>
                {insight.confidence === "medium" && (
                  <span className="text-[10px] uppercase tracking-wide text-muted-foreground border border-border rounded px-1.5 py-0.5">
                    worth a look
                  </span>
                )}
              </div>
              <p className="text-sm text-foreground/90">
                {observation(insight)}
              </p>
              <p className="text-sm text-muted-foreground">
                {NEXT_STEP[insight.kind]}
              </p>
              <details className="text-xs text-muted-foreground">
                <summary className="cursor-pointer rounded py-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
                  View evidence
                </summary>
                <div className="space-y-2 py-2">
                  {observation(insight) !== insight.summary && (
                    <p>{insight.summary}</p>
                  )}
                  <CheckEvidence insight={insight} />
                  <p>
                    Last analyzed:{" "}
                    {new Date(insight.detectedAt).toLocaleString()}
                  </p>
                </div>
              </details>
            </div>
          </div>
        ))}
        {related.length > 0 && (
          <RelatedPatterns key={monitorId} related={related} />
        )}
      </div>
      <a
        href={`/monitors/${encodeURIComponent(monitorId)}?tab=checks`}
        className="inline-block rounded py-3 text-sm font-medium text-primary underline underline-offset-4 hover:text-primary/80 active:text-primary/70 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        All check history
      </a>
      <p className="text-xs text-muted-foreground mt-2">
        Found by looking at the last 14 days, refreshed daily. Warden reports
        the shape; what causes it is still your call.
      </p>
    </section>
  );
}
