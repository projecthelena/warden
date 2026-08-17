import { useQuery } from "@tanstack/react-query";

// A pattern Warden found in a monitor's history. Distinct from an incident: an incident
// says something broke, a finding says something about the shape of how it keeps breaking.
export interface MonitorInsight {
    id: number;
    monitorId: string;
    monitorName: string;
    kind:
        | "latency_sawtooth"
        | "periodic_reset"
        | "time_of_day"
        | "repeat_offender"
        | "co_failure"
        | "latency_drift";
    summary: string;
    detail?: Record<string, unknown>;
    confidence: "high" | "medium";
    detectedAt: string;
}

// Findings are recomputed once a day, so there is nothing to gain from polling them.
const DAY_MS = 24 * 60 * 60 * 1000;

export function useMonitorInsights(monitorId: string | undefined) {
    return useQuery<MonitorInsight[]>({
        queryKey: ["insights", monitorId],
        enabled: !!monitorId,
        staleTime: DAY_MS,
        queryFn: async () => {
            const res = await fetch(`/api/monitors/${monitorId}/insights`, { credentials: "include" });
            if (!res.ok) throw new Error("Failed to load patterns");
            return res.json();
        },
    });
}
