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
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronDown } from "lucide-react";

import { Button } from "../../ui/button";
import { Checkbox } from "../../ui/checkbox";
import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuTrigger,
} from "../../ui/dropdown-menu";
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

import { type ReactNode, useEffect, useState } from "react";

import { FacetedFilter } from "./FacetedFilter";

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
}: AnchorDataTableProps<TData, TFilters>) {
	const [columnVisibility, setColumnVisibility] = useState<VisibilityState>({});
	const [rowSelection, setRowSelection] = useState<RowSelectionState>({});
	const [selectAllMode, setSelectAllMode] = useState<
		"none" | "page" | "all-matching"
	>("none");

	const buildFilterValues = () =>
		Object.fromEntries(
			(filters ?? []).map((filter) => [filter.key, filter.value]),
		);

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

	// Render standard filters UI if filters prop is provided
	const renderFilters = () => {
		if (!filters || !onFiltersChange) return null;

		const hasActiveFilters = filters.some((filter) =>
			filter.type === "text"
				? filter.value.length > 0
				: filter.value.length > 0,
		);

		const clearAllFilters = () => {
			const clearedFilters = Object.fromEntries(
				filters.map((f) => [f.key, f.type === "text" ? "" : []]),
			);
			onFiltersChange(clearedFilters as TFilters);
		};

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

	const table = useReactTable({
		data,
		columns: enableRowSelection
			? [
					{
						id: "select",
						header: ({ table }) => (
							<DropdownMenu>
								<DropdownMenuTrigger
									render={
										<Checkbox
										checked={
											selectAllMode === "all-matching" ||
											table.getIsAllPageRowsSelected()
										}
										indeterminate={
											selectAllMode !== "all-matching" &&
											!table.getIsAllPageRowsSelected() &&
											table.getIsSomePageRowsSelected()
										}
										aria-label="Select all"
										onCheckedChange={() => {
											setSelectAllMode((prev) =>
												prev === "none"
													? "page"
													: prev === "page"
														? "all-matching"
														: "none",
											);
										}}
										/>
									}
								/>
								<DropdownMenuContent align="start">
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
								</DropdownMenuContent>
							</DropdownMenu>
						),
						cell: ({ row }) => (
							<Checkbox
								checked={row.getIsSelected()}
								onCheckedChange={(value) => {
									row.toggleSelected(!!value);
									setSelectAllMode("none");
								}}
								aria-label="Select row"
							/>
						),
						enableSorting: false,
						enableHiding: false,
					},
					...columns,
				]
			: columns,
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
			</div>
			<div className="border-y border-border">
				<Table>
					<TableHeader className="bg-card">
						{table.getHeaderGroups().map((headerGroup) => (
							<TableRow key={headerGroup.id}>
								{headerGroup.headers.map((header) => (
									<TableHead key={header.id}>
										{header.isPlaceholder ? null : (
											<button
												type="button"
												className={`flex items-center gap-2 ${
													header.column.getCanSort()
														? "cursor-pointer select-none hover:text-foreground"
														: ""
												}`}
												onClick={header.column.getToggleSortingHandler()}
												disabled={!header.column.getCanSort()}
											>
												{flexRender(
													header.column.columnDef.header,
													header.getContext(),
												)}
												{header.column.getCanSort() && (
													<span className="ml-1">
														{header.column.getIsSorted() === "desc" ? (
															<ArrowDown className="size-4" />
														) : header.column.getIsSorted() === "asc" ? (
															<ArrowUp className="size-4" />
														) : (
															<ArrowUpDown className="size-4 text-muted-foreground" />
														)}
													</span>
												)}
											</button>
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
									{(enableRowSelection
										? [{ id: "select" }, ...columns]
										: columns
									).map((column, cellIndex) => (
										<TableCell
											// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholder cells have no stable id.
											key={`skeleton-cell-${rowIndex}-${column.id ?? cellIndex}`}
										>
											<Skeleton className="h-4 w-full" />
										</TableCell>
									))}
								</TableRow>
							))
						) : table.getRowModel().rows.length ? (
							table.getRowModel().rows.map((row) => (
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
							<TableRow>
								<TableCell
									colSpan={
										enableRowSelection ? columns.length + 1 : columns.length
									}
									className="h-24 text-center text-sm text-muted-foreground"
								>
									No results.
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
						disabled={!table.getCanPreviousPage() || loading}
					>
						Previous
					</Button>
					<Button
						variant="outline"
						size="sm"
						onClick={() => table.nextPage()}
						disabled={!table.getCanNextPage() || loading}
					>
						Next
					</Button>
				</div>
				<Select
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
				<span className="ml-auto">{total} total</span>
			</div>
		</div>
	);
}
