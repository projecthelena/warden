import { useQuery } from "@tanstack/react-query";
import { Group, useMonitorStore } from "@/lib/store";

const API_URL = import.meta.env.VITE_API_URL || "";

// Reuse the fetch logic but wrapped in a pure function
async function fetchMonitorsData(): Promise<Group[]> {
    const res = await fetch(`${API_URL}/api/uptime`, { credentials: 'include' });
    if (!res.ok) throw new Error("Failed to fetch uptime history");
    const data = await res.json();
    return data.groups || [];
}

export function useMonitorsQuery() {
    const { setGroups, isAuthChecked, user } = useMonitorStore();

    return useQuery({
        queryKey: ["monitors"],
        queryFn: async () => {
            const groups = await fetchMonitorsData();
            // Sync with Zustand for now to keep existing components working if they read from store
            // Ideally we migrate read components to use this hook too, but step by step.
            setGroups(groups);
            return groups;
        },
        refetchInterval: 30000, // Poll every 30 seconds
        refetchIntervalInBackground: true, // Keep polling even when tab is backgrounded
        enabled: isAuthChecked && !!user, // Only fetch if authenticated
        staleTime: 0, // Ensure data is always considered stale so invalidation works immediately
        refetchOnMount: true, // Always refetch on mount
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
            // Invalidate monitors to refresh the list
            queryClient.invalidateQueries({ queryKey: ["monitors"] });
            // Also invalidate overview just in case, though monitors query drives it now.
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
    url: string;
    groupId: string;
    interval: number;
}

async function createMonitorReq(payload: CreateMonitorPayload) {
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
        },
    });
}
