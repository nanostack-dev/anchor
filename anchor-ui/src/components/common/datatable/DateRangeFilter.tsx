import { Button } from "@nanostackorg/design-system/components/button";
import { Calendar } from "@nanostackorg/design-system/components/calendar";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@nanostackorg/design-system/components/popover";
import { format } from "date-fns";
import { CalendarIcon, X } from "lucide-react";
import * as React from "react";

interface DateRangeFilterProps {
	label: string;
	value: { from?: string; to?: string };
	onChange: (value: { from?: string; to?: string }) => void;
	placeholder?: string;
}

export function DateRangeFilter({
	label,
	value,
	onChange,
	placeholder = "Pick date range",
}: DateRangeFilterProps) {
	const [open, setOpen] = React.useState(false);

	const fromDate = value.from ? new Date(value.from) : undefined;
	const toDate = value.to ? new Date(value.to) : undefined;

	const handleDateSelect = (range: { from?: Date; to?: Date } | undefined) => {
		if (!range) {
			onChange({ from: undefined, to: undefined });
			return;
		}

		onChange({
			from: range.from ? range.from.toISOString().split("T")[0] : undefined,
			to: range.to ? range.to.toISOString().split("T")[0] : undefined,
		});
	};

	const handleClear = () => {
		onChange({ from: undefined, to: undefined });
	};

	const hasValue = value.from || value.to;
	const displayText = hasValue
		? `${value.from ? format(new Date(value.from), "MMM dd") : "..."} - ${value.to ? format(new Date(value.to), "MMM dd") : "..."}`
		: placeholder;

	return (
		<div className="flex items-center gap-2">
			<Popover open={open} onOpenChange={setOpen}>
				<PopoverTrigger render={<Button variant="outline" size="sm" />}>
					<CalendarIcon className="mr-2 h-4 w-4" />
					{hasValue ? `${label}: ${displayText}` : label}
				</PopoverTrigger>
				<PopoverContent align="start" aria-label={`${label} date range`}>
					<Calendar
						mode="range"
						selected={{ from: fromDate, to: toDate }}
						onSelect={handleDateSelect}
						numberOfMonths={2}
					/>
				</PopoverContent>
			</Popover>
			{hasValue && (
				<Button
					variant="ghost"
					size="sm"
					onClick={handleClear}
					aria-label={`Clear ${label.toLowerCase()}`}
				>
					<X className="h-4 w-4" />
				</Button>
			)}
		</div>
	);
}
