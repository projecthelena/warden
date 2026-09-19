/* Hallmark · component: maintenance scheduler · genre: modern-minimal · theme: existing Warden tokens
 * Hallmark · pre-emit critique: P5 H4 E4 S5 R5 V4 · contrast: pass · responsive: pass
 */
import { useState } from "react";
import { CalendarClock, ChevronDownIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Calendar } from "@/components/ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import { Sheet, SheetContent, SheetDescription, SheetFooter, SheetHeader, SheetTitle, SheetTrigger } from "@/components/ui/sheet";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Incident, useMonitorStore } from "@/lib/store";
import { formatDateTimeLocal, zonedDateTimeToISOString } from "@/lib/maintenance";
import { useToast } from "@/components/ui/use-toast";

const HOURS = Array.from({ length: 12 }, (_, index) => String(index + 1));
const MINUTES = Array.from({ length: 60 }, (_, index) => String(index).padStart(2, "0"));

function parseTime(value: string) {
    const [rawHour = "10", minute = "00"] = value.split(":");
    const hour24 = Number(rawHour);
    return {
        hour: String(hour24 % 12 || 12),
        minute,
        period: hour24 >= 12 ? "PM" : "AM",
    };
}

function buildTime(hour: string, minute: string, period: string) {
    const hour12 = Number(hour);
    const hour24 = period === "PM" ? (hour12 % 12) + 12 : hour12 % 12;
    return `${String(hour24).padStart(2, "0")}:${minute}:00`;
}

function defaultLocalParts(timezone: string, offsetMinutes: number) {
    const instant = new Date(Date.now() + offsetMinutes * 60_000);
    instant.setUTCMinutes(Math.ceil(instant.getUTCMinutes() / 5) * 5, 0, 0);
    const [datePart, timePart] = formatDateTimeLocal(instant.toISOString(), timezone).split("T");
    const [year, month, day] = datePart.split("-").map(Number);
    return { date: new Date(year, month - 1, day), time: `${timePart}:00` };
}

function TimePicker({ value, onChange, testId }: { value: string; onChange: (value: string) => void; testId: string }) {
    const { hour, minute, period } = parseTime(value);
    const update = (nextHour = hour, nextMinute = minute, nextPeriod = period) => {
        onChange(buildTime(nextHour, nextMinute, nextPeriod));
    };

    return (
        <div className="grid grid-cols-[1fr_1fr_1fr] gap-2" data-testid={testId}>
            <Select value={hour} onValueChange={(next) => update(next)}>
                <SelectTrigger aria-label="Hour">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    {HOURS.map((item) => (
                        <SelectItem key={item} value={item}>
                            {item}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            <Select value={minute} onValueChange={(next) => update(hour, next)}>
                <SelectTrigger aria-label="Minute">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent className="max-h-64">
                    {MINUTES.map((item) => (
                        <SelectItem key={item} value={item}>
                            {item}
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>
            <Select value={period} onValueChange={(next) => update(hour, minute, next)}>
                <SelectTrigger aria-label="AM or PM">
                    <SelectValue />
                </SelectTrigger>
                <SelectContent>
                    <SelectItem value="AM">AM</SelectItem>
                    <SelectItem value="PM">PM</SelectItem>
                </SelectContent>
            </Select>
        </div>
    );
}

interface CreateMaintenanceSheetProps {
    onCreate: (incident: Omit<Incident, "id">) => void;
    groups: { id: string; name: string }[];
}

export function CreateMaintenanceSheet({ onCreate, groups }: CreateMaintenanceSheetProps) {
    const { toast } = useToast();
    const timezone = useMonitorStore((state) => state.user?.timezone || "UTC");
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [selectedGroupId, setSelectedGroupId] = useState<string>("");

    const initialStart = defaultLocalParts(timezone, 0);
    const initialEnd = defaultLocalParts(timezone, 60);
    const [startDate, setStartDate] = useState<Date>(initialStart.date);
    const [startTime, setStartTime] = useState(initialStart.time);
    const [endDate, setEndDate] = useState<Date>(initialEnd.date);
    const [endTime, setEndTime] = useState(initialEnd.time);

    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!selectedGroupId) {
            toast({
                title: "Affected group required",
                description: "Choose the group covered by this maintenance.",
                variant: "destructive",
            });
            return;
        }

        if (!startDate || !startTime || !endDate || !endTime) {
            toast({
                title: "Date and time required",
                description: "Choose when the maintenance starts and ends.",
                variant: "destructive",
            });
            return;
        }

        const localValue = (date: Date, time: string) => `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, "0")}-${String(date.getDate()).padStart(2, "0")}T${time}`;

        const start = zonedDateTimeToISOString(localValue(startDate, startTime), timezone);
        const end = zonedDateTimeToISOString(localValue(endDate, endTime), timezone);

        if (new Date(end) <= new Date(start)) {
            toast({
                title: "Invalid time range",
                description: "End time must be after start time.",
                variant: "destructive",
            });
            return;
        }

        onCreate({
            title,
            description,
            type: "maintenance",
            severity: "minor",
            status: "scheduled",
            startTime: start,
            endTime: end,
            affectedGroups: [selectedGroupId],
        });

        setOpen(false);
        resetForm();
    };

    const resetForm = () => {
        const nextStart = defaultLocalParts(timezone, 0);
        const nextEnd = defaultLocalParts(timezone, 60);
        setTitle("");
        setDescription("");
        setSelectedGroupId("");
        setStartDate(nextStart.date);
        setStartTime(nextStart.time);
        setEndDate(nextEnd.date);
        setEndTime(nextEnd.time);
    };

    // Helper for DateTime Row tailored to user request
    const DateTimeRow = ({ label, date, setDate, time, setTime }: { label: string; date: Date | undefined; setDate: (d: Date | undefined) => void; time: string; setTime: (t: string) => void }) => {
        const [popoverOpen, setPopoverOpen] = useState(false);

        return (
            <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.15fr)]">
                <div className="flex min-w-0 flex-col gap-3">
                    <Label className="px-1 text-xs text-muted-foreground font-medium">{label} Date</Label>
                    <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
                        <PopoverTrigger asChild>
                            <Button variant="outline" className={cn("w-full justify-between font-normal", !date && "text-muted-foreground")}>
                                {date ? date.toLocaleDateString() : "Select date"}
                                <ChevronDownIcon className="h-4 w-4 opacity-50" />
                            </Button>
                        </PopoverTrigger>
                        <PopoverContent className="w-auto overflow-hidden p-0" align="start">
                            <Calendar
                                mode="single"
                                selected={date}
                                captionLayout="dropdown"
                                onSelect={(d) => {
                                    setDate(d);
                                    setPopoverOpen(false);
                                }}
                            />
                            <div className="flex gap-2 border-t p-3">
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    className="flex-1"
                                    onClick={() => {
                                        setDate(new Date());
                                        setPopoverOpen(false);
                                    }}
                                >
                                    Today
                                </Button>
                                <Button
                                    type="button"
                                    variant="outline"
                                    size="sm"
                                    className="flex-1"
                                    onClick={() => {
                                        const tomorrow = new Date();
                                        tomorrow.setDate(tomorrow.getDate() + 1);
                                        setDate(tomorrow);
                                        setPopoverOpen(false);
                                    }}
                                >
                                    Tomorrow
                                </Button>
                            </div>
                        </PopoverContent>
                    </Popover>
                </div>
                <div className="flex min-w-0 flex-col gap-3">
                    <Label className="px-1 text-xs text-muted-foreground font-medium">Time</Label>
                    <TimePicker value={time} onChange={setTime} testId={`${label.toLowerCase().replaceAll(" ", "-")}-picker`} />
                </div>
            </div>
        );
    };

    return (
        <Sheet
            open={open}
            onOpenChange={(val) => {
                setOpen(val);
                if (!val) resetForm();
            }}
        >
            <SheetTrigger asChild>
                <Button size="sm" className="gap-2" data-testid="create-maintenance-trigger">
                    <CalendarClock className="w-4 h-4" /> Schedule Maintenance
                </Button>
            </SheetTrigger>
            <SheetContent className="sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle>Schedule Maintenance</SheetTitle>
                    <SheetDescription>Plan a maintenance window for a specific group.</SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label>Title</Label>
                        <Input value={title} onChange={(e) => setTitle(e.target.value)} required placeholder="e.g. Database Upgrade" data-testid="maintenance-title-input" />
                    </div>

                    <div className="grid gap-2">
                        <Label>Affected Group</Label>
                        <Select value={selectedGroupId} onValueChange={setSelectedGroupId}>
                            <SelectTrigger data-testid="maintenance-group-select">
                                <SelectValue placeholder="Select Group" />
                            </SelectTrigger>
                            <SelectContent>
                                {groups.map((g) => (
                                    <SelectItem key={g.id} value={g.id}>
                                        {g.name}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-4">
                        <DateTimeRow label="Start Time" date={startDate} setDate={setStartDate} time={startTime} setTime={setStartTime} />
                        <DateTimeRow label="End Time" date={endDate} setDate={setEndDate} time={endTime} setTime={setEndTime} />
                        <div className="text-xs text-muted-foreground text-right px-1">Time Zone: {timezone}</div>
                    </div>

                    <div className="grid gap-2">
                        <Label>Description</Label>
                        <Input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="Details about the maintenance..." />
                    </div>

                    <SheetFooter className="mt-4">
                        <Button type="submit" data-testid="create-maintenance-submit">
                            Schedule Maintenance
                        </Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
