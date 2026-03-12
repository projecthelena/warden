import { useMemo, useState, useCallback, useRef, useEffect } from "react";
import { createPortal } from "react-dom";

interface DayData {
    date: string;
    uptimePercent: number;
    totalChecks: number;
}

interface BarBucket {
    startDate: string;
    endDate: string;
    dayCount: number;
    uptimePercent: number;
    totalChecks: number;
    hasData: boolean;
}

interface UptimeBarProps {
    days: DayData[];
    overallUptime: number;
    showPercentage?: boolean;
}

function aggregateDays(days: DayData[], targetBars: number): BarBucket[] {
    if (days.length <= targetBars) {
        return days.map((d) => ({
            startDate: d.date,
            endDate: d.date,
            dayCount: 1,
            uptimePercent: d.uptimePercent,
            totalChecks: d.totalChecks,
            hasData: d.uptimePercent >= 0,
        }));
    }

    const bucketSize = Math.ceil(days.length / targetBars);
    const buckets: BarBucket[] = [];

    for (let i = 0; i < days.length; i += bucketSize) {
        const chunk = days.slice(i, i + bucketSize);
        const withData = chunk.filter((d) => d.uptimePercent >= 0);
        const hasData = withData.length > 0;

        buckets.push({
            startDate: chunk[0].date,
            endDate: chunk[chunk.length - 1].date,
            dayCount: chunk.length,
            uptimePercent: hasData ? Math.min(...withData.map((d) => d.uptimePercent)) : -1,
            totalChecks: chunk.reduce((sum, d) => sum + d.totalChecks, 0),
            hasData,
        });
    }

    return buckets;
}

function getBarColor(uptimePercent: number): string {
    if (uptimePercent < 0) return ""; // no data — handled by inline style
    if (uptimePercent >= 100) return "bg-emerald-500";
    if (uptimePercent >= 99) return "bg-emerald-400";
    if (uptimePercent >= 95) return "bg-yellow-500";
    if (uptimePercent >= 90) return "bg-orange-500";
    return "bg-red-500";
}

function getDotColor(uptimePercent: number): string {
    if (uptimePercent < 0) return "bg-muted";
    if (uptimePercent >= 100) return "bg-emerald-500";
    if (uptimePercent >= 99) return "bg-emerald-400";
    if (uptimePercent >= 95) return "bg-yellow-500";
    if (uptimePercent >= 90) return "bg-orange-500";
    return "bg-red-500";
}

function formatDateLabel(dateStr: string): string {
    const d = new Date(dateStr + "T00:00:00");
    return d.toLocaleDateString(undefined, { weekday: "short", month: "short", day: "numeric", year: "numeric" });
}

function formatDateRange(startDate: string, endDate: string): string {
    const s = new Date(startDate + "T00:00:00");
    const e = new Date(endDate + "T00:00:00");
    const sYear = s.getFullYear();
    const eYear = e.getFullYear();
    const sMonth = s.toLocaleDateString(undefined, { month: "short" });
    const eMonth = e.toLocaleDateString(undefined, { month: "short" });

    if (sMonth === eMonth && sYear === eYear) {
        return `${sMonth} ${s.getDate()}\u2013${e.getDate()}, ${sYear}`;
    }
    return `${sMonth} ${s.getDate()} \u2013 ${eMonth} ${e.getDate()}, ${eYear}`;
}

function formatUptime(pct: number): string {
    if (pct < 0) return "No data";
    if (pct >= 100) return "100%";
    return pct.toFixed(2) + "%";
}

function formatDowntime(uptimePercent: number, dayCount: number): string | null {
    if (uptimePercent < 0 || uptimePercent >= 100) return null;
    const totalMinutes = (1 - uptimePercent / 100) * dayCount * 24 * 60;
    if (totalMinutes < 1) return null;
    const roundedMinutes = Math.round(totalMinutes);
    const hours = Math.floor(roundedMinutes / 60);
    const mins = roundedMinutes % 60;
    if (hours > 0) return `~${hours}h ${mins}m downtime`;
    return `~${mins}m downtime`;
}

const NO_DATA_PATTERN =
    "repeating-linear-gradient(-45deg, hsl(var(--muted) / 0.15), hsl(var(--muted) / 0.15) 2px, transparent 2px, transparent 5px)";

export function UptimeBar({ days, overallUptime, showPercentage = true }: UptimeBarProps) {
    const [hoveredIndex, setHoveredIndex] = useState<number | null>(null);
    const [tooltipVisible, setTooltipVisible] = useState(false);
    const [tooltipPos, setTooltipPos] = useState<{ x: number; y: number; align: "left" | "center" | "right" } | null>(null);
    const hoverTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
    const containerRef = useRef<HTMLDivElement>(null);
    const [containerWidth, setContainerWidth] = useState(640);
    const isTouchRef = useRef(false);
    const [touchActiveIndex, setTouchActiveIndex] = useState<number | null>(null);

    // Responsive bar count via ResizeObserver
    useEffect(() => {
        const el = containerRef.current;
        if (!el) return;
        const observer = new ResizeObserver((entries) => {
            for (const entry of entries) {
                setContainerWidth(entry.contentRect.width);
            }
        });
        observer.observe(el);
        return () => observer.disconnect();
    }, []);

    // Dismiss touch tooltip on tap-away
    useEffect(() => {
        if (touchActiveIndex === null) return;
        const handler = (e: MouseEvent) => {
            if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
                setTouchActiveIndex(null);
                setHoveredIndex(null);
                setTooltipVisible(false);
            }
        };
        document.addEventListener("click", handler);
        return () => document.removeEventListener("click", handler);
    }, [touchActiveIndex]);

    const targetBars = containerWidth >= 640 ? 45 : containerWidth >= 400 ? 30 : 20;

    const buckets = useMemo(() => aggregateDays(days, targetBars), [days, targetBars]);
    const barGap = "gap-[2px]";

    const showTooltip = useCallback((index: number, rect: DOMRect) => {
        setHoveredIndex(index);

        const TOOLTIP_WIDTH = 220;
        const HALF_TOOLTIP = TOOLTIP_WIDTH / 2;

        let x = rect.left + rect.width / 2;
        let align: "left" | "center" | "right" = "center";

        if (x < HALF_TOOLTIP + 8) {
            x = rect.left;
            align = "left";
        } else if (x > window.innerWidth - HALF_TOOLTIP - 8) {
            x = rect.right;
            align = "right";
        }

        setTooltipPos({ x, y: rect.top, align });
        setTooltipVisible(true);
    }, []);

    const handleMouseEnter = useCallback(
        (index: number, e: React.MouseEvent<HTMLDivElement>) => {
            if (isTouchRef.current) return;
            // Capture rect synchronously — e.currentTarget is nullified after handler returns
            const rect = e.currentTarget.getBoundingClientRect();
            if (hoverTimeoutRef.current) clearTimeout(hoverTimeoutRef.current);
            hoverTimeoutRef.current = setTimeout(() => {
                showTooltip(index, rect);
            }, 75);
        },
        [showTooltip]
    );

    const handleMouseLeave = useCallback(() => {
        if (isTouchRef.current) return;
        if (hoverTimeoutRef.current) {
            clearTimeout(hoverTimeoutRef.current);
            hoverTimeoutRef.current = null;
        }
        setHoveredIndex(null);
        setTooltipVisible(false);
    }, []);

    const handleClick = useCallback(
        (index: number, e: React.MouseEvent<HTMLDivElement>) => {
            if (!isTouchRef.current) return;
            e.stopPropagation();
            if (touchActiveIndex === index) {
                setTouchActiveIndex(null);
                setHoveredIndex(null);
                setTooltipVisible(false);
            } else {
                setTouchActiveIndex(index);
                showTooltip(index, e.currentTarget.getBoundingClientRect());
            }
        },
        [touchActiveIndex, showTooltip]
    );

    // Detect touch device
    useEffect(() => {
        const handler = () => {
            isTouchRef.current = true;
        };
        window.addEventListener("touchstart", handler, { once: true, passive: true });
        return () => window.removeEventListener("touchstart", handler);
    }, []);

    const uptimeDisplay = useMemo(() => {
        if (overallUptime >= 100) return "100%";
        return overallUptime.toFixed(2) + "%";
    }, [overallUptime]);

    const uptimeColor = useMemo(() => {
        if (overallUptime >= 99.9) return "text-emerald-500";
        if (overallUptime >= 99) return "text-emerald-400";
        if (overallUptime >= 95) return "text-yellow-500";
        if (overallUptime >= 90) return "text-orange-500";
        return "text-red-500";
    }, [overallUptime]);

    const hoveredBucket = hoveredIndex !== null ? buckets[hoveredIndex] : null;

    return (
        <div className="relative w-full">
            {/* Bars + percentage row */}
            <div className="flex items-center gap-3">
                {/* Bars container */}
                <div
                    ref={containerRef}
                    className={`flex-1 flex items-end ${barGap} h-8 min-w-0`}
                    role="img"
                    aria-label={`Uptime over last ${days.length} days: ${uptimeDisplay}`}
                >
                    {buckets.map((bucket, i) => {
                        const isFirst = i === 0;
                        const isLast = i === buckets.length - 1;
                        const noData = !bucket.hasData;
                        const barHeight =
                            bucket.uptimePercent < 0
                                ? "100%"
                                : `${Math.max(40, bucket.uptimePercent)}%`;

                        const roundedClass = isFirst
                            ? "rounded-l"
                            : isLast
                                ? "rounded-r"
                                : "rounded-[1px]";

                        return (
                            <div
                                key={`${bucket.startDate}-${bucket.endDate}`}
                                className={`flex-1 min-w-0 transition-all duration-150 cursor-default border border-foreground/[0.08] ${
                                    noData ? "" : getBarColor(bucket.uptimePercent)
                                } ${roundedClass} ${
                                    hoveredIndex !== null
                                        ? hoveredIndex === i
                                            ? "!border-foreground/25"
                                            : "opacity-40"
                                        : ""
                                }`}
                                style={{
                                    height: barHeight,
                                    ...(noData ? { background: NO_DATA_PATTERN } : {}),
                                }}
                                onMouseEnter={(e) => handleMouseEnter(i, e)}
                                onMouseLeave={handleMouseLeave}
                                onClick={(e) => handleClick(i, e)}
                            />
                        );
                    })}
                </div>

                {/* Overall uptime */}
                {showPercentage && (
                    <div className={`text-sm font-mono font-bold tabular-nums whitespace-nowrap ${uptimeColor}`}>
                        {uptimeDisplay}
                    </div>
                )}
            </div>

            {/* Tooltip - rendered via portal */}
            {hoveredBucket &&
                tooltipPos &&
                createPortal(
                    <div
                        className={`fixed z-[9999] pointer-events-none transition-opacity duration-150 ${
                            tooltipVisible ? "opacity-100" : "opacity-0"
                        }`}
                        style={{
                            left: tooltipPos.x,
                            top: tooltipPos.y - 8,
                            transform:
                                tooltipPos.align === "left"
                                    ? "translateY(-100%)"
                                    : tooltipPos.align === "right"
                                        ? "translate(-100%, -100%)"
                                        : "translate(-50%, -100%)",
                        }}
                    >
                        <div className="bg-popover border border-border rounded-lg px-3 py-2 shadow-xl backdrop-blur-sm text-xs whitespace-nowrap">
                            <div className="font-medium text-foreground">
                                {hoveredBucket.dayCount > 1
                                    ? formatDateRange(hoveredBucket.startDate, hoveredBucket.endDate)
                                    : formatDateLabel(hoveredBucket.startDate)}
                            </div>
                            <div className="flex items-center gap-1.5 text-muted-foreground mt-0.5">
                                <span
                                    className={`inline-block w-2 h-2 rounded-full ${getDotColor(hoveredBucket.uptimePercent)}`}
                                />
                                {formatUptime(hoveredBucket.uptimePercent)}
                                {hoveredBucket.dayCount > 1 && (
                                    <span className="ml-0.5 opacity-70">
                                        ({hoveredBucket.dayCount} days)
                                    </span>
                                )}
                                {hoveredBucket.totalChecks > 0 && hoveredBucket.dayCount === 1 && (
                                    <span className="ml-1 opacity-70">
                                        ({hoveredBucket.totalChecks.toLocaleString()} checks)
                                    </span>
                                )}
                            </div>
                            {(() => {
                                const dt = formatDowntime(hoveredBucket.uptimePercent, hoveredBucket.dayCount);
                                return dt ? (
                                    <div className="text-muted-foreground/70 mt-0.5">{dt}</div>
                                ) : null;
                            })()}
                        </div>
                    </div>,
                    document.body
                )}
        </div>
    );
}
