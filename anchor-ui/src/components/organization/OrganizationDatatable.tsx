import type {
	Options,
	ProductOrganizationResponse,
	ProductOrganizationSearchRequest,
	SearchProductOrganizationsData,
} from "@/client";
import { SortDirection } from "@/client";
import { searchProductOrganizationsOptions } from "@/client/@tanstack/react-query.gen";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<ProductOrganizationResponse>();

export function OrganizationDatatable() {
	const { currentProduct } = useProduct();
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

	const [idFilter, setIdFilter] = useState<string[]>([]);
	const debouncedIds = useDebounce(idFilter, 100);

	const searchProductOrganizationsOptionsParams = useMemo<
		Options<SearchProductOrganizationsData>
	>(
		() => ({
			path: {
				product_id: currentProduct?.id ?? "",
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<
					ProductOrganizationSearchRequest["sort_by"]
				>(sorting[0]?.id, "created_at"),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					names: debouncedName.length > 0 ? debouncedName : undefined,
					ids: debouncedIds.length > 0 ? debouncedIds : undefined,
				},
			},
		}),
		[
			currentProduct?.id,
			pagination,
			sorting,
			debouncedFullTextSearch,
			debouncedName,
			debouncedIds,
		],
	);

	const {
		data: organizationData,
		isLoading,
		error,
		refetch,
	} = useQuery({
		...searchProductOrganizationsOptions(
			searchProductOrganizationsOptionsParams,
		),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct?.id,
	});

	const { items = [], total: fetchedTotal = 0 } = organizationData ?? {};
	useEffect(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

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
						<span className="text-sm text-muted-foreground max-w-[200px] truncate block">
							{description || "No description"}
						</span>
					);
				},
				enableSorting: false,
			}),
			columnHelper.accessor("id", {
				header: () => <span>ID</span>,
				cell: (info) => (
					<span className="text-sm font-mono text-muted-foreground max-w-[150px] truncate block">
						{info.getValue()}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
			columnHelper.accessor("updated_at", {
				header: () => <span>Updated At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
		],
		[],
	);

	// Build filter options from current data
	const nameOptions = useMemo(
		() =>
			Array.from(
				new Set(
					(items as ProductOrganizationResponse[]).map((item) => item.name),
				),
			).map((name) => ({ label: name, value: name })),
		[items],
	);

	const idOptions = useMemo(
		() =>
			(items as ProductOrganizationResponse[]).map((item) => ({
				label: item.id,
				value: item.id,
			})),
		[items],
	);

	if (!currentProduct) {
		return (
			<div className="flex items-center justify-center p-8">
				<div className="text-center">
					<p className="text-muted-foreground">
						Please select a product to view organizations
					</p>
				</div>
			</div>
		);
	}

	return (
		<AnchorDataTable
			columns={columns}
			data={items}
			loading={isLoading}
			resourceName="organizations"
			error={error}
			onRetry={() => {
				void refetch();
			}}
			total={total}
			pagination={pagination}
			onPaginationChange={setPagination}
			sorting={sorting}
			onSortingChange={setSorting}
			fullTextSearch={fullTextSearch}
			onFullTextSearchChange={setFullTextSearch}
			fullTextSearchPlaceHolder="Search organizations"
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
				{
					key: "id",
					label: "ID",
					type: "select",
					value: idFilter,
					options: idOptions,
					placeholder: "Filter by ID",
					multi: true,
				},
			]}
			onFiltersChange={(filters) => {
				setPagination((p) => ({ ...p, pageIndex: 0 }));
				setNameFilter(Array.isArray(filters.name) ? filters.name : []);
				setIdFilter(Array.isArray(filters.id) ? filters.id : []);
			}}
			enableRowSelection={false}
		/>
	);
}
