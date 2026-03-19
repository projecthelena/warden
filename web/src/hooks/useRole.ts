import { useMonitorStore } from "@/lib/store";

export function useRole() {
    const user = useMonitorStore((s) => s.user);
    const role = user?.role || "viewer";

    return {
        role,
        isAdmin: role === "admin",
        canEdit: role === "admin" || role === "editor",
        isViewer: role === "viewer",
        isStatusViewer: role === "status_viewer",
    };
}
