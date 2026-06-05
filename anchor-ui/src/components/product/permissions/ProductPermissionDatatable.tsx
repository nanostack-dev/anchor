import type {
	Options,
	ProductPermissionResponse,
	ProductPermissionSearchRequest,
	SearchProductPermissionsData,
} from "@/client";
import { SortDirection } from "@/client";
import { searchProductPermissionsOptions } from "@/client/@tanstack/react-query.gen";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<ProductPermissionResponse>();

interface ProductPermissionDatatableProps {
	productId: string;
}

export function ProductPermissionDatatable({
	productId,
}: ProductPermissionDatatableProps) {
	const [total, setTotal] = useState(0);

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "created_at", desc: true },
	]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const debouncedFullTextSearch = useDebounce(fullTextSearch, 300);

	const [nameFilter, setNameFilter] = useState<string[]>([]);
	const debouncedName = useDebounce(nameFilter, 100);
	const [
		searchProductPermissionsOptionsParams,
		setSearchProductPermissionsOptionsParams,
	] = useState<Options<SearchProductPermissionsData>>(() => ({
		path: {
			product_id: productId,
		},
		body: {
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<ProductPermissionSearchRequest["sort_by"]>(
				"created_at",
				"created_at",
			),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			full_text_search: debouncedFullTextSearch || undefined,
			filter: {
				names: debouncedName.length > 0 ? debouncedName : undefined,
			},
		},
	}));
	const {
		data: productPermissionData,
		isLoading,
		error,
	} = useQuery({
		...searchProductPermissionsOptions(searchProductPermissionsOptionsParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = productPermissionData ?? {};
	useEffect(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	// Update search options when pagination, sorting, or filters change
	useEffect(() => {
		setSearchProductPermissionsOptionsParams({
			path: {
				product_id: productId,
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<
					ProductPermissionSearchRequest["sort_by"]
				>(sorting[0]?.id ?? "created_at", "created_at"),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					names: debouncedName.length > 0 ? debouncedName : undefined,
				},
			},
		});
	}, [productId, pagination, sorting, debouncedFullTextSearch, debouncedName]);

	const columns = useMemo(
		() => [
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => info.getValue(),
				enableSorting: true,
			}),
			columnHelper.accessor("description", {
				header: () => <span>Description</span>,
				cell: (info) => {
					const description = info.getValue();
					return (
						<Tooltip>
							<TooltipTrigger>
								<span className="text-sm text-muted-foreground max-w-[200px] truncate block">
									{description || "No description"}
								</span>
							</TooltipTrigger>
							<TooltipContent side="top" className="max-w-xs">
								{description || "No description available"}
							</TooltipContent>
						</Tooltip>
					);
				},
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
		],
		[],
	);

	// Build name options from current data
	const nameOptions = useMemo(
		() =>
			Array.from(
				new Set(
					(items as ProductPermissionResponse[]).map((item) => item.name),
				),
			).map((name) => ({ label: name, value: name })),
		[items],
	);

	return (
		<>
			<AnchorDataTable
				columns={columns}
				data={items}
				loading={isLoading}
				total={total}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={setFullTextSearch}
				fullTextSearchPlaceHolder="Search permissions"
				filters={[
					{
						key: "name",
						label: "Name",
						type: "select",
						value: nameFilter,
						options: nameOptions,
						placeholder: "Filter by name",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setNameFilter(Array.isArray(filters.name) ? filters.name : []);
				}}
				enableRowSelection={false}
			/>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load permissions: {error.message}
				</div>
			)}
		</>
	);
}
