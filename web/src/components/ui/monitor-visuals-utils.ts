import { Monitor } from "@/lib/store";

export interface DisplaySlot {
    time: Date;
    point: Monitor['history'][number] | null;
}

export function buildTimeSlots(
    history: Monitor['history'] | undefined,
    interval: number | undefined,
    now: number = Date.now()
): DisplaySlot[] {
    const LIMIT = 30;
    const step = (interval && interval > 0 ? interval : 60) * 1000; // ms
    const tolerance = step / 2;

    // Anchor to the latest history point so the rightmost bar always shows data.
    // Falls back to `now` only when there is no history at all.
    let anchor = now;
    if (history && history.length > 0) {
        const latestTs = Math.max(...history.map(h => new Date(h.timestamp).getTime()));
        // Anchor to latest point only if within 2 intervals of now (normal polling drift).
        // Beyond that, the gap is real (e.g. monitor was paused) — anchor to now to show it.
        if (now - latestTs <= 2 * step) {
            anchor = latestTs;
        }
    }

    // Build 30 time slots anchored to the latest check, going backward
    const slots: DisplaySlot[] = [];
    for (let i = 0; i < LIMIT; i++) {
        slots.push({
            time: new Date(anchor - (LIMIT - 1 - i) * step),
            point: null,
        });
    }

    if (!history || history.length === 0) {
        return slots;
    }

    // Sort history newest-first so that when multiple checks fall in the same slot,
    // the latest one wins (we assign on first match and skip duplicates)
    const sorted = [...history].sort(
        (a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime()
    );

    for (const point of sorted) {
        const ts = new Date(point.timestamp).getTime();
        let bestIdx = -1;
        let bestDist = Infinity;
        for (let i = 0; i < LIMIT; i++) {
            if (slots[i].point !== null) continue; // already filled
            const dist = Math.abs(slots[i].time.getTime() - ts);
            if (dist < bestDist) {
                bestDist = dist;
                bestIdx = i;
            }
        }
        if (bestIdx !== -1 && bestDist <= tolerance) {
            slots[bestIdx].point = point;
        }
    }

    return slots;
}
