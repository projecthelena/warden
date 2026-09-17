import { AlertTriangle, CheckCircle2, PauseCircle, RefreshCw, XCircle } from "lucide-react";
import type { Incident, Monitor } from "@/lib/store";

interface StatusGroup {
    id: string;
    monitors: Array<Pick<Monitor, "status">>;
}

export function getOverallStatus(groups: StatusGroup[], incidents: Incident[], maintenanceGroupIds: Set<string>) {
    const effectiveIncidents = (incidents || []).filter((incident) => {
        if (incident.type !== "incident" || incident.status === "resolved") return false;
        if (!incident.affectedGroups || incident.affectedGroups.length === 0) return true;
        return !incident.affectedGroups.some((groupId) => maintenanceGroupIds.has(groupId));
    });

    const hasActiveOutage = effectiveIncidents.length > 0;
    const hasDown = groups.some(
        (group) => !maintenanceGroupIds.has(group.id) && group.monitors.some((monitor) => monitor.status === "down")
    );
    const hasDegraded = groups.some(
        (group) => !maintenanceGroupIds.has(group.id) && group.monitors.some((monitor) => monitor.status === "degraded")
    );
    const monitors = groups.flatMap((group) => group.monitors);
    const allMonitorsPaused = monitors.length > 0 && monitors.every((monitor) => monitor.status === "paused");
    const isUnderMaintenance = maintenanceGroupIds.size > 0;

    if (isUnderMaintenance && !hasActiveOutage && !hasDown) {
        return {
            icon: RefreshCw,
            label: "System Under Maintenance",
            description: "Scheduled maintenance is currently in progress.",
            color: "blue" as const,
        };
    }
    if (hasActiveOutage || hasDown) {
        return {
            icon: XCircle,
            label: "System Outage",
            description: "Some systems are experiencing issues.",
            color: "red" as const,
        };
    }
    if (hasDegraded) {
        return {
            icon: AlertTriangle,
            label: "Partially Degraded Service",
            description: "Some monitors are reporting degraded performance.",
            color: "yellow" as const,
        };
    }
    if (allMonitorsPaused) {
        return {
            icon: PauseCircle,
            label: "Monitoring Paused",
            description: "Health checks are temporarily paused.",
            color: "gray" as const,
        };
    }
    return {
        icon: CheckCircle2,
        label: "All Systems Operational",
        description: "All monitors are running normally.",
        color: "green" as const,
    };
}
