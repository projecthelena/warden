import { Group } from "@/lib/store";

const DEFAULT_POLLING_MS = 30_000; // 30s fallback when no active monitors
const MIN_POLLING_MS = 5_000;      // 5s floor
const MAX_POLLING_MS = 60_000;     // 60s ceiling

export function computePollingInterval(groups: Group[]): number {
  let min = Infinity;
  for (const group of groups) {
    for (const monitor of group.monitors) {
      if (monitor.active && monitor.status !== "paused") {
        if (monitor.interval < min) min = monitor.interval;
      }
    }
  }
  if (min === Infinity) return DEFAULT_POLLING_MS;
  return Math.max(MIN_POLLING_MS, Math.min(min * 1000, MAX_POLLING_MS));
}
