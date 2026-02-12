import { describe, it, expect } from "vitest";
import { computePollingInterval } from "./pollingInterval";
import { Group } from "@/lib/store";

function makeMonitor(overrides: Partial<Group["monitors"][0]> = {}): Group["monitors"][0] {
  return {
    id: "m-1",
    name: "Test",
    url: "https://example.com",
    status: "up",
    active: true,
    latency: 100,
    history: [],
    lastCheck: new Date().toISOString(),
    events: [],
    interval: 60,
    ...overrides,
  };
}

function makeGroup(monitors: Group["monitors"]): Group {
  return { id: "g-1", name: "Default", monitors };
}

describe("computePollingInterval", () => {
  it("returns 30000 for empty groups", () => {
    expect(computePollingInterval([])).toBe(30_000);
  });

  it("returns interval * 1000 for a single active monitor", () => {
    const groups = [makeGroup([makeMonitor({ interval: 60 })])];
    expect(computePollingInterval(groups)).toBe(60_000);
  });

  it("uses the shortest active monitor interval", () => {
    const groups = [
      makeGroup([
        makeMonitor({ id: "m-1", interval: 60 }),
        makeMonitor({ id: "m-2", interval: 10 }),
        makeMonitor({ id: "m-3", interval: 30 }),
      ]),
    ];
    expect(computePollingInterval(groups)).toBe(10_000);
  });

  it("returns 30000 when all monitors are paused", () => {
    const groups = [
      makeGroup([
        makeMonitor({ status: "paused", interval: 10 }),
        makeMonitor({ status: "paused", interval: 20 }),
      ]),
    ];
    expect(computePollingInterval(groups)).toBe(30_000);
  });

  it("ignores inactive monitors", () => {
    const groups = [
      makeGroup([
        makeMonitor({ active: false, interval: 5 }),
        makeMonitor({ active: true, interval: 30 }),
      ]),
    ];
    expect(computePollingInterval(groups)).toBe(30_000);
  });

  it("caps at 60000 for very long intervals", () => {
    const groups = [makeGroup([makeMonitor({ interval: 600 })])];
    expect(computePollingInterval(groups)).toBe(60_000);
  });

  it("floors at 5000 for very short intervals", () => {
    const groups = [makeGroup([makeMonitor({ interval: 2 })])];
    expect(computePollingInterval(groups)).toBe(5_000);
  });

  it("works across multiple groups", () => {
    const groups = [
      makeGroup([makeMonitor({ id: "m-1", interval: 60 })]),
      { id: "g-2", name: "Other", monitors: [makeMonitor({ id: "m-2", interval: 15 })] },
    ];
    expect(computePollingInterval(groups)).toBe(15_000);
  });
});
