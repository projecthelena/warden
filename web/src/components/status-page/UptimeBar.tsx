import { useMemo, useState, useCallback } from "react";
import { createPortal } from "react-dom";

interface DayData {
    date: string;
    uptimePercent: number;
    totalChecks: number;
}

interface UptimeBarProps {
    days: DayData[];
    overallUptime: number;
}

function getBarColor(uptimePercent: number): string {
    if (uptimePercent < 0) return "bg-muted/40"; // no data
    if (uptimePercent >= 100) return "bg-emerald-500";
    if (uptimePercent >= 99) return "bg-emerald-400";
    if (uptimePercent >= 95) return "bg-yellow-500";
    if (uptimePercent >= 90) return "bg-orange-500";
    return "bg-red-500";
}

function formatDateLabel(dateStr: string): string {
    const d = new Date(dateStr + "T00:00:00");
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
}

function formatUptime(pct: number): string {
    if (pct < 0) return "No data";
    if (pct >= 100) return "100%";
    if (pct >= 99.99) return pct.toFixed(2) + "%";
    return pct.toFixed(2) + "%";
}

export function UptimeBar({ days, overallUptime }: UptimeBarProps) {
    const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
    const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number; align: 'left' | 'center' | 'right' } | null>(null);

    // Use all 90 days — CSS handles responsive sizing
    const visibleDays = days;

    const handleMouseEnter = useCallback((index: number, e: React.MouseEvent<HTMLDivElement>) => {
        setHoveredIndex(index);
        const barRect = e.currentTarget.getBoundingClientRect();

        const TOOLTIP_WIDTH = 180;
        const HALF_TOOLTIP = TOOLTIP_WIDTH / 2;

        let x = barRect.left + barRect.width / 2;  // screen coordinates
        let align: 'left' | 'center' | 'right' = 'center';

        // Clamp to viewport edges
        if (x < HALF_TOOLTIP + 8) {
            x = barRect.left;
            align = 'left';
        } else if (x > window.innerWidth - HALF_TOOLTIP - 8) {
            x = barRect.right;
            align = 'right';
        }

        setTooltipPos({
            x,
            y: barRect.top,  // screen Y coordinate
            align,
        });
    }, []);

    const handleMouseLeave = useCallback(() => {
        setHoveredIndex(null);
        setTooltipPos(null);
    }, []);

    const uptimeDisplay = useMemo(() => {
        if (overallUptime >= 100) return "100%";
        if (overallUptime >= 99.99) return overallUptime.toFixed(2) + "%";
        return overallUptime.toFixed(2) + "%";
    }, [overallUptime]);

    const uptimeColor = useMemo(() => {
        if (overallUptime >= 99.9) return "text-emerald-500";
        if (overallUptime >= 99) return "text-emerald-400";
        if (overallUptime >= 95) return "text-yellow-500";
        if (overallUptime >= 90) return "text-orange-500";
        return "text-red-500";
    }, [overallUptime]);

    return (
        <div className="relative flex items-center gap-3 w-full">
            {/* Bars container */}
            <div className="flex-1 flex items-end gap-[1px] h-8 min-w-0">
                {visibleDays.map((day, i) => (
                    <div
                        key={day.date}
                        className={`flex-1 min-w-0 rounded-[1px] transition-all duration-150 cursor-default ${getBarColor(day.uptimePercent)} ${
                            hoveredIndex === i ? "opacity-80 scale-y-110" : "opacity-100"
                        }`}
                        style={{ height: "100%" }}
                        onMouseEnter={(e) => handleMouseEnter(i, e)}
                        onMouseLeave={handleMouseLeave}
                    />
                ))}
            </div>

            {/* Overall uptime */}
            <div className={`text-sm font-semibold tabular-nums whitespace-nowrap ${uptimeColor}`}>
                {uptimeDisplay}
            </div>

            {/* Tooltip - rendered via portal to escape overflow-hidden containers */}
            {hoveredIndex !== null && tooltipPos && createPortal(
                <div
                    className="fixed z-[9999] pointer-events-none"
                    style={{
                        left: tooltipPos.x,
                        top: tooltipPos.y - 8,
                        transform: tooltipPos.align === 'left'
                            ? 'translateY(-100%)'
                            : tooltipPos.align === 'right'
                                ? 'translate(-100%, -100%)'
                                : 'translate(-50%, -100%)',
                    }}
                >
                    <div className="bg-popover border border-border rounded-md px-3 py-2 shadow-lg text-xs whitespace-nowrap">
                        <div className="font-medium text-foreground">
                            {formatDateLabel(visibleDays[hoveredIndex].date)}
                        </div>
                        <div className="text-muted-foreground">
                            {formatUptime(visibleDays[hoveredIndex].uptimePercent)}
                            {visibleDays[hoveredIndex].totalChecks > 0 && (
                                <span className="ml-1.5 opacity-70">
                                    ({visibleDays[hoveredIndex].totalChecks.toLocaleString()} checks)
                                </span>
                            )}
                        </div>
                    </div>
                </div>,
                document.body
            )}
        </div>
    );
}
