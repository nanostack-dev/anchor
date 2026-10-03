import { deleteProductRole } from "@/client";
import {
	type Options,
	type ProductRoleResponse,
	type ProductRoleSearchRequest,
	type SearchProductRolesData,
	SortDirection,
} from "@/client";
import { searchProductRolesOptions } from "@/client/@tanstack/react-query.gen";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { IconButton } from "@nanostackorg/design-system/components/button";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Copy, Eye, PenLine, Plus, Trash2 } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";
import { DeleteProductRoleDialog } from "./DeleteProductRoleDialog";
import { ProductRoleDialog } from "./ProductRoleDialog";

const columnHelper = createColumnHelper<ProductRoleResponse>();

interface ProductRoleDatatableProps {
	productId: string;
}

export function ProductRoleDatatable({ productId }: ProductRoleDatatableProps) {
	const navigate = useNavigate();
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
		isFetching,
		error,
		refetch,
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
						<Box
							as="span"
							className="text-sm text-muted-foreground max-w-[200px] truncate block"
						>
							{description || "No description"}
						</Box>
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
					<Box className={"flex gap-2"}>
						<IconButton
							variant="outline"
							size="md"
							onClick={() => {
								navigator.clipboard.writeText(row.original.id);
								toast.add({ type: "success", title: "ID copied to clipboard" });
							}}
							icon={Copy}
							label="Copy ID"
						/>
						<ButtonLink
							aria-label={`View ${row.original.name}`}
							href={ROUTE_PATHS.PRODUCT_ROLE_DETAIL.replace(
								"$roleId",
								encodeURIComponent(row.original.id),
							)}
							variant="outline"
							tone="neutral"
						>
							<Eye />
						</ButtonLink>
						<ButtonLink
							aria-label={`Edit ${row.original.name}`}
							href={`${ROUTE_PATHS.PRODUCT_ROLE_DETAIL.replace(
								"$roleId",
								encodeURIComponent(row.original.id),
							)}?${new URLSearchParams({ edit: "true" }).toString()}`}
							variant="outline"
							tone="neutral"
						>
							<PenLine />
						</ButtonLink>
						<DeleteProductRoleDialog
							productId={productId}
							role={row.original}
							trigger={
								<IconButton
									tone="critical"
									variant="outline"
									size="sm"
									icon={Trash2}
									label="Delete role"
								/>
							}
						/>
					</Box>
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
			<Box className="flex items-center justify-between mb-4">
				<Box className="flex items-center gap-2">
					<ProductRoleDialog
						productId={productId}
						mode="create"
						trigger={
							<Button variant="solid" tone="brand">
								<Plus />
								Create Role
							</Button>
						}
					/>
				</Box>
			</Box>
			<AnchorDataTable
				columns={columns}
				data={items}
				onRowClick={(role) => {
					void navigate({
						to: ROUTE_PATHS.PRODUCT_ROLE_DETAIL,
						params: { roleId: role.id },
						search: { edit: true },
					});
				}}
				loading={isLoading || isFetching}
				resourceName="roles"
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
				getRowId={(row) => row.id}
				getRowLabel={(row) => row.name}
				selectionScope={productId}
				bulkActions={[
					{
						id: "delete",
						label: "Delete selected",
						description:
							"This permanently deletes the selected roles. Roles currently assigned to users cannot be deleted.",
						destructive: true,
						removesRows: true,
						run: (row) =>
							deleteProductRole({
								path: { product_id: productId, role_id: row.id },
								throwOnError: true,
							}),
					},
				]}
				onBulkActionComplete={async () => {
					await refetch({ throwOnError: true });
				}}
			/>
		</>
	);
}
