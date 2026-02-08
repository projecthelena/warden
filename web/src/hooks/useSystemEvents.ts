import { useQuery } from "@tanstack/react-query";
import { useMonitorStore, SystemIncident, SSLWarning } from "@/lib/store";

const API_URL = import.meta.env.VITE_API_URL || "";

interface SystemEventsResponse {
    active: SystemIncident[];
    history: SystemIncident[];
    sslWarnings: SSLWarning[];
}

async function fetchSystemEventsData(): Promise<SystemEventsResponse> {
    const res = await fetch(`${API_URL}/api/events`, { credentials: 'include' });
    if (!res.ok) throw new Error("Failed to fetch system events");
    return res.json();
}

export function useSystemEventsQuery() {
    const { setSystemEvents, isAuthChecked, user } = useMonitorStore();

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
        refetchInterval: 30000, // Poll every 30 seconds
        refetchIntervalInBackground: true, // Keep polling even when tab is backgrounded
        enabled: isAuthChecked && !!user, // Only fetch if authenticated
        staleTime: 0,
    });
}
