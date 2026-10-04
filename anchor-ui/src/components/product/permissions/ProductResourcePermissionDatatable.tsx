import { deleteProductResourcePermission } from "@/client";
import type {
	Options,
	ProductPermissionResponse,
	ProductResourcePermissionSearchRequest,
	SearchProductResourcePermissionsData,
} from "@/client";
import { SortDirection } from "@/client";
import { searchProductResourcePermissionsOptions } from "@/client/@tanstack/react-query.gen";
import { CreateProductResourcePermissionDialog } from "@/components/product/permissions/CreateProductResourcePermissionDialog";
import { DeleteProductResourcePermissionDialog } from "@/components/product/permissions/DeleteProductResourcePermissionDialog";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Button } from "@nanostackorg/design-system/components/button";
import { Box } from "@nanostackorg/design-system/layout/box";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Eye, PenLine, Plus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<ProductPermissionResponse>();

interface ProductPermissionDatatableProps {
	productId: string;
}

export function ProductResourcePermissionDatatable({
	productId,
}: ProductPermissionDatatableProps) {
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
	const [
		searchProductResourcePermissionsOptionsParams,
		setSearchProductResourcePermissionsOptionsParams,
	] = useState<Options<SearchProductResourcePermissionsData>>(() => ({
		path: {
			product_id: productId,
		},
		body: {
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<
				ProductResourcePermissionSearchRequest["sort_by"]
			>("created_at", "created_at"),
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
		isFetching,
		error,
		refetch,
	} = useQuery({
		...searchProductResourcePermissionsOptions(
			searchProductResourcePermissionsOptionsParams,
		),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = productPermissionData ?? {};
	useMemo(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	// Update search options when pagination, sorting, or filters change
	useEffect(() => {
		setSearchProductResourcePermissionsOptionsParams({
			path: {
				product_id: productId,
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<
					ProductResourcePermissionSearchRequest["sort_by"]
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
						<ButtonLink
							aria-label={`View ${row.original.name}`}
							href={ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL.replace(
								"$permissionName",
								encodeURIComponent(row.original.name),
							)}
							variant="outline"
							tone="neutral"
						>
							<Eye />
						</ButtonLink>
						<ButtonLink
							aria-label={`Edit ${row.original.name}`}
							href={`${ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL.replace(
								"$permissionName",
								encodeURIComponent(row.original.name),
							)}?${new URLSearchParams({ edit: "true" }).toString()}`}
							variant="outline"
							tone="neutral"
						>
							<PenLine />
						</ButtonLink>
						<DeleteProductResourcePermissionDialog
							productId={productId}
							permission={row.original}
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
				new Set(
					(items as ProductPermissionResponse[]).map((item) => item.name),
				),
			).map((name) => ({ label: name, value: name })),
		[items],
	);

	return (
		<>
			<Box className="flex items-center justify-between mb-4">
				<Box className="flex items-center gap-2">
					<CreateProductResourcePermissionDialog
						productId={productId}
						trigger={
							<Button variant="solid" tone="brand">
								<Plus />
								Create Permission
							</Button>
						}
					/>
				</Box>
			</Box>
			<AnchorDataTable
				columns={columns}
				data={items}
				onRowClick={(permission) => {
					void navigate({
						to: ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL,
						params: { permissionName: permission.name },
						search: { edit: true },
					});
				}}
				loading={isLoading || isFetching}
				resourceName="resource permissions"
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
				getRowId={(row) => row.name}
				getRowLabel={(row) => row.name}
				selectionScope={productId}
				bulkActions={[
					{
						id: "delete",
						label: "Delete selected",
						description:
							"Any roles or API keys using the selected permissions will lose them immediately. This cannot be undone.",
						destructive: true,
						removesRows: true,
						run: (row) =>
							deleteProductResourcePermission({
								path: { product_id: productId, permission_name: row.name },
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
