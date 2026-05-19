import { useQuery } from "@tanstack/react-query";

const API_URL = import.meta.env.VITE_API_URL || "";

export interface EnrichedMonitorEvent {
    id: string;
    type: string;
    message: string;
    timestamp: string;
    statusCode?: number;
    latency?: number;
    errorMessage?: string;
    responseBody?: string;
    responseHeaders?: Record<string, string>;
}

async function fetchMonitorEvents(id: string, date?: string, limit?: number): Promise<EnrichedMonitorEvent[]> {
    const params = new URLSearchParams();
    if (date) params.set("date", date);
    if (limit) params.set("limit", String(limit));
    const qs = params.toString();
    const url = `${API_URL}/api/monitors/${id}/events${qs ? `?${qs}` : ""}`;
    const res = await fetch(url, { credentials: "include" });
    if (!res.ok) throw new Error("Failed to fetch monitor events");
    return res.json();
}

export function useMonitorEvents(monitorId: string | undefined, date?: string, limit = 100) {
    return useQuery({
        queryKey: ["monitor-events", monitorId, date, limit],
        queryFn: () => fetchMonitorEvents(monitorId!, date, limit),
        enabled: !!monitorId,
        staleTime: 30_000,
    });
}

// useIncidentEvents loads the enriched events that fell inside an outage window. Used by
// IncidentCard's expand drawer to show what actually happened during a downtime period:
// status code, latency, error, response body and headers per failed check.
//
// IMPORTANT — query-key stability for ongoing outages:
// For a resolved outage we know `endedAt`. For an ongoing one the natural "to" is "now",
// but if we put a fresh `new Date().toISOString()` into the React Query key on every
// render, the key changes every render → refetch → setData → re-render → fresh timestamp
// → refetch — an infinite loop that hammers the backend until the rate limiter (429)
// catches up. To avoid that, the key uses the literal sentinel `"ongoing"` when the
// outage is open; the actual `to` parameter is computed inside `queryFn` at fetch time.
// Ongoing rows freshen via `refetchInterval`, not via key churn.
export function useIncidentEvents(
    monitorId: string | undefined,
    from: string | undefined,
    endedAt: string | null | undefined,
    enabled: boolean,
    limit = 200,
) {
    const isOngoing = !endedAt;
    return useQuery({
        queryKey: ["incident-events", monitorId, from, endedAt ?? "ongoing", limit],
        queryFn: async () => {
            const to = endedAt ?? new Date().toISOString();
            const params = new URLSearchParams();
            params.set("from", from!);
            params.set("to", to);
            params.set("limit", String(limit));
            const res = await fetch(
                `${API_URL}/api/monitors/${monitorId}/events?${params.toString()}`,
                { credentials: "include" },
            );
            if (!res.ok) throw new Error("Failed to fetch incident events");
            return (await res.json()) as EnrichedMonitorEvent[];
        },
        enabled: !!monitorId && !!from && enabled,
        staleTime: 60_000,
        // Resolved outages don't change; only poll while the outage is still ongoing.
        refetchInterval: isOngoing ? 60_000 : false,
    });
}

