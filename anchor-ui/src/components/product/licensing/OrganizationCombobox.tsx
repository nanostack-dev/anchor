import type { ProductOrganizationResponse } from "@/client";
import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { Button } from "@/components/ui/button";
import {
	Command,
	CommandEmpty,
	CommandInput,
	CommandItem,
	CommandList,
} from "@/components/ui/command";
import {
	Popover,
	PopoverContent,
	PopoverTrigger,
} from "@/components/ui/popover";
import { Spinner } from "@/components/ui/spinner";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useDebounce } from "@uidotdev/usehooks";
import { Check, ChevronDown } from "lucide-react";
import { useState } from "react";

interface OrganizationComboboxProps {
	productId: string;
	value: ProductOrganizationResponse | null;
	onChange: (organization: ProductOrganizationResponse) => void;
	disabled?: boolean;
}

/**
 * Searchable organization picker backed by the product organization search
 * endpoint (server-side full-text search, first 20 matches).
 */
export function OrganizationCombobox({
	productId,
	value,
	onChange,
	disabled = false,
}: OrganizationComboboxProps) {
	const [open, setOpen] = useState(false);
	const [search, setSearch] = useState("");
	const debouncedSearch = useDebounce(search, 300);

	const { data, isFetching } = useQuery({
		...searchProductOrganizationsOptions({
			path: { product_id: productId },
			body: {
				pagination: { limit: 20, offset: 0 },
				full_text_search: debouncedSearch.trim() || undefined,
			},
		}),
		placeholderData: keepPreviousData,
		enabled: open,
	});

	const organizations = data?.items ?? [];

	return (
		<Popover open={open} onOpenChange={setOpen}>
			<PopoverTrigger
				render={
					<Button
						variant="outline"
						className="w-full justify-between"
						disabled={disabled}
					/>
				}
			>
				<span className={value ? "" : "text-muted-foreground"}>
					{value ? value.name : "Select an organization"}
				</span>
				<ChevronDown className="size-4 text-muted-foreground" />
			</PopoverTrigger>
			<PopoverContent className="p-0 w-80">
				<Command shouldFilter={false}>
					<CommandInput
						placeholder="Search organizations..."
						value={search}
						onValueChange={setSearch}
						autoFocus
					/>
					<CommandList>
						{isFetching && organizations.length === 0 ? (
							<div className="flex items-center justify-center p-4">
								<Spinner className="text-muted-foreground" />
							</div>
						) : organizations.length === 0 ? (
							<CommandEmpty>No organizations found.</CommandEmpty>
						) : (
							organizations.map((organization) => (
								<CommandItem
									key={organization.id}
									value={organization.id}
									onSelect={() => {
										onChange(organization);
										setOpen(false);
									}}
								>
									<div className="flex min-w-0 flex-col">
										<span className="truncate">{organization.name}</span>
										<span className="truncate font-mono text-xs text-muted-foreground">
											{organization.id}
										</span>
									</div>
									{value?.id === organization.id && (
										<Check className="ml-auto size-4 text-primary" />
									)}
								</CommandItem>
							))
						)}
					</CommandList>
				</Command>
			</PopoverContent>
		</Popover>
	);
}
