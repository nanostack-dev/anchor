import {
	type ProductRoleResponse,
	ProductRoleType,
	SortDirection,
} from "@/client";
import { searchOrganizationRolesOptions } from "@/client/@tanstack/react-query.gen";
import { useScopeTypes } from "@/components/product/resource-types/ResourceTypeSelect";
import { typeName } from "@/components/product/resource-types/resource-type-scope";
import { Badge } from "@nanostackorg/design-system/components/badge";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useMemo, useRef, useState } from "react";
import { AnchorDataTable } from "../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<ProductRoleResponse>();

type SortField = "id" | "name" | "created_at";

export function roleKind(role: ProductRoleResponse) {
	if (role.type === ProductRoleType.CUSTOM)
		return { label: "Custom", tone: "info" as const };
	return { label: "Managed", tone: "neutral" as const };
}

interface OrganizationRolesTableProps {
	productId: string;
	organizationId: string;
}

export function OrganizationRolesTable({
	productId,
	organizationId,
}: OrganizationRolesTableProps) {
	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "name", desc: false },
	]);
	const [typeFilter, setTypeFilter] = useState<string[]>([]);
	const scopeTypes = useScopeTypes(productId);
	const types = scopeTypes.data ?? [];
	const typesRef = useRef(types);
	typesRef.current = types;

	const searchOptions = useMemo(
		() => ({
			path: { product_id: productId, organization_id: organizationId },
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: (sorting[0]?.id === "created_at"
					? "created_at"
					: "name") as SortField,
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				filter: {
					resource_types: typeFilter.length > 0 ? typeFilter : undefined,
				},
			},
		}),
		[productId, organizationId, pagination, sorting, typeFilter],
	);
	const { data, isLoading, isFetching, error, refetch } = useQuery({
		...searchOrganizationRolesOptions(searchOptions),
		placeholderData: keepPreviousData,
	});
	const { items = [], total = 0 } = data ?? {};

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
			columnHelper.accessor("type", {
				header: () => <span>Type</span>,
				cell: (info) => {
					const kind = roleKind(info.row.original);
					return <Badge tone={kind.tone}>{kind.label}</Badge>;
				},
				enableSorting: false,
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
		],
		[],
	);
	const typeOptions = useMemo(
		() => types.map((type) => ({ label: type.name, value: type.slug })),
		[types],
	);

	return (
		<Stack space="md">
			<Text tone="muted">
				Read only. Managed roles are the ones your product defines; every
				organization holds the same ones. Custom roles belong to this
				organization: its admins change them from your product's own settings,
				through the organization roles API.
			</Text>
			<AnchorDataTable
				columns={columns}
				data={items}
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
				filters={[
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
					setTypeFilter(
						Array.isArray(filters.resource_type) ? filters.resource_type : [],
					);
				}}
				enableRowSelection={false}
			/>
		</Stack>
	);
}
