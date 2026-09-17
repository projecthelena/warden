export function formatDowntime(failedCheckCount: number, intervalSeconds: number): string | null {
    if (failedCheckCount <= 0) return null;
    const totalSeconds = failedCheckCount * intervalSeconds;
    if (totalSeconds < 60) return `~${totalSeconds}s downtime`;
    const roundedMinutes = Math.round(totalSeconds / 60);
    const hours = Math.floor(roundedMinutes / 60);
    const mins = roundedMinutes % 60;
    if (hours > 0) return `~${hours}h ${mins}m downtime`;
    return `~${mins}m downtime`;
}
