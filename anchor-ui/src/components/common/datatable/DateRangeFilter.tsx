import { format } from "date-fns";
import { CalendarIcon, X } from "lucide-react";
import * as React from "react";
import { Button } from "../../ui/button";
import { Calendar } from "../../ui/calendar";
import { Popover, PopoverContent, PopoverTrigger } from "../../ui/popover";

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
				<PopoverTrigger asChild>
					<Button variant="outline" size="sm" className="justify-start">
						<CalendarIcon className="mr-2 h-4 w-4" />
						{hasValue ? `${label}: ${displayText}` : label}
					</Button>
				</PopoverTrigger>
				<PopoverContent className="w-auto p-0" align="start">
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
					className="h-8 px-2"
				>
					<X className="h-4 w-4" />
				</Button>
			)}
		</div>
	);
}
