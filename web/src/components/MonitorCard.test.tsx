import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { Incident, Monitor } from "@/lib/store";
import { MonitorCard } from "./MonitorCard";
import { TooltipProvider } from "@/components/ui/tooltip";

let incidents: Incident[] = [];

vi.mock("@/lib/store", async (importOriginal) => {
  const original = await importOriginal<typeof import("@/lib/store")>();
  return {
    ...original,
    useMonitorStore: () => ({ user: { timezone: "America/Bogota" }, incidents }),
  };
});

const monitor: Monitor = {
  id: "monitor-1",
  name: "Router",
  type: "ping",
  url: "192.0.2.1",
  status: "up",
  active: true,
  latency: 2,
  history: [],
  lastCheck: "2026-09-15T22:15:00Z",
  events: [],
  interval: 60,
};

const windowAt = (startTime: string, endTime: string): Incident => ({
  id: "maintenance-1",
  title: "Router restart",
  description: "",
  type: "maintenance",
  severity: "minor",
  status: "scheduled",
  startTime,
  endTime,
  affectedGroups: ["network"],
});

function renderCard() {
  return render(
    <MemoryRouter>
      <TooltipProvider>
        <MonitorCard monitor={monitor} groupId="network" />
      </TooltipProvider>
    </MemoryRouter>,
  );
}

describe("MonitorCard maintenance state", () => {
  afterEach(() => vi.useRealTimers());

  it("shows maintenance during the window", () => {
    vi.useFakeTimers();
    vi.setSystemTime("2026-09-15T22:15:00Z");
    incidents = [windowAt("2026-09-15T22:13:00Z", "2026-09-15T22:18:00Z")];

    renderCard();

    expect(screen.getByText("Maintenance")).toBeInTheDocument();
  });

  it("returns to the monitor status when a scheduled window expires", () => {
    vi.useFakeTimers();
    vi.setSystemTime("2026-09-15T22:19:00Z");
    incidents = [windowAt("2026-09-15T22:13:00Z", "2026-09-15T22:18:00Z")];

    renderCard();

    expect(screen.queryByText("Maintenance")).not.toBeInTheDocument();
    expect(screen.getByText("Operational")).toBeInTheDocument();
  });
});
