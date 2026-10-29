import { deleteProductResourcePermission } from "@/client";
import type {
	ProductResourcePermissionResponse,
	ProductResourcePermissionSearchRequest,
	ProductRoleResponse,
} from "@/client";
import { SortDirection } from "@/client";
import { searchProductResourcePermissionsOptions } from "@/client/@tanstack/react-query.gen";
import { DeleteProductResourcePermissionDialog } from "@/components/product/permissions/DeleteProductResourcePermissionDialog";
import { useScopeTypes } from "@/components/product/resource-types/ResourceTypeSelect";
import { typeName } from "@/components/product/resource-types/resource-type-scope";
import { useRolesHolding } from "@/components/product/roles/role-queries";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Box } from "@nanostackorg/design-system/layout/box";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Eye, PenLine, Plus } from "lucide-react";
import { useMemo, useRef, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<ProductResourcePermissionResponse>();
const LISTED_ROLES = 5;

type Holding =
	| { status: "loading" }
	| { status: "error" }
	| { status: "ready"; roles: Map<string, ProductRoleResponse[]> };

function HoldingRoles({
	holding,
	permission,
}: {
	holding: Holding;
	permission: string;
}) {
	if (holding.status === "loading")
		return (
			<Box className="w-24">
				<Skeleton />
				<Box as="span" className="sr-only">
					Loading roles
				</Box>
			</Box>
		);
	if (holding.status === "error")
		return (
			<Box as="span" className="text-muted-foreground text-sm">
				Couldn't load roles
			</Box>
		);
	const roles = holding.roles.get(permission.toLowerCase()) ?? [];
	if (roles.length === 0)
		return (
			<Box as="span" className="text-muted-foreground text-sm">
				No roles
			</Box>
		);
	const listed = roles.slice(0, LISTED_ROLES);
	return (
		<Box
			as="ul"
			className="flex max-w-[16rem] flex-wrap items-center gap-1"
			aria-label={`Roles holding ${permission}: ${roles.map((role) => role.name).join(", ")}`}
		>
			{listed.map((role) => (
				<Box as="li" key={role.id} className="max-w-full">
					<Badge variant="outline">
						<Box
							as="span"
							className="block max-w-[10rem] truncate"
							title={role.name}
						>
							{role.name}
						</Box>
					</Badge>
				</Box>
			))}
			{roles.length > LISTED_ROLES && (
				<Box as="li" className="text-muted-foreground text-xs">
					+{roles.length - LISTED_ROLES} more
				</Box>
			)}
		</Box>
	);
}

interface ProductResourcePermissionDatatableProps {
	productId: string;
	resourceTypes?: string[];
	onResourceTypesChange?: (slugs: string[]) => void;
}

export function ProductResourcePermissionDatatable({
	productId,
	resourceTypes = [],
	onResourceTypesChange,
}: ProductResourcePermissionDatatableProps) {
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
				sort_by: mapSortingToApiField<
					ProductResourcePermissionSearchRequest["sort_by"]
				>(sorting[0]?.id ?? "created_at", "created_at"),
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
		data: productPermissionData,
		isLoading,
		isFetching,
		error,
		refetch,
	} = useQuery({
		...searchProductResourcePermissionsOptions(searchOptions),
		placeholderData: keepPreviousData,
	});
	const { items = [], total = 0 } = productPermissionData ?? {};
	const pageNames = useMemo(() => items.map((item) => item.name), [items]);
	const rolesQuery = useRolesHolding(productId, pageNames);
	const holding = useMemo<Holding>(
		() =>
			rolesQuery.isError
				? { status: "error" }
				: rolesQuery.data
					? { status: "ready", roles: rolesQuery.data }
					: { status: "loading" },
		[rolesQuery.isError, rolesQuery.data],
	);
	const holdingRef = useRef(holding);
	holdingRef.current = holding;

	const columns = useMemo(
		() => [
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => (
					<Box
						as="span"
						className="block max-w-[16rem] whitespace-normal font-mono text-sm break-words"
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
			columnHelper.display({
				id: "roles",
				header: () => <span>Roles</span>,
				cell: ({ row }) => (
					<HoldingRoles
						holding={holdingRef.current}
						permission={row.original.name}
					/>
				),
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
	const createHref =
		typeFilter.length === 1
			? `${ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_NEW}?resource_type=${encodeURIComponent(typeFilter[0])}`
			: ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_NEW;

	return (
		<>
			<Box className="mb-4 flex items-center justify-between">
				<ButtonLink href={createHref} variant="solid" tone="brand">
					<Plus />
					Create Permission
				</ButtonLink>
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
