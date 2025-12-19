import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { StatusPage } from "@/lib/store";

const API_URL = import.meta.env.VITE_API_URL || "";

async function fetchStatusPagesData(): Promise<StatusPage[]> {
    const res = await fetch(`${API_URL}/api/status-pages`, { credentials: 'include' });
    if (!res.ok) throw new Error("Failed to fetch status pages");
    const data = await res.json();
    return data.pages || [];
}

async function toggleStatusPageReq({ slug, public: isPublic, title, groupId }: { slug: string, public: boolean, title: string, groupId?: string }) {
    const res = await fetch(`${API_URL}/api/status-pages/${slug}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ public: isPublic, title, groupId }),
        credentials: 'include'
    });
    if (!res.ok) throw new Error("Failed to toggle status page");
    return res.json();
}

export function useStatusPagesQuery() {
    return useQuery({
        queryKey: ["status-pages"],
        queryFn: fetchStatusPagesData,
    });
}

export function useToggleStatusPageMutation() {
    const queryClient = useQueryClient();

    return useMutation({
        mutationFn: toggleStatusPageReq,
        onSuccess: () => {
            queryClient.invalidateQueries({ queryKey: ["status-pages"] });
        },
    });
}
