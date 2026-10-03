export function mergeStatusMonitors<T extends { id: string }>(current: T[], incoming: T[]): T[] {
    const merged = new Map(current.map((monitor) => [monitor.id, monitor]));
    incoming.forEach((monitor) => merged.set(monitor.id, monitor));
    return Array.from(merged.values());
}

export function buildStatusMonitorQuery(options: {
    group: string;
    page: number;
    pageSize: number;
    status: string;
    search?: string;
}): URLSearchParams {
    const query = new URLSearchParams({
        group: options.group,
        page: String(options.page),
        page_size: String(options.pageSize),
        status: options.status,
    });
    const search = options.search?.trim();
    if (search) query.set("search", search);
    return query;
}
