import * as React from "react"
import { Check, ChevronsUpDown } from "lucide-react"

import { cn } from "@/lib/utils"
import { Button } from "@/components/ui/button"
import {
    Command,
    CommandEmpty,
    CommandGroup,
    CommandInput,
    CommandItem,
    CommandList,
} from "@/components/ui/command"
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from "@/components/ui/popover"

export function SelectTimezone({ value, onValueChange, className }: { value?: string, onValueChange?: (value: string) => void, className?: string }) {
    const [open, setOpen] = React.useState(false)

    // Memoize timezones to avoid re-calculating on every render
    const allTimezones = React.useMemo(() => Intl.supportedValuesOf('timeZone'), []);

    return (
        <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
                <Button
                    variant="outline"
                    role="combobox"
                    aria-expanded={open}
                    className={cn("w-full justify-between", className)}
                >
                    {value
                        ? value.replace(/_/g, " ")
                        : "Select timezone..."}
                    <ChevronsUpDown className="ml-2 h-4 w-4 shrink-0 opacity-50" />
                </Button>
            </PopoverTrigger>
            <PopoverContent className="w-[300px] p-0">
                <Command>
                    <CommandInput placeholder="Search timezone..." />
                    <CommandList>
                        <CommandEmpty>No timezone found.</CommandEmpty>
                        <CommandGroup>
                            {allTimezones.map((tz) => (
                                <CommandItem
                                    key={tz}
                                    value={tz}
                                    onSelect={(currentValue) => {
                                        // Shadcn Combobox standard:
                                        // currentValue is the lowercase "value" prop usually, or text content if value missing.
                                        // But here we explicitly set value={tz}.
                                        // We want to pass the original 'tz' string (preserving case) if possible, 
                                        // or we map back if cmdk lowercases it. 
                                        // Creating a map or found check is safest.

                                        // Simple lookup to ensure correct casing:
                                        const original = allTimezones.find((t) => t.toLowerCase() === currentValue.toLowerCase()) || currentValue;

                                        onValueChange?.(original)
                                        setOpen(false)
                                    }}
                                >
                                    <Check
                                        className={cn(
                                            "mr-2 h-4 w-4",
                                            value === tz ? "opacity-100" : "opacity-0"
                                        )}
                                    />
                                    {tz.replace(/_/g, " ")}
                                </CommandItem>
                            ))}
                        </CommandGroup>
                    </CommandList>
                </Command>
            </PopoverContent>
        </Popover>
    )
}
