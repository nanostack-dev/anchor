import { deleteProductRole } from "@/client";
import {
	type ProductRoleResponse,
	type ProductRoleSearchRequest,
	SortDirection,
} from "@/client";
import { searchProductRolesOptions } from "@/client/@tanstack/react-query.gen";
import { useScopeTypes } from "@/components/product/resource-types/ResourceTypeSelect";
import { typeName } from "@/components/product/resource-types/resource-type-scope";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import {
	ButtonLink,
	IconButton,
} from "@nanostackorg/design-system/components/button";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Copy, Eye, PenLine, Plus, Trash2 } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";
import { DeleteProductRoleDialog } from "./DeleteProductRoleDialog";

const columnHelper = createColumnHelper<ProductRoleResponse>();

interface ProductRoleDatatableProps {
	productId: string;
	resourceTypes?: string[];
	onResourceTypesChange?: (slugs: string[]) => void;
}

export function ProductRoleDatatable({
	productId,
	resourceTypes = [],
	onResourceTypesChange,
}: ProductRoleDatatableProps) {
	const navigate = useNavigate();
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
	const [localTypes, setLocalTypes] = useState(resourceTypes);
	const typeFilter = onResourceTypesChange ? resourceTypes : localTypes;
	const scopeTypes = useScopeTypes(productId);
	const types = scopeTypes.data ?? [];
	const typesRef = useRef(types);
	typesRef.current = types;

	const searchOptions = useMemo(
		() => ({
			path: { product_id: productId },
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
					resource_types: typeFilter.length > 0 ? typeFilter : undefined,
				},
			},
		}),
		[
			productId,
			pagination,
			sorting,
			debouncedFullTextSearch,
			debouncedName,
			typeFilter,
		],
	);
	const {
		data: productRoleData,
		isLoading,
		isFetching,
		error,
		refetch,
	} = useQuery({
		...searchProductRolesOptions(searchOptions),
		placeholderData: keepPreviousData,
	});
	const { items = [], total = 0 } = productRoleData ?? {};

	const columns = useMemo(
		() => [
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => (
					<Box
						as="span"
						className="block min-w-0 max-w-[16rem] whitespace-normal font-medium break-words"
					>
						{info.getValue()}
					</Box>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("resource_type", {
				header: () => <span>Resource type</span>,
				cell: (info) => (
					<Box
						as="span"
						className="block max-w-[12rem] whitespace-normal break-words"
					>
						{typeName(typesRef.current, info.getValue())}
					</Box>
				),
				enableSorting: false,
			}),
			columnHelper.accessor((role) => role.permissions.length, {
				id: "permissions",
				header: () => <span>Permissions</span>,
				cell: (info) => (
					<Box as="span" className="whitespace-nowrap tabular-nums">
						{info.getValue()}
					</Box>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("description", {
				header: () => <span>Description</span>,
				cell: (info) => (
					<Box
						as="span"
						className="block max-w-[200px] truncate text-muted-foreground text-sm"
					>
						{info.getValue() || "No description"}
					</Box>
				),
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
					<Box className="flex gap-2">
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

	const nameOptions = useMemo(
		() =>
			Array.from(new Set(items.map((item) => item.name))).map((name) => ({
				label: name,
				value: name,
			})),
		[items],
	);
	const typeOptions = useMemo(
		() => types.map((type) => ({ label: type.name, value: type.slug })),
		[types],
	);

	return (
		<>
			<Box className="mb-4 flex items-center justify-between">
				<ButtonLink
					href={ROUTE_PATHS.PRODUCT_ROLE_NEW}
					variant="solid"
					tone="brand"
				>
					<Plus />
					Create Role
				</ButtonLink>
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
					{
						key: "resource_type",
						label: "Resource type",
						type: "select",
						value: typeFilter,
						options: typeOptions,
						placeholder: "Filter by resource type",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setNameFilter(Array.isArray(filters.name) ? filters.name : []);
					const slugs = Array.isArray(filters.resource_type)
						? filters.resource_type
						: [];
					if (onResourceTypesChange) onResourceTypesChange(slugs);
					else setLocalTypes(slugs);
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
