import type { Incident } from "@/lib/store";

const FINISHED_STATUSES = new Set(["completed", "resolved"]);

export function isMaintenanceActive(incident: Incident, now = new Date()) {
  if (incident.type !== "maintenance" || FINISHED_STATUSES.has(incident.status)) return false;

  const start = new Date(incident.startTime);
  const end = incident.endTime ? new Date(incident.endTime) : null;
  return start <= now && (!end || end > now);
}

export function isMaintenanceFinished(incident: Incident, now = new Date()) {
  if (FINISHED_STATUSES.has(incident.status)) return true;
  return Boolean(incident.endTime && new Date(incident.endTime) <= now);
}

export function getMaintenanceState(incidents: Incident[], now = new Date()) {
  const maintenanceIncidents = incidents.filter(
    (incident) => incident.type === "maintenance" && !isMaintenanceFinished(incident, now),
  );
  const activeMaintenance = maintenanceIncidents.filter((incident) => isMaintenanceActive(incident, now));
  const maintenanceGroupIds = new Set<string>();
  activeMaintenance.forEach((incident) => {
    incident.affectedGroups?.forEach((groupId) => maintenanceGroupIds.add(groupId));
  });

  return { maintenanceIncidents, activeMaintenance, maintenanceGroupIds };
}

function zonedParts(date: Date, timeZone: string) {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hourCycle: "h23",
  }).formatToParts(date);

  return Object.fromEntries(parts.map(({ type, value }) => [type, value]));
}

export function formatDateTimeLocal(date: string, timeZone: string) {
  const parts = zonedParts(new Date(date), timeZone);
  return `${parts.year}-${parts.month}-${parts.day}T${parts.hour}:${parts.minute}`;
}

export function zonedDateTimeToISOString(localDateTime: string, timeZone: string) {
  const [datePart, timePart] = localDateTime.split("T");
  const [year, month, day] = datePart.split("-").map(Number);
  const [hour, minute, second = 0] = timePart.split(":").map(Number);
  const desiredAsUTC = Date.UTC(year, month - 1, day, hour, minute, second);

  let candidate = new Date(desiredAsUTC);
  for (let i = 0; i < 2; i += 1) {
    const actual = zonedParts(candidate, timeZone);
    const actualAsUTC = Date.UTC(
      Number(actual.year),
      Number(actual.month) - 1,
      Number(actual.day),
      Number(actual.hour),
      Number(actual.minute),
      Number(actual.second),
    );
    candidate = new Date(candidate.getTime() + desiredAsUTC - actualAsUTC);
  }

  return candidate.toISOString();
}
