export interface UptimeSummary {
    percent: number;
    totalChecks: number;
    downChecks: number;
    downtimeSeconds: number;
}

export function formatUptime(value: number): string {
    if (value >= 100) return "100%";
    if (value > 99.99) {
        const rounded = Math.round(value * 1_000) / 1_000;
        return `${Math.min(rounded, 99.999).toFixed(3)}%`;
    }
    return `${value.toFixed(2)}%`;
}

export function formatUptimeSummary(summary: UptimeSummary | undefined): string {
    if (!summary || summary.totalChecks === 0) return "No data";
    return formatUptime(summary.percent);
}

export function formatUptimePeriod(days: number): string {
    return days === 1 ? "Last 24 hours" : `Last ${days} days`;
}

export function uptimeTone(value: number): string {
    if (value >= 99) return "text-emerald-400";
    if (value >= 95) return "text-amber-400";
    return "text-rose-400";
}

export function uptimeSummaryTone(summary: UptimeSummary | undefined): string {
    if (!summary || summary.totalChecks === 0) return "text-muted-foreground";
    return uptimeTone(summary.percent);
}

export function formatDuration(seconds: number): string {
    if (seconds < 60) return `${seconds}s`;

    const totalMinutes = Math.round(seconds / 60);
    const days = Math.floor(totalMinutes / (24 * 60));
    const hours = Math.floor((totalMinutes % (24 * 60)) / 60);
    const minutes = totalMinutes % 60;
    const parts = [];
    if (days > 0) parts.push(`${days}d`);
    if (hours > 0) parts.push(`${hours}h`);
    if (minutes > 0 || parts.length === 0) parts.push(`${minutes}m`);
    return parts.join(" ");
}

export function formatUptimeDetail(summary: UptimeSummary | undefined): string {
    if (!summary || summary.totalChecks === 0) return "No checks in this range";
    if (summary.downChecks === 0) return `No downtime across ${summary.totalChecks.toLocaleString()} checks`;
    const checks = `${summary.downChecks.toLocaleString()} failed ${summary.downChecks === 1 ? "check" : "checks"}`;
    return `${formatDuration(summary.downtimeSeconds)} monitored downtime · ${checks}`;
}
