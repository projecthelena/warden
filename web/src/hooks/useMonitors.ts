import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Group, Monitor, MonitorType, OverviewGroup, RequestConfig, useMonitorStore } from "@/lib/store";
import { computePollingInterval } from "@/lib/pollingInterval";

const API_URL = import.meta.env.VITE_API_URL || "";

// Reuse the fetch logic but wrapped in a pure function
async function fetchMonitorsData(): Promise<Group[]> {
    const res = await fetch(`${API_URL}/api/uptime`, { credentials: 'include' });
    if (!res.ok) throw new Error("Failed to fetch uptime history");
    const data = await res.json();
    return data.groups || [];
}

export function useMonitorsQuery(enabled = true) {
    const { setGroups, isAuthChecked, user, groups } = useMonitorStore();
    const pollingInterval = computePollingInterval(groups);

    return useQuery({
        queryKey: ["monitors"],
        queryFn: async () => {
            const groups = await fetchMonitorsData();
            // Sync with Zustand for now to keep existing components working if they read from store
            // Ideally we migrate read components to use this hook too, but step by step.
            setGroups(groups);
            return groups;
        },
        refetchInterval: pollingInterval,
        refetchIntervalInBackground: true, // Keep polling even when tab is backgrounded
        enabled: enabled && isAuthChecked && !!user, // Only fetch if authenticated
        staleTime: 0, // Ensure data is always considered stale so invalidation works immediately
        refetchOnMount: true, // Always refetch on mount
    });
}

async function fetchOverviewData(): Promise<OverviewGroup[]> {
    const res = await fetch(`${API_URL}/api/overview`, { credentials: "include" });
    if (!res.ok) throw new Error("Failed to fetch group overview");
    const data = await res.json();
    return data.groups || [];
}

export function useOverviewQuery() {
    const { isAuthChecked, user } = useMonitorStore();
    return useQuery({
        queryKey: ["overview"],
        queryFn: fetchOverviewData,
        enabled: isAuthChecked && !!user,
        staleTime: 5_000,
        refetchInterval: 30_000,
        refetchIntervalInBackground: true,
    });
}

export type GroupMonitorStatusFilter = "all" | "operational" | "issues" | "paused";

export interface GroupMonitorsResponse {
    group: Group;
    counts: { all: number; operational: number; issues: number; paused: number };
    pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

export interface GroupMonitorQuery {
    page: number;
    pageSize: number;
    search: string;
    status: GroupMonitorStatusFilter;
}

export function buildGroupMonitorsURL(groupId: string, query: GroupMonitorQuery) {
    const params = new URLSearchParams({
        page: String(query.page),
        page_size: String(query.pageSize),
        status: query.status,
    });
    if (query.search.trim()) params.set("search", query.search.trim());
    return `${API_URL}/api/groups/${encodeURIComponent(groupId)}/monitors?${params}`;
}

export function useGroupMonitorsQuery(groupId: string | undefined, query: GroupMonitorQuery) {
    const { isAuthChecked, user } = useMonitorStore();
    return useQuery({
        queryKey: ["group-monitors", groupId, query.page, query.pageSize, query.search, query.status],
        queryFn: async (): Promise<GroupMonitorsResponse> => {
            const res = await fetch(buildGroupMonitorsURL(groupId!, query), { credentials: "include" });
            if (!res.ok) {
                if (res.status === 404) throw new Error("Group not found");
                throw new Error("Failed to fetch group monitors");
            }
            return res.json();
        },
        enabled: isAuthChecked && !!user && !!groupId,
        placeholderData: keepPreviousData,
        refetchInterval: 30_000,
        refetchIntervalInBackground: true,
    });
}

import { useMutation, useQueryClient } from "@tanstack/react-query";

async function deleteGroupReq(id: string) {
    const res = await fetch(`${API_URL}/api/groups/${id}`, {
        method: 'DELETE',
        credentials: 'include'
    });
    if (!res.ok) throw new Error("Failed to delete group");
    return true;
}

export function useDeleteGroupMutation() {
    const queryClient = useQueryClient();


    return useMutation({
        mutationFn: deleteGroupReq,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["monitors"] });
            queryClient.invalidateQueries({ queryKey: ["group-monitors"] });
            queryClient.invalidateQueries({ queryKey: ["overview"] });
        },
    });
}

// Create Group
async function createGroupReq(name: string) {
    const res = await fetch(`${API_URL}/api/groups`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name }),
        credentials: 'include'
    });
    if (!res.ok) throw new Error("Failed to create group");
    return res.json();
}

export function useCreateGroupMutation() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: createGroupReq,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["monitors"] });
            queryClient.invalidateQueries({ queryKey: ["status-pages"] });
            queryClient.invalidateQueries({ queryKey: ["overview"] });
        },
    });
}

// Create Monitor
interface CreateMonitorPayload {
    name: string;
    type: MonitorType;
    url: string;
    groupId: string;
    interval: number;
    confirmationThreshold?: number;
    notificationCooldownMinutes?: number;
    latencyThreshold?: number;
    requestConfig?: RequestConfig;
}

// warning carries the reason when the monitor's first check failed, which the server
// already waited for before answering.
interface CreatedMonitor extends Monitor {
    warning?: string;
}

async function createMonitorReq(payload: CreateMonitorPayload): Promise<CreatedMonitor> {
    const res = await fetch(`${API_URL}/api/monitors`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        credentials: 'include'
    });
    if (!res.ok) throw new Error("Failed to create monitor");
    return res.json();
}

export function useCreateMonitorMutation() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: createMonitorReq,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["monitors"] });
            queryClient.invalidateQueries({ queryKey: ["group-monitors"] });
            queryClient.invalidateQueries({ queryKey: ["overview"] });
        },
    });
}
