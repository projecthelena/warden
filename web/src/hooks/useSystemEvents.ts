import { useQuery } from "@tanstack/react-query";
import { useMonitorStore, SystemIncident, SSLWarning } from "@/lib/store";
import { computePollingInterval } from "@/lib/pollingInterval";

const API_URL = import.meta.env.VITE_API_URL || "";

export interface SystemEventsResponse {
    active: SystemIncident[];
    history: SystemIncident[];
    sslWarnings: SSLWarning[];
}

export interface SystemEventsFilter {
    date?: string;
    monitorId?: string;
    groupId?: string;
}

async function fetchSystemEventsData(filter: SystemEventsFilter = {}): Promise<SystemEventsResponse> {
    const params = new URLSearchParams();
    if (filter.date) params.set("date", filter.date);
    if (filter.monitorId) params.set("monitorId", filter.monitorId);
    if (filter.groupId) params.set("groupId", filter.groupId);
    const qs = params.toString();
    const url = qs ? `${API_URL}/api/events?${qs}` : `${API_URL}/api/events`;
    const res = await fetch(url, { credentials: 'include' });
    if (!res.ok) throw new Error("Failed to fetch system events");
    return res.json();
}

export function useSystemEventsQuery() {
    const { setSystemEvents, isAuthChecked, user, groups } = useMonitorStore();
    const pollingInterval = computePollingInterval(groups);

    return useQuery({
        queryKey: ["system-events"],
        queryFn: async () => {
            const data = await fetchSystemEventsData();

            // Validate data structure to avoid crashes (defensive)
            const safeData = {
                active: Array.isArray(data.active) ? data.active : [],
                history: Array.isArray(data.history) ? data.history : [],
                sslWarnings: Array.isArray(data.sslWarnings) ? data.sslWarnings : [],
            };

            setSystemEvents(safeData);
            return safeData;
        },
        refetchInterval: pollingInterval,
        refetchIntervalInBackground: true, // Keep polling even when tab is backgrounded
        enabled: isAuthChecked && !!user, // Only fetch if authenticated
        staleTime: 0,
    });
}

// useFilteredSystemEvents fetches a snapshot of /api/events with optional filters WITHOUT
// touching the global Zustand store. The App-level useSystemEventsQuery polls /api/events
// with no filters and writes to Zustand; any filtered page (IncidentsView, MonitorPage)
// must keep its data local so the global poll doesn't clobber it.
//
// Enabled whenever at least one filter dimension is set, so passing an empty filter is
// effectively a no-op (returns no data, no fetch).
export function useFilteredSystemEvents(filter: SystemEventsFilter) {
    const hasFilter = !!(filter.date || filter.monitorId || filter.groupId);
    return useQuery({
        queryKey: ["system-events", "filtered", filter.date ?? "", filter.monitorId ?? "", filter.groupId ?? ""],
        queryFn: () => fetchSystemEventsData(filter),
        enabled: hasFilter,
        staleTime: 30_000,
        refetchInterval: 30_000, // keep "ongoing" durations fresh
    });
}

// useSystemEventsByDate is preserved for callers that only need date scoping. New code
// should prefer useFilteredSystemEvents.
export function useSystemEventsByDate(date: string | undefined) {
    return useFilteredSystemEvents({ date });
}
