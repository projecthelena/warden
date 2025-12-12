import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export type Environment = "production" | "preprod" | "development" | "system" | "unknown";

export const HOURS_PER_DAY = 24;
export const HOURS_PER_MONTH = HOURS_PER_DAY * 30;
export const HOURS_PER_YEAR = HOURS_PER_DAY * 365;

export const toDailyCost = (hourlyCost: number) => hourlyCost * HOURS_PER_DAY;
export const toMonthlyCost = (hourlyCost: number) => hourlyCost * HOURS_PER_MONTH;
export const toYearlyCost = (hourlyCost: number) => hourlyCost * HOURS_PER_YEAR;

export const formatCurrency = (value: number, options?: Intl.NumberFormatOptions) =>
  new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
    ...options
  }).format(value);

export const formatPercentage = (value: number, options?: { fractionDigits?: number }) => {
  const digits = options?.fractionDigits ?? 0;
  return `${value.toFixed(digits)}%`;
};

export const formatNumber = (value: number) =>
  new Intl.NumberFormat("en-US", { notation: "compact", maximumFractionDigits: 1 }).format(value);

export const milliToCores = (value: number) => value / 1000;
export const bytesToGiB = (value: number) => value / 1024 / 1024 / 1024;

export function formatBytes(bytes: number, decimals = 2) {
  if (!+bytes) return '0 Bytes';
  const k = 1024;
  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(dm))} ${sizes[i]}`;
}

export const computeWastePercent = (requested: number, used: number) => {
  if (!requested || requested <= 0) return 0;
  const diff = requested - used;
  if (diff <= 0) return 0;
  return Math.min(100, (diff / requested) * 100);
};

export const computeEfficiencyPercent = (usage: number, requested: number) => {
  if (!requested || requested <= 0) return 0;
  return Math.min(100, (usage / requested) * 100);
};

export const environmentLabels: Record<Environment, string> = {
  production: "Production",
  preprod: "Preprod",
  development: "Development",
  system: "System",
  unknown: "Unknown"
};

export const environmentStyle: Record<Environment, string> = {
  production: "border-emerald-500/30 bg-emerald-500/10 text-emerald-300",
  preprod: "border-sky-500/30 bg-sky-500/10 text-sky-300",
  development: "border-amber-500/30 bg-amber-500/10 text-amber-300",
  system: "border-slate-500/40 bg-slate-700/30 text-slate-100",
  unknown: "border-muted bg-muted/40 text-muted-foreground"
};

export const getEnvironmentGroup = (env: Environment | "mixed") => {
  if (env === "system") return "System";
  if (env === "production") return "Production";
  if (env === "mixed") return "Mixed";
  return "Non-prod";
};

export const relativeTimeFromIso = (iso: string) => {
  if (!iso) return "";
  const formatter = new Intl.RelativeTimeFormat("en", { numeric: "auto" });
  const value = new Date(iso).getTime();
  const diff = value - Date.now();
  const minutes = Math.round(diff / 1000 / 60);
  if (Math.abs(minutes) < 60) return formatter.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return formatter.format(hours, "hour");
  const days = Math.round(hours / 24);
  return formatter.format(days, "day");
};
