import { Check, X } from "lucide-react";
// FacetedFilter.tsx
import * as React from "react";
import { Badge } from "../../ui/badge";
import { Button } from "../../ui/button";
import {
	Command,
	CommandEmpty,
	CommandInput,
	CommandItem,
	CommandList,
} from "../../ui/command";
import { Popover, PopoverContent, PopoverTrigger } from "../../ui/popover";

type Option = { label: string; value: string };

interface FacetedFilterProps {
	label: string;
	options: Option[];
	selected: string[];
	onChange: (selected: string[]) => void;
	placeholder?: string;
	multi?: boolean;
}

export function FacetedFilter({
	label,
	options,
	selected,
	onChange,
	placeholder = "Filter...",
	multi = true,
}: FacetedFilterProps) {
	const [open, setOpen] = React.useState(false);

	const handleSelect = (value: string) => {
		if (multi) {
			if (selected.includes(value)) {
				onChange(selected.filter((v) => v !== value));
			} else {
				onChange([...selected, value]);
			}
		} else {
			onChange([value]);
			setOpen(false);
		}
	};

	const handleClear = (value: string) => {
		onChange(selected.filter((v) => v !== value));
	};

	const [search, setSearch] = React.useState("");
	const inputRef = React.useRef<HTMLInputElement>(null);

	// Filter options based on search
	const filteredOptions = React.useMemo(() => {
		if (!search.trim()) return options;
		return options.filter(
			(opt) =>
				opt.label.toLowerCase().includes(search.toLowerCase()) ||
				opt.value.toLowerCase().includes(search.toLowerCase()),
		);
	}, [search, options]);

	// Check if search is a custom value
	const isCustom =
		search.trim() &&
		!options.some(
			(opt) =>
				opt.value.toLowerCase() === search.trim().toLowerCase() ||
				opt.label.toLowerCase() === search.trim().toLowerCase(),
		);

	return (
		<div className="flex items-center gap-2">
			<Popover open={open} onOpenChange={setOpen}>
				<PopoverTrigger
					render={
						<Button variant="outline" size="sm" className="justify-start" />
					}
				>
					{selected.length > 0
						? `${label}: ${selected
								.map((v) => options.find((opt) => opt.value === v)?.label || v)
								.join(", ")}`
						: `${label}`}
				</PopoverTrigger>
				<PopoverContent className="p-0 w-56">
					<Command>
						<CommandInput
							ref={inputRef}
							placeholder={placeholder}
							value={search}
							onValueChange={setSearch}
							autoFocus
						/>
						<CommandList>
							{filteredOptions.length === 0 && isCustom ? (
								<CommandItem
									key={`add-${search}`}
									onSelect={() => {
										handleSelect(search.trim());
										setSearch("");
										inputRef.current?.blur();
									}}
									className="text-primary"
									value={search.trim()}
								>
									Add "{search.trim()}"
								</CommandItem>
							) : null}
							{filteredOptions.length === 0 && !isCustom ? (
								<CommandEmpty>No options found.</CommandEmpty>
							) : null}
							{filteredOptions.map((opt) => (
								<CommandItem
									key={opt.value}
									onSelect={() => {
										handleSelect(opt.value);
										setSearch("");
										inputRef.current?.blur();
									}}
									className={selected.includes(opt.value) ? "bg-muted" : ""}
								>
									<span>{opt.label}</span>
									{selected.includes(opt.value) && (
										<Check className="ml-auto size-4 text-primary" />
									)}
								</CommandItem>
							))}
						</CommandList>
					</Command>
				</PopoverContent>
			</Popover>
			{selected.map((v) => {
				const opt = options.find((o) => o.value === v);
				return (
					<Badge
						key={v}
						variant="secondary"
						className="flex items-center gap-1"
					>
						{opt?.label || v}
						<button
							type="button"
							className="ml-1"
							onClick={() => handleClear(v)}
							aria-label={`Remove ${opt?.label || v}`}
						>
							<X className="size-3" />
						</button>
					</Badge>
				);
			})}
		</div>
	);
}
