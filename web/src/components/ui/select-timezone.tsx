import * as React from "react"
import { Check, ChevronsUpDown, Search } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover"

export function SelectTimezone({ value, onValueChange, className }: { value?: string, onValueChange?: (value: string) => void, className?: string }) {
    const [open, setOpen] = React.useState(false)
    const [query, setQuery] = React.useState("")

    // Memoize timezones to avoid re-calculating on every render
    const allTimezones = React.useMemo(() => {
        const supported = Intl.supportedValuesOf('timeZone');
        return supported.includes('UTC') ? supported : ['UTC', ...supported];
    }, []);
    const filteredTimezones = React.useMemo(() => {
        const normalizedQuery = query.trim().toLowerCase();
        if (!normalizedQuery) return allTimezones;
        return allTimezones.filter((timezone) => timezone.toLowerCase().includes(normalizedQuery));
    }, [allTimezones, query]);

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    type="button"
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    aria-label="Timezone"
                    data-testid="timezone-select"
                    className={cn("w-full justify-between", className)}
                >
                    {value
                        ? value.replace(/_/g, " ")
                        : "Select timezone..."}
                    <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[300px] p-0">
                <div className="flex items-center border-b px-3">
                    <Search className="mr-2 h-4 w-4 shrink-0 opacity-50" />
                    <Input
                        value={query}
                        onChange={(event) => setQuery(event.target.value)}
                        placeholder="Search timezone..."
                        className="h-11 border-0 bg-transparent px-0 shadow-none focus-visible:ring-0"
                    />
                </div>
                <div role="listbox" aria-label="Timezones" className="max-h-[300px] overflow-y-auto overflow-x-hidden p-1">
                    {filteredTimezones.length === 0 && (
                        <p className="py-6 text-center text-sm">No timezone found.</p>
                    )}
                    {filteredTimezones.map((tz) => (
                            <button
                                type="button"
                                role="option"
                                aria-selected={value === tz}
                                key={tz}
                                onClick={() => {
                                    onValueChange?.(tz)
                                    setOpen(false)
                                    setQuery("")
                                }}
                                className="relative flex w-full cursor-default select-none items-center rounded-sm px-2 py-1.5 text-left text-sm outline-none hover:bg-accent hover:text-accent-foreground focus:bg-accent focus:text-accent-foreground"
                            >
                                <Check
                                    className={cn(
                                        "mr-2 h-4 w-4",
                                        value === tz ? "opacity-100" : "opacity-0"
                                    )}
                                />
                                {tz.replace(/_/g, " ")}
                            </button>
                    ))}
                </div>
            </PopoverContent>
        </Popover>
    )
}
