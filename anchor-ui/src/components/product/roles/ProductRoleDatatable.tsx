import {
	type Options,
	type ProductRoleResponse,
	type ProductRoleSearchRequest,
	type SearchProductRolesData,
	SortDirection,
} from "@/client";
import { searchProductRolesOptions } from "@/client/@tanstack/react-query.gen";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Copy, PenLine, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";
import { Button } from "../../ui/button";
import { DeleteProductRoleDialog } from "./DeleteProductRoleDialog";
import { ProductRoleDialog } from "./ProductRoleDialog";

const columnHelper = createColumnHelper<ProductRoleResponse>();

interface ProductRoleDatatableProps {
	productId: string;
}

export function ProductRoleDatatable({ productId }: ProductRoleDatatableProps) {
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
	const [searchProductRolesOptionsParams, setSearchProductRolesOptionsParams] =
		useState<Options<SearchProductRolesData>>(() => ({
			path: {
				product_id: productId,
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<ProductRoleSearchRequest["sort_by"]>(
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
		data: productRoleData,
		isLoading,
		error,
	} = useQuery({
		...searchProductRolesOptions(searchProductRolesOptionsParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = productRoleData ?? {};
	useMemo(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	// Update search options when pagination, sorting, or filters change
	useEffect(() => {
		setSearchProductRolesOptionsParams({
			path: {
				product_id: productId,
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<ProductRoleSearchRequest["sort_by"]>(
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
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<div className={"flex gap-2"}>
						<Button
							variant="outline"
							size="icon"
							onClick={() => {
								navigator.clipboard.writeText(row.original.id);
								toast.success("ID copied to clipboard");
							}}
						>
							<span className="sr-only">Copy ID</span>
							<Copy className="h-4 w-4" />
						</Button>
						<ProductRoleDialog
							productId={productId}
							mode="edit"
							existingRole={row.original}
							trigger={
								<Button variant="outline" size="icon">
									<span className="sr-only">Edit product</span>
									<PenLine className="h-4 w-4" />
								</Button>
							}
						/>
						<DeleteProductRoleDialog
							productId={productId}
							role={row.original}
							trigger={
								<Button variant="outlineDestructive" size="sm">
									<span className="sr-only">Delete role</span>
									<Trash2 />
								</Button>
							}
						/>
					</div>
				),
			}),
		],
		[productId],
	);

	// Build name options from current data
	const nameOptions = useMemo(
		() =>
			Array.from(
				new Set((items as ProductRoleResponse[]).map((item) => item.name)),
			).map((name) => ({ label: name, value: name })),
		[items],
	);

	return (
		<>
			<div className="flex items-center justify-between mb-4">
				<div className="flex items-center gap-2">
					<ProductRoleDialog
						productId={productId}
						mode="create"
						trigger={
							<Button>
								<Plus />
								Create Role
							</Button>
						}
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
				fullTextSearchPlaceHolder="Search roles"
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
					Failed to load roles: {error.message}
				</div>
			)}
		</>
	);
}
