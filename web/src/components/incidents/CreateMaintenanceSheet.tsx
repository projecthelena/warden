import { useState } from "react";
import { CalendarClock, ChevronDownIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Calendar } from "@/components/ui/calendar";
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover";
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetFooter,
    SheetHeader,
    SheetTitle,
    SheetTrigger,
} from "@/components/ui/sheet";
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select";
import { Incident } from "@/lib/store";

interface CreateMaintenanceSheetProps {
    onCreate: (incident: Omit<Incident, 'id'>) => void;
    groups: { id: string; name: string }[];
}

export function CreateMaintenanceSheet({ onCreate, groups }: CreateMaintenanceSheetProps) {
    const [title, setTitle] = useState("");
    const [description, setDescription] = useState("");
    const [selectedGroupId, setSelectedGroupId] = useState<string>("");

    // Separate Date and Time states
    const [startDate, setStartDate] = useState<Date>();
    const [startTime, setStartTime] = useState("10:00:00");
    const [endDate, setEndDate] = useState<Date>();
    const [endTime, setEndTime] = useState("11:00:00");

    const [open, setOpen] = useState(false);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();

        if (!selectedGroupId) {
            alert("Please select an affected group");
            return;
        }

        if (!startDate || !startTime || !endDate || !endTime) {
            alert("Please select start and end date/time");
            return;
        }

        // Combine Date and Time
        const start = new Date(startDate);
        const [startH, startM] = startTime.split(':');
        start.setHours(parseInt(startH), parseInt(startM));

        const end = new Date(endDate);
        const [endH, endM] = endTime.split(':');
        end.setHours(parseInt(endH), parseInt(endM));

        onCreate({
            title,
            description,
            type: 'maintenance',
            severity: 'minor',
            status: 'scheduled',
            startTime: start.toISOString(),
            endTime: end.toISOString(),
            affectedGroups: [selectedGroupId]
        });

        setOpen(false);
        resetForm();
    };

    const resetForm = () => {
        setTitle("");
        setDescription("");
        setSelectedGroupId("");
        setStartDate(undefined);
        setStartTime("10:00:00");
        setEndDate(undefined);
        setEndTime("11:00:00");
    };

    // Helper for DateTime Row tailored to user request
    const DateTimeRow = ({
        label,
        date,
        setDate,
        time,
        setTime
    }: {
        label: string,
        date: Date | undefined,
        setDate: (d: Date | undefined) => void,
        time: string,
        setTime: (t: string) => void
    }) => {
        const [popoverOpen, setPopoverOpen] = useState(false);

        return (
            <div className="flex gap-4">
                <div className="flex flex-col gap-3">
                    <Label className="px-1 text-xs text-muted-foreground font-medium">{label} Date</Label>
                    <Popover open={popoverOpen} onOpenChange={setPopoverOpen}>
                        <PopoverTrigger asChild>
                            <Button
                                variant="outline"
                                className={cn(
                                    "w-40 justify-between font-normal",
                                    !date && "text-muted-foreground"
                                )}
                            >
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
                        </PopoverContent>
                    </Popover>
                </div>
                <div className="flex flex-col gap-3 flex-1">
                    <Label className="px-1 text-xs text-muted-foreground font-medium">Time</Label>
                    <Input
                        type="time"
                        step="1"
                        value={time}
                        onChange={e => setTime(e.target.value)}
                        className="appearance-none [&::-webkit-calendar-picker-indicator]:hidden [&::-webkit-calendar-picker-indicator]:appearance-none"
                    />
                </div>
            </div>
        );
    };

    return (
        <Sheet open={open} onOpenChange={(val) => { setOpen(val); if (!val) resetForm(); }}>
            <SheetTrigger asChild>
                <Button size="sm" className="gap-2" data-testid="create-maintenance-trigger">
                    <CalendarClock className="w-4 h-4" /> Schedule Maintenance
                </Button>
            </SheetTrigger>
            <SheetContent className="sm:max-w-[500px]">
                <SheetHeader>
                    <SheetTitle>Schedule Maintenance</SheetTitle>
                    <SheetDescription>
                        Plan a maintenance window for a specific group.
                    </SheetDescription>
                </SheetHeader>
                <form onSubmit={handleSubmit} className="grid gap-6 py-6">
                    <div className="grid gap-2">
                        <Label>Title</Label>
                        <Input
                            value={title}
                            onChange={e => setTitle(e.target.value)}
                            required
                            placeholder="e.g. Database Upgrade"
                            data-testid="maintenance-title-input"
                        />
                    </div>

                    <div className="grid gap-2">
                        <Label>Affected Group</Label>
                        <Select value={selectedGroupId} onValueChange={setSelectedGroupId}>
                            <SelectTrigger data-testid="maintenance-group-select">
                                <SelectValue placeholder="Select Group" />
                            </SelectTrigger>
                            <SelectContent>
                                {groups.map(g => (
                                    <SelectItem key={g.id} value={g.id}>{g.name}</SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>

                    <div className="grid gap-4">
                        <DateTimeRow
                            label="Start Time"
                            date={startDate}
                            setDate={setStartDate}
                            time={startTime}
                            setTime={setStartTime}
                        />
                        <DateTimeRow
                            label="End Time"
                            date={endDate}
                            setDate={setEndDate}
                            time={endTime}
                            setTime={setEndTime}
                        />
                        <div className="text-xs text-muted-foreground text-right px-1">
                            Time Zone: {Intl.DateTimeFormat().resolvedOptions().timeZone} ({new Date().toLocaleTimeString('en-US', { timeZoneName: 'short' }).split(' ')[2] || 'Local'})
                        </div>
                    </div>

                    <div className="grid gap-2">
                        <Label>Description</Label>
                        <Input
                            value={description}
                            onChange={e => setDescription(e.target.value)}
                            placeholder="Details about the maintenance..."
                        />
                    </div>

                    <SheetFooter className="mt-4">
                        <Button type="submit" className="w-full" data-testid="create-maintenance-submit">Schedule Maintenance</Button>
                    </SheetFooter>
                </form>
            </SheetContent>
        </Sheet>
    );
}
