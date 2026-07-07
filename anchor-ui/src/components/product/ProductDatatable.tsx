import {
	type Options,
	type ProductResponse,
	type ProductSearchRequest,
	type SearchProductsData,
	SortDirection,
} from "@/client";
import {
	searchProductsOptions,
	searchProductsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import {
	keepPreviousData,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { PenLine, Plus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../common/datatable/AnchorDataTable";
import { Button } from "../ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "../ui/tooltip";
import { ProductCreateDialog } from "./ProductCreateDialog";
import { ProductDeleteDialog } from "./ProductDeleteDialog";

const columnHelper = createColumnHelper<ProductResponse>();

export function ProductDatatable() {
	const queryClient = useQueryClient();
	const navigate = useNavigate();
	const { refreshProducts } = useProduct();
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

	// Faceted name filter state (multi-select)
	const [nameFilter, setNameFilter] = useState<string[]>([]);
	const debouncedName = useDebounce(nameFilter, 100);
	const [searchProductsOptionsParams, setSearchProductsOptionsParams] =
		useState<Options<SearchProductsData>>(() => ({
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<ProductSearchRequest["sort_by"]>(
					sorting[0]?.id,
					"created_at",
				),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					names: debouncedName.length > 0 ? debouncedName : undefined,
				},
			},
		}));
	const {
		data: productData,
		isLoading,
		error,
	} = useQuery({
		...searchProductsOptions(searchProductsOptionsParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = productData ?? {};
	useMemo(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	// Update search options when pagination, sorting, or filters change
	useEffect(() => {
		setSearchProductsOptionsParams({
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<ProductSearchRequest["sort_by"]>(
					sorting[0]?.id ?? "created_at",
					"created_at",
				),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					names: debouncedName.length > 0 ? debouncedName : undefined,
				},
			},
		});
	}, [pagination, sorting, debouncedFullTextSearch, debouncedName]);

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
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<div className={"flex gap-2"}>
						<Tooltip>
							<TooltipTrigger
								render={
									<Button
										size="icon"
										variant="outline"
										onClick={() => {
											navigate({
												to: "/products/$productId/edit",
												params: { productId: row.original.id },
											});
										}}
									/>
								}
							>
								<span className="sr-only">Edit product</span>
								<PenLine className="h-4 w-4" />
							</TooltipTrigger>
							<TooltipContent>Edit product</TooltipContent>
						</Tooltip>
						<ProductDeleteDialog
							product={row.original}
							onDeleted={async () => {
								await queryClient.invalidateQueries({
									queryKey: searchProductsQueryKey(searchProductsOptionsParams),
								});
								setPagination((p) => ({ ...p }));
							}}
						/>
					</div>
				),
			}),
		],
		[queryClient, navigate, searchProductsOptionsParams],
	);

	// Build name options from current data
	const nameOptions = useMemo(
		() =>
			Array.from(
				new Set((items as ProductResponse[]).map((item) => item.name)),
			).map((name) => ({ label: name, value: name })),
		[items],
	);

	return (
		<>
			<div className="flex items-center justify-between mb-4">
				<div className="flex items-center gap-2">
					<ProductCreateDialog
						trigger={
							<Button>
								<Plus />
								Create Product
							</Button>
						}
						onCreated={async () => {
							await queryClient.invalidateQueries({
								queryKey: searchProductsQueryKey(searchProductsOptionsParams),
							});
							setPagination((p) => ({ ...p }));
							refreshProducts();
						}}
					/>
				</div>
			</div>
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
				fullTextSearchPlaceHolder="Search products"
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
					Failed to load products: {error.message}
				</div>
			)}
		</>
	);
}
