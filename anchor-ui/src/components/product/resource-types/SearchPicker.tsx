import { Button } from "@nanostackorg/design-system/components/button";
import {
	Combobox,
	ComboboxContent,
	ComboboxEmpty,
	ComboboxInput,
	ComboboxItem,
	ComboboxList,
} from "@nanostackorg/design-system/components/combobox";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";

export const SEARCH_PAGE = 20;
const SEARCH_DELAY_MS = 250;

function useDebounced(value: string) {
	const [settled, setSettled] = useState(value);
	useEffect(() => {
		const timer = setTimeout(() => setSettled(value), SEARCH_DELAY_MS);
		return () => clearTimeout(timer);
	}, [value]);
	return settled;
}

export interface SearchPickerProps<T> {
	id: string;
	placeholder: string;
	listLabel: string;
	value: T | null;
	onChange: (value: T | null) => void;
	queryKey: readonly unknown[];
	search: (term: string, signal: AbortSignal) => Promise<T[]>;
	itemKey: (item: T) => string;
	itemLabel: (item: T) => string;
	itemDetail?: (item: T) => string | undefined;
	itemBlocked?: (item: T) => string | undefined;
	noMatch: string;
	nothing: string;
	failure: string;
	disabled?: boolean;
}

export function SearchPicker<T>({
	id,
	placeholder,
	listLabel,
	value,
	onChange,
	queryKey,
	search,
	itemKey,
	itemLabel,
	itemDetail,
	itemBlocked,
	noMatch,
	nothing,
	failure,
	disabled,
}: SearchPickerProps<T>) {
	const [open, setOpen] = useState(false);
	const [term, setTerm] = useState("");
	const settled = useDebounced(term.trim());
	const results = useQuery({
		queryKey: [...queryKey, settled],
		queryFn: ({ signal }) => search(settled, signal),
		enabled: !disabled,
		placeholderData: keepPreviousData,
	});
	const items = useMemo(() => {
		const found = results.data ?? [];
		if (!value || found.some((item) => itemKey(item) === itemKey(value)))
			return found;
		return [value, ...found];
	}, [results.data, value, itemKey]);
	const searching = results.isFetching || settled !== term.trim();
	const emptyCatalog =
		!disabled && results.isSuccess && settled === "" && !results.data.length;

	return (
		<Stack space="xs">
			<Combobox<T>
				items={items}
				value={value}
				filter={null}
				disabled={disabled}
				itemToStringLabel={itemLabel}
				isItemEqualToValue={(item, other) => itemKey(item) === itemKey(other)}
				onValueChange={(next) => {
					setTerm("");
					onChange(next);
				}}
				onInputValueChange={(next, details) => {
					if (details.reason === "input-change") setTerm(next);
				}}
				onOpenChange={(next) => {
					setOpen(next);
					if (!next) setTerm("");
				}}
			>
				<ComboboxInput
					id={id}
					placeholder={placeholder}
					disabled={disabled}
					triggerLabel={`Show ${listLabel.toLowerCase()}`}
					aria-busy={open && searching ? true : undefined}
				/>
				<ComboboxContent>
					{results.isPending ? (
						<Box className="flex items-center gap-2 px-4 py-3">
							<Spinner size="sm" />
							<Text tone="muted" size="sm">
								Searching…
							</Text>
						</Box>
					) : (
						<ComboboxEmpty>{settled ? noMatch : nothing}</ComboboxEmpty>
					)}
					<ComboboxList aria-label={listLabel}>
						{(item: T) => {
							const blocked = itemBlocked?.(item);
							const detail = blocked ?? itemDetail?.(item);
							return (
								<ComboboxItem
									key={itemKey(item)}
									value={item}
									disabled={Boolean(blocked)}
								>
									<Box className="flex min-w-0 flex-col">
										<Box
											as="span"
											className="[overflow-wrap:anywhere] text-foreground"
										>
											{itemLabel(item)}
										</Box>
										{detail && (
											<Box
												as="span"
												className="[overflow-wrap:anywhere] font-normal text-muted-foreground text-xs"
											>
												{detail}
											</Box>
										)}
									</Box>
								</ComboboxItem>
							);
						}}
					</ComboboxList>
				</ComboboxContent>
			</Combobox>
			{results.isError && (
				<Inline space="sm" alignY="center">
					<Text tone="critical" size="sm">
						{failure}
					</Text>
					<Button size="sm" variant="outline" onClick={() => results.refetch()}>
						Try again
					</Button>
				</Inline>
			)}
			{emptyCatalog && (
				<Text tone="muted" size="sm">
					{nothing}
				</Text>
			)}
		</Stack>
	);
}
