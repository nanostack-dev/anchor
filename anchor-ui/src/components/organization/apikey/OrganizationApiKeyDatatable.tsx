import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

import {
	type Options,
	type OrganizationApiKeyResponse,
	type OrganizationApiKeySearchRequest,
	OrganizationApiKeyStatus,
	type SearchOrganizationApiKeysData,
	SortDirection,
} from "@/client";
import { searchOrganizationApiKeysOptions } from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@/components/ui/card";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<OrganizationApiKeyResponse>();

type OrganizationApiKeyFilters = {
	name: string[];
	status: OrganizationApiKeyStatus[];
};

interface OrganizationApiKeyDatatableProps {
	organizationId: string;
}

export function OrganizationApiKeyDatatable({
	organizationId,
}: OrganizationApiKeyDatatableProps) {
	const { currentProduct } = useProduct();

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
	const debouncedNameFilter = useDebounce(nameFilter, 100);
	const [statusFilter, setStatusFilter] = useState<OrganizationApiKeyStatus[]>(
		[],
	);
	const debouncedStatusFilter = useDebounce(statusFilter, 100);

	const requestBody: OrganizationApiKeySearchRequest = useMemo(
		() => ({
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<OrganizationApiKeySearchRequest["sort_by"]>(
				sorting[0]?.id,
				"created_at",
			),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			full_text_search: debouncedFullTextSearch || undefined,
			filter: {
				names: debouncedNameFilter.length > 0 ? debouncedNameFilter : undefined,
				status:
					debouncedStatusFilter.length > 0 ? debouncedStatusFilter : undefined,
			},
		}),
		[
			pagination,
			sorting,
			debouncedFullTextSearch,
			debouncedNameFilter,
			debouncedStatusFilter,
		],
	);

	const queryOptions: Options<SearchOrganizationApiKeysData> = {
		path: {
			product_id: currentProduct?.id as string,
			organization_id: organizationId,
		},
		body: requestBody,
	};

	const {
		data: apiKeyData,
		isLoading,
		error,
	} = useQuery({
		...searchOrganizationApiKeysOptions(queryOptions),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct?.id && !!organizationId,
	});

	const items = apiKeyData?.items ?? [];
	const total = apiKeyData?.total ?? 0;

	const columns = useMemo(
		() => [
			columnHelper.accessor("name", {
				header: "Name",
				cell: (info) => <div className="font-medium">{info.getValue()}</div>,
				enableSorting: true,
			}),
			columnHelper.accessor("description", {
				header: "Description",
				cell: (info) => (
					<span className="block max-w-[260px] truncate text-sm text-muted-foreground">
						{info.getValue() || "No description"}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("obfuscated_value", {
				header: "Key",
				cell: (info) => <code className="text-xs">{info.getValue()}</code>,
				enableSorting: false,
			}),
			columnHelper.accessor("permissions", {
				header: "Permissions",
				cell: (info) => {
					const permissions = info.getValue();

					if (permissions.length === 0) {
						return (
							<span className="text-sm text-muted-foreground">
								No permissions
							</span>
						);
					}

					const visiblePermissions = permissions.slice(0, 2);
					const remainingCount = permissions.length - visiblePermissions.length;

					return (
						<div className="flex flex-wrap gap-1">
							{visiblePermissions.map((permission) => (
								<Badge key={permission.permission_name} variant="secondary">
									{permission.permission_name}
								</Badge>
							))}
							{remainingCount > 0 ? (
								<Badge variant="outline">+{remainingCount} more</Badge>
							) : null}
						</div>
					);
				},
				enableSorting: false,
			}),
			columnHelper.accessor("status", {
				header: "Status",
				cell: (info) => {
					const status = info.getValue();
					const label =
						status === OrganizationApiKeyStatus.ACTIVE ? "Active" : "Inactive";

					return (
						<Badge
							variant={
								status === OrganizationApiKeyStatus.ACTIVE
									? "default"
									: "secondary"
							}
						>
							{label}
						</Badge>
					);
				},
				enableSorting: true,
			}),
			columnHelper.accessor("last_used_at", {
				header: "Last Used",
				cell: (info) => {
					const value = info.getValue();
					return value ? dayjs(value).format("MMM D, YYYY") : "Never";
				},
				enableSorting: true,
			}),
			columnHelper.accessor("created_at", {
				header: "Created",
				cell: (info) => dayjs(info.getValue()).format("MMM D, YYYY"),
				enableSorting: true,
			}),
		],
		[],
	);

	const nameOptions = useMemo(
		() =>
			Array.from(new Set(items.map((item) => item.name))).map((name) => ({
				label: name,
				value: name,
			})),
		[items],
	);

	const statusOptions = useMemo(
		() => [
			{ label: "Active", value: OrganizationApiKeyStatus.ACTIVE },
			{ label: "Inactive", value: OrganizationApiKeyStatus.INACTIVE },
		],
		[],
	);

	return (
		<Card>
			<CardHeader>
				<CardTitle>Organization API Keys</CardTitle>
				<CardDescription>
					View API keys issued for this organization. Creation and updates stay
					in product-side apps.
				</CardDescription>
			</CardHeader>
			<CardContent>
				<AnchorDataTable<
					OrganizationApiKeyResponse,
					OrganizationApiKeyFilters
				>
					columns={columns}
					data={items}
					total={total}
					pagination={pagination}
					onPaginationChange={setPagination}
					sorting={sorting}
					onSortingChange={setSorting}
					fullTextSearch={fullTextSearch}
					onFullTextSearchChange={setFullTextSearch}
					fullTextSearchPlaceHolder="Search organization API keys"
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
							key: "status",
							label: "Status",
							type: "select",
							value: statusFilter,
							options: statusOptions,
							placeholder: "Filter by status",
							multi: true,
						},
					]}
					onFiltersChange={(filters) => {
						setPagination((prev) => ({ ...prev, pageIndex: 0 }));
						setNameFilter(Array.isArray(filters.name) ? filters.name : []);
						setStatusFilter(
							Array.isArray(filters.status) ? filters.status : [],
						);
					}}
					loading={isLoading}
					enableRowSelection={false}
				/>
				{error ? (
					<div className="mt-4 text-sm text-destructive">
						Failed to load organization API keys: {error.message}
					</div>
				) : null}
			</CardContent>
		</Card>
	);
}
