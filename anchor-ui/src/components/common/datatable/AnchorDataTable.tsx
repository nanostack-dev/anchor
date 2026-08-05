"use client";

import {
	type ColumnDef,
	type PaginationState,
	type RowSelectionState,
	type SortingState,
	type VisibilityState,
	flexRender,
	getCoreRowModel,
	getFilteredRowModel,
	getPaginationRowModel,
	getSortedRowModel,
	useReactTable,
} from "@tanstack/react-table";
import {
	ArrowDown,
	ArrowUp,
	ArrowUpDown,
	ChevronDown,
	Inbox,
	RotateCw,
	SearchX,
	TriangleAlert,
} from "lucide-react";

import { getApiErrorMessage } from "@/lib/api-error";
import { Button } from "../../ui/button";
import { Checkbox } from "../../ui/checkbox";
import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuTrigger,
} from "../../ui/dropdown-menu";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "../../ui/empty";
import { Input } from "../../ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "../../ui/select";
import { Skeleton } from "../../ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "../../ui/table";

import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import { FacetedFilter } from "./FacetedFilter";

/**
 * How the current selection came to be.
 *
 * "custom" is what individual row checkboxes produce. It exists so the effect
 * below can tell "the user picked these rows" apart from "the selection was
 * cleared" — collapsing both onto "none" is what made a row toggle wipe itself.
 */
type SelectAllMode = "none" | "page" | "all-matching" | "custom";

type FilterDef =
	| {
			key: string;
			label: string;
			type: "text";
			value: string;
			placeholder?: string;
			options?: undefined;
	  }
	| {
			key: string;
			label: string;
			type: "select";
			value: string[];
			options: { label: string; value: string }[];
			placeholder?: string;
			multi?: boolean;
	  };

// biome-ignore lint/suspicious/noExplicitAny: TanStack ColumnDef is invariant in TValue; the shared table must accept mixed accessor value types.
type DataTableColumnDef<TData> = ColumnDef<TData, any>;

interface AnchorDataTableProps<
	TData,
	TFilters = Record<string, string | string[]>,
> {
	columns: DataTableColumnDef<TData>[];
	data: TData[];
	loading?: boolean;
	total: number;
	pagination: PaginationState;
	onPaginationChange: (
		updater: PaginationState | ((old: PaginationState) => PaginationState),
	) => void;
	sorting: SortingState;
	onSortingChange: (
		updater: SortingState | ((old: SortingState) => SortingState),
	) => void;
	fullTextSearch?: string;
	onFullTextSearchChange?: (search: string) => void;
	fullTextSearchPlaceHolder?: string;
	filters?: FilterDef[];
	onFiltersChange?: (filters: TFilters) => void;
	children?: ReactNode;
	pageSizeOptions?: number[];
	onSelectionChange?: (selected: TData[] | "all-matching" | []) => void;
	enableRowSelection?: boolean;
	/**
	 * Plural noun for the rows, e.g. `"organizations"`. Used verbatim in the
	 * empty and error copy so every table names what it could not show.
	 */
	resourceName?: string;
	/** Load failure for `data`. The table renders it in place of the rows. */
	error?: unknown;
	/** Wire to the query's `refetch` so the error state can offer a retry. */
	onRetry?: () => void;
}

/**
 * API messages arrive with inconsistent punctuation, and these are stitched
 * into sentences alongside our own copy. Terminate them so the seam reads.
 */
function errorMessage(error: unknown): string | undefined {
	const raw =
		getApiErrorMessage(error) ??
		(error instanceof Error ? error.message : undefined);
	if (!raw) return undefined;
	return /[.!?]$/.test(raw) ? raw : `${raw}.`;
}

export function AnchorDataTable<
	TData,
	TFilters = Record<string, string | string[]>,
>({
	columns,
	data,
	loading = false,
	total,
	pagination,
	onPaginationChange,
	sorting,
	onSortingChange,
	fullTextSearch,
	onFullTextSearchChange,
	fullTextSearchPlaceHolder = "Search...",
	filters,
	onFiltersChange,
	children,
	pageSizeOptions = [10, 20, 50, 100],
	onSelectionChange,
	enableRowSelection = true,
	resourceName,
	error,
	onRetry,
}: AnchorDataTableProps<TData, TFilters>) {
	const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
	const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
	const [selectAllMode, setSelectAllMode] = useState<SelectAllMode>("none");

	const buildFilterValues = () =>
		Object.fromEntries(
			(filters ?? []).map((filter) => [filter.key, filter.value]),
		);

	// Bulk modes own `rowSelection` outright: each one rewrites it from the mode.
	// "custom" is deliberately absent — under it the row checkboxes are the only
	// writer, so this effect must not touch the selection at all.
	// biome-ignore lint/correctness/useExhaustiveDependencies: page selection must resync when visible rows change across pagination and filtering.
	useEffect(() => {
		if (!enableRowSelection) return;
		if (selectAllMode === "none") {
			setRowSelection({});
			onSelectionChange?.([]);
			return;
		}
		if (selectAllMode === "page") {
			const pageRows = table.getRowModel().rows;
			const selection: RowSelectionState = {};
			for (const row of pageRows) {
				selection[row.id] = true;
			}
			setRowSelection(selection);
			onSelectionChange?.(pageRows.map((row) => row.original));
			return;
		}
		if (selectAllMode === "all-matching") {
			setRowSelection({});
			onSelectionChange?.("all-matching");
		}
	}, [
		selectAllMode,
		enableRowSelection,
		onSelectionChange,
		data,
		pagination.pageIndex,
		pagination.pageSize,
	]);

	// Reporting half of "custom": read the selection the row checkboxes wrote and
	// hand it out. Kept separate from the effect above so that `rowSelection` can
	// be a dependency here without the bulk modes re-triggering on the very state
	// they just wrote.
	// biome-ignore lint/correctness/useExhaustiveDependencies: the report must resend when the picked rows change identity across pagination and filtering.
	useEffect(() => {
		if (!enableRowSelection) return;
		if (selectAllMode !== "custom") return;
		onSelectionChange?.(
			table.getSelectedRowModel().rows.map((row) => row.original),
		);
	}, [
		selectAllMode,
		enableRowSelection,
		onSelectionChange,
		rowSelection,
		data,
	]);

	const hasActiveFilters = (filters ?? []).some(
		(filter) => filter.value.length > 0,
	);
	const hasActiveSearch = (fullTextSearch ?? "").length > 0;
	// A narrowed view: the user asked for a subset, so "nothing here" means
	// "nothing matched", not "you have none".
	const isNarrowed = hasActiveFilters || hasActiveSearch;
	// Name what the user actually applied, so the recovery button names its action.
	const narrowedBy =
		hasActiveFilters && hasActiveSearch
			? "search and filters"
			: hasActiveFilters
				? "filters"
				: "search";

	const clearAllFilters = () => {
		if (!filters || !onFiltersChange) return;
		onFiltersChange(
			Object.fromEntries(
				filters.map((f) => [f.key, f.type === "text" ? "" : []]),
			) as TFilters,
		);
	};

	// Recovery action offered by the "no matches" state: drop every constraint
	// the user has applied, search included, and go back to page one.
	const resetQuery = () => {
		clearAllFilters();
		onFullTextSearchChange?.("");
		onPaginationChange({ ...pagination, pageIndex: 0 });
	};

	// Render standard filters UI if filters prop is provided
	const renderFilters = () => {
		if (!filters || !onFiltersChange) return null;

		return (
			<div className="flex gap-2 items-center">
				{filters.map((filter) =>
					filter.type === "text" ? (
						<Input
							key={filter.key}
							placeholder={filter.placeholder || filter.label}
							value={filter.value}
							onChange={(e) => {
								onPaginationChange({ ...pagination, pageIndex: 0 });
								onFiltersChange({
									...buildFilterValues(),
									[filter.key]: e.target.value,
								} as TFilters);
							}}
							className="max-w-xs"
						/>
					) : filter.type === "select" ? (
						<FacetedFilter
							key={filter.key}
							label={filter.label}
							options={filter.options}
							selected={filter.value}
							multi={filter.multi !== false}
							placeholder={filter.placeholder}
							onChange={(selected) => {
								onPaginationChange({ ...pagination, pageIndex: 0 });
								onFiltersChange({
									...buildFilterValues(),
									[filter.key]: selected,
								} as TFilters);
							}}
						/>
					) : null,
				)}
				{hasActiveFilters && (
					<Button
						variant="ghost"
						size="sm"
						onClick={clearAllFilters}
						className="text-muted-foreground hover:text-foreground"
					>
						Clear all
					</Button>
				)}
			</div>
		);
	};

	// The select column is read through a ref rather than closing over
	// `selectAllMode` directly. A column definition that changes identity makes
	// `flexRender` hand React a brand new component type for every header and
	// cell, so the entire table body unmounts and remounts — on each selection
	// toggle, each keystroke, each page change. Reading the mode from a ref keeps
	// the definition referentially stable while the header still renders the
	// current value: React re-renders the header whenever this component
	// re-renders, which is exactly when the mode changes.
	const selectAllModeRef = useRef(selectAllMode);
	selectAllModeRef.current = selectAllMode;

	const selectColumn = useMemo<DataTableColumnDef<TData>>(
		() => ({
			id: "select",
			// The checkbox and the menu are siblings, not one control.
			//
			// This used to be a single `DropdownMenuTrigger` whose `render` was
			// the Checkbox, and it was inert for two independent reasons. The
			// menu never opened: merging the two through `render` drops the
			// trigger's own open behaviour, and it stays dropped even once the
			// element is clickable. `onCheckedChange` never fired either, but
			// for the unrelated reason below — the `disabled` header button
			// this sat inside swallowed the click before it arrived.
			//
			// Splitting them is the call-site fix. `components/ui/dropdown-menu.tsx`
			// is canonical shadcn and is not where this gets patched.
			header: ({ table }) => (
				<div className="flex items-center gap-1">
					<Checkbox
						checked={
							selectAllModeRef.current === "all-matching" ||
							table.getIsAllPageRowsSelected()
						}
						indeterminate={
							selectAllModeRef.current !== "all-matching" &&
							!table.getIsAllPageRowsSelected() &&
							table.getIsSomePageRowsSelected()
						}
						aria-label="Select all"
						onCheckedChange={(checked) =>
							setSelectAllMode(checked ? "page" : "none")
						}
					/>
					<DropdownMenu>
						<DropdownMenuTrigger
							render={
								<Button
									variant="ghost"
									size="icon-xs"
									aria-label="Selection options"
									className="text-muted-foreground hover:text-foreground"
								/>
							}
						>
							<ChevronDown />
						</DropdownMenuTrigger>
						<DropdownMenuContent align="start">
							{/* The label has to sit inside a Group — Base UI's
								GroupLabel throws "MenuGroupContext is missing" outside
								one, which took the whole popup down with it. */}
							<DropdownMenuGroup>
								<DropdownMenuLabel>Select</DropdownMenuLabel>
								<DropdownMenuItem onClick={() => setSelectAllMode("none")}>
									None
								</DropdownMenuItem>
								<DropdownMenuItem onClick={() => setSelectAllMode("page")}>
									All in current page
								</DropdownMenuItem>
								<DropdownMenuItem
									onClick={() => setSelectAllMode("all-matching")}
								>
									All matching query
								</DropdownMenuItem>
							</DropdownMenuGroup>
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
			),
			cell: ({ row }) => (
				<Checkbox
					checked={row.getIsSelected()}
					onCheckedChange={(value) => {
						row.toggleSelected(!!value);
						setSelectAllMode("custom");
					}}
					aria-label="Select row"
				/>
			),
			enableSorting: false,
			enableHiding: false,
		}),
		[],
	);

	// Memoized for the same reason the definition above is: a fresh array here
	// rebuilds every column on every render, which is what remounts the table.
	const tableColumns = useMemo(
		() => (enableRowSelection ? [selectColumn, ...columns] : columns),
		[enableRowSelection, selectColumn, columns],
	);

	const table = useReactTable({
		data,
		columns: tableColumns,
		pageCount: Math.ceil(total / pagination.pageSize),
		state: {
			pagination,
			sorting,
			globalFilter: fullTextSearch,
			columnVisibility,
			rowSelection,
		},
		manualPagination: true,
		manualSorting: true,
		manualFiltering: true,
		onPaginationChange,
		onSortingChange,
		onGlobalFilterChange: onFullTextSearchChange,
		onColumnVisibilityChange: setColumnVisibility,
		onRowSelectionChange: setRowSelection,
		getCoreRowModel: getCoreRowModel(),
		getPaginationRowModel: getPaginationRowModel(),
		getSortedRowModel: getSortedRowModel(),
		getFilteredRowModel: getFilteredRowModel(),
		debugTable: false,
	});

	const rows = table.getRowModel().rows;
	const bodyColSpan = table.getVisibleLeafColumns().length;
	const noun = resourceName ?? "results";
	const detail = errorMessage(error);
	// A failed refetch behind `keepPreviousData` still has usable rows on screen.
	// Blanking them out would destroy more than it explains, so the full-body
	// error state is reserved for the case where there is nothing left to read.
	const showErrorState = !loading && !!error && rows.length === 0;
	const showStaleWarning = !loading && !!error && rows.length > 0;

	const retryButton = onRetry ? (
		<Button variant="outline" size="sm" onClick={onRetry}>
			<RotateCw />
			Try again
		</Button>
	) : null;

	return (
		<div className="w-full overflow-hidden rounded-xl border border-border bg-card shadow-sm">
			<div className="flex flex-col gap-2 p-4">
				<div className="flex items-center gap-2">
					{onFullTextSearchChange && (
						<Input
							placeholder={fullTextSearchPlaceHolder}
							value={fullTextSearch ?? ""}
							onChange={(e) => {
								onPaginationChange({ ...pagination, pageIndex: 0 });
								onFullTextSearchChange(e.target.value);
							}}
							className="max-w-sm"
						/>
					)}
					{children}
					<DropdownMenu>
						<DropdownMenuTrigger
							render={<Button variant="outline" className="ml-auto" />}
						>
							Columns <ChevronDown />
						</DropdownMenuTrigger>
						<DropdownMenuContent align="end">
							{table
								.getAllLeafColumns()
								.filter((column) => column.getCanHide())
								.map((column) => (
									<DropdownMenuCheckboxItem
										key={column.id}
										className="capitalize"
										checked={column.getIsVisible()}
										onCheckedChange={(value) =>
											column.toggleVisibility(!!value)
										}
									>
										{column.id}
									</DropdownMenuCheckboxItem>
								))}
						</DropdownMenuContent>
					</DropdownMenu>
				</div>
				{renderFilters()}
				{showStaleWarning && (
					<output className="flex items-center gap-2 text-sm text-destructive">
						<TriangleAlert className="size-4 shrink-0" />
						<span>
							Couldn&rsquo;t refresh {noun}
							{detail ? `: ${detail}` : "."} Showing the last results loaded.
						</span>
						{retryButton}
					</output>
				)}
			</div>
			<div className="border-y border-border">
				<Table>
					<TableHeader className="bg-card">
						{table.getHeaderGroups().map((headerGroup) => (
							<TableRow key={headerGroup.id}>
								{headerGroup.headers.map((header) => (
									<TableHead key={header.id}>
										{header.isPlaceholder ? null : header.column.getCanSort() ? (
											<button
												type="button"
												className="flex cursor-pointer items-center gap-2 select-none hover:text-foreground"
												onClick={header.column.getToggleSortingHandler()}
											>
												{flexRender(
													header.column.columnDef.header,
													header.getContext(),
												)}
												<span className="ml-1">
													{header.column.getIsSorted() === "desc" ? (
														<ArrowDown className="size-4" />
													) : header.column.getIsSorted() === "asc" ? (
														<ArrowUp className="size-4" />
													) : (
														<ArrowUpDown className="size-4 text-muted-foreground" />
													)}
												</span>
											</button>
										) : (
											// Unsortable headers render bare. Every header used to be
											// wrapped in a button that was `disabled` when the column
											// could not sort, and a disabled button makes its whole
											// subtree unclickable — which is what kept the selection
											// header's checkbox from ever seeing a click.
											<div className="flex items-center gap-2">
												{flexRender(
													header.column.columnDef.header,
													header.getContext(),
												)}
											</div>
										)}
									</TableHead>
								))}
							</TableRow>
						))}
					</TableHeader>
					<TableBody>
						{loading ? (
							Array.from({ length: 5 }).map((_, rowIndex) => (
								<TableRow
									// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholder rows have no stable id.
									key={`skeleton-row-${rowIndex}`}
								>
									{tableColumns.map((column, cellIndex) => (
										<TableCell
											key={`skeleton-cell-${rowIndex}-${column.id ?? cellIndex}`}
										>
											<Skeleton className="h-4 w-full" />
										</TableCell>
									))}
								</TableRow>
							))
						) : showErrorState ? (
							<TableRow className="hover:bg-transparent">
								<TableCell colSpan={bodyColSpan} className="p-0">
									<Empty
										aria-live="polite"
										className="border-none bg-transparent py-10"
									>
										<EmptyHeader>
											<EmptyMedia variant="icon" className="text-destructive">
												<TriangleAlert />
											</EmptyMedia>
											<EmptyTitle>Couldn&rsquo;t load {noun}</EmptyTitle>
											<EmptyDescription>
												{detail ??
													"The request did not complete. Check your connection and try again."}
											</EmptyDescription>
										</EmptyHeader>
										{retryButton}
									</Empty>
								</TableCell>
							</TableRow>
						) : rows.length ? (
							rows.map((row) => (
								<TableRow
									key={row.id}
									data-state={row.getIsSelected() && "selected"}
								>
									{row.getVisibleCells().map((cell) => (
										<TableCell key={cell.id}>
											{flexRender(
												cell.column.columnDef.cell,
												cell.getContext(),
											)}
										</TableCell>
									))}
								</TableRow>
							))
						) : (
							<TableRow className="hover:bg-transparent">
								<TableCell colSpan={bodyColSpan} className="p-0">
									<Empty className="border-none bg-transparent py-10">
										<EmptyHeader>
											<EmptyMedia variant="icon">
												{isNarrowed ? <SearchX /> : <Inbox />}
											</EmptyMedia>
											<EmptyTitle>
												{isNarrowed
													? `No ${noun} match your ${narrowedBy}`
													: `No ${noun} yet`}
											</EmptyTitle>
											<EmptyDescription>
												{isNarrowed
													? `Try something broader, or clear the ${narrowedBy} you have applied.`
													: `Once ${noun} exist, they will appear here.`}
											</EmptyDescription>
										</EmptyHeader>
										{isNarrowed && (
											<Button variant="outline" size="sm" onClick={resetQuery}>
												Clear {narrowedBy}
											</Button>
										)}
									</Empty>
								</TableCell>
							</TableRow>
						)}
					</TableBody>
				</Table>
			</div>
			<div className="flex items-center justify-end gap-2 p-4">
				{enableRowSelection && (
					<div className="flex-1 text-sm text-muted-foreground">
						{Object.keys(rowSelection).length} of{" "}
						{table.getFilteredRowModel().rows.length} row(s) selected.
					</div>
				)}
				<div className="flex gap-2">
					<Button
						variant="outline"
						size="sm"
						onClick={() => table.previousPage()}
						disabled={!table.getCanPreviousPage() || loading || showErrorState}
					>
						Previous
					</Button>
					<Button
						variant="outline"
						size="sm"
						onClick={() => table.nextPage()}
						disabled={!table.getCanNextPage() || loading || showErrorState}
					>
						Next
					</Button>
				</div>
				<Select
					items={pageSizeOptions.map((size) => ({
						value: String(size),
						label: `Show ${size}`,
					}))}
					value={String(pagination.pageSize)}
					onValueChange={(value) =>
						onPaginationChange({
							...pagination,
							pageSize: Number(value),
							pageIndex: 0,
						})
					}
				>
					<SelectTrigger className="ml-4 w-32" size="sm">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{pageSizeOptions.map((size) => (
							<SelectItem key={size} value={String(size)}>
								Show {size}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<span className="ml-auto text-sm text-muted-foreground">
					{showErrorState ? "" : `${total} total`}
				</span>
			</div>
		</div>
	);
}
