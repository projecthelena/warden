import { describe, expect, it } from "vitest";
import type { Incident } from "@/lib/store";
import { formatDateTimeLocal, isMaintenanceActive, isMaintenanceFinished, zonedDateTimeToISOString } from "./maintenance";

const maintenance = (overrides: Partial<Incident> = {}): Incident => ({
  id: "maintenance-1",
  title: "Router restart",
  description: "",
  type: "maintenance",
  severity: "minor",
  status: "scheduled",
  startTime: "2026-09-15T22:13:00Z",
  endTime: "2026-09-15T22:18:00Z",
  affectedGroups: ["network"],
  ...overrides,
});

describe("maintenance lifecycle", () => {
  it("is active only inside its time window", () => {
    expect(isMaintenanceActive(maintenance(), new Date("2026-09-15T22:15:00Z"))).toBe(true);
    expect(isMaintenanceActive(maintenance(), new Date("2026-09-15T22:19:00Z"))).toBe(false);
  });

  it("moves expired scheduled windows into history", () => {
    expect(isMaintenanceFinished(maintenance(), new Date("2026-09-15T22:19:00Z"))).toBe(true);
  });
});

describe("maintenance timezone conversion", () => {
  it("round trips a Bogota wall clock time", () => {
    const iso = zonedDateTimeToISOString("2026-09-15T17:13", "America/Bogota");
    expect(iso).toBe("2026-09-15T22:13:00.000Z");
    expect(formatDateTimeLocal(iso, "America/Bogota")).toBe("2026-09-15T17:13");
  });
});
