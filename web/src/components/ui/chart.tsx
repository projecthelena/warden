/* eslint-disable react-refresh/only-export-components */
import * as React from "react";
import type { LegendProps, TooltipProps } from "recharts";
import { Tooltip as RechartsTooltip } from "recharts";
import { cn } from "@/lib/utils";

export type ChartConfig = Record<
  string,
  {
    label: string;
    color?: string;
  }
>;

const ChartConfigContext = React.createContext<ChartConfig>({});

export interface ChartContainerProps extends React.HTMLAttributes<HTMLDivElement> {
  config: ChartConfig;
}

export const ChartContainer = React.forwardRef<HTMLDivElement, ChartContainerProps>(
  ({ config, className, children, ...props }, ref) => (
    <ChartConfigContext.Provider value={config}>
      <div ref={ref} className={cn("relative flex w-full flex-1", className)} {...props}>
        {children}
      </div>
    </ChartConfigContext.Provider>
  )
);
ChartContainer.displayName = "ChartContainer";

export const useChartConfig = () => React.useContext(ChartConfigContext);

export interface ChartTooltipProps extends TooltipProps<number, string> { }

export const ChartTooltip = ({ content, ...props }: ChartTooltipProps) => (
  <RechartsTooltip content={content ?? <ChartTooltipContent />} {...props} />
);

export interface ChartTooltipContentProps extends TooltipProps<number, string> {
  formatter?: (value: number | string) => React.ReactNode;
  hideLabel?: boolean;
}

export const ChartTooltipContent = ({ active, payload, label, formatter, hideLabel }: ChartTooltipContentProps) => {
  const config = useChartConfig();
  if (!active || !payload?.length) return null;
  const datum = payload[0];
  const key = String(datum.name ?? datum.dataKey ?? "");
  const color = datum.color ?? config[key]?.color;
  const resolvedLabel = config[key]?.label ?? label ?? key;
  const value = datum.value ?? 0;

  return (
    <div className="rounded-md border border-border bg-background px-3 py-2 text-xs shadow-lg">
      {!hideLabel && resolvedLabel && <div className="mb-1 font-medium text-foreground">{resolvedLabel}</div>}
      <div className="flex items-center gap-2 text-muted-foreground">
        {color && <span className="h-2 w-2 rounded-full" style={{ backgroundColor: color }} />}
        <span>{formatter ? formatter(value) : value}</span>
      </div>
    </div>
  );
};

export interface ChartLegendContentProps extends Pick<LegendProps, "payload"> { }

export const ChartLegendContent = ({ payload }: ChartLegendContentProps) => {
  const config = useChartConfig();
  if (!payload?.length) return null;
  return (
    <div className="flex flex-wrap items-center justify-center gap-4 text-xs text-muted-foreground">
      {payload.map((entry) => {
        const key = String(entry.value);
        const color = config[key]?.color ?? entry.color;
        const label = config[key]?.label ?? entry.value;
        return (
          <div key={key} className="flex items-center gap-2">
            <span className="h-2 w-2 rounded-full" style={{ backgroundColor: color }} />
            <span>{label}</span>
          </div>
        );
      })}
    </div>
  );
};
