import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

import type {
	Options,
	ProductWorkspaceResponse,
	ProductWorkspaceSearchRequest,
	SearchOrganizationWorkspacesData,
} from "@/client";
import { SortDirection } from "@/client";
import { searchOrganizationWorkspacesOptions } from "@/client/@tanstack/react-query.gen";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";

const columnHelper = createColumnHelper<ProductWorkspaceResponse>();

interface OrganizationWorkspaceDatatableProps {
	organizationId: string;
}

export function OrganizationWorkspaceDatatable({
	organizationId,
}: OrganizationWorkspaceDatatableProps) {
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
	const debouncedNames = useDebounce(nameFilter, 100);
	const [idFilter, setIdFilter] = useState<string[]>([]);
	const debouncedIds = useDebounce(idFilter, 100);

	const queryOptions =
		useMemo<Options<SearchOrganizationWorkspacesData> | null>(() => {
			if (!currentProduct?.id || !organizationId) {
				return null;
			}

			return {
				path: {
					product_id: currentProduct.id,
					organization_id: organizationId,
				},
				body: {
					pagination: {
						limit: pagination.pageSize,
						offset: pagination.pageIndex * pagination.pageSize,
					},
					sort_by: mapSortingToApiField<
						ProductWorkspaceSearchRequest["sort_by"]
					>(sorting[0]?.id, "created_at"),
					sort_direction: sorting[0]?.desc
						? SortDirection.DESC
						: SortDirection.ASC,
					full_text_search: debouncedFullTextSearch || undefined,
					filter: {
						names: debouncedNames.length > 0 ? debouncedNames : undefined,
						ids: debouncedIds.length > 0 ? debouncedIds : undefined,
					},
				},
			};
		}, [
			currentProduct?.id,
			organizationId,
			pagination,
			sorting,
			debouncedFullTextSearch,
			debouncedNames,
			debouncedIds,
		]);

	const { data, isLoading, error, refetch } = useQuery({
		...searchOrganizationWorkspacesOptions(
			queryOptions ?? {
				path: { product_id: "", organization_id: "" },
				body: {},
			},
		),
		placeholderData: keepPreviousData,
		enabled: !!queryOptions,
	});

	const items = data?.items ?? [];

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
			columnHelper.accessor("id", {
				header: "ID",
				cell: (info) => (
					<span className="block max-w-[180px] truncate font-mono text-sm text-muted-foreground">
						{info.getValue()}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: "Created",
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
			columnHelper.accessor("updated_at", {
				header: "Updated",
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
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

	const idOptions = useMemo(
		() => items.map((item) => ({ label: item.id, value: item.id })),
		[items],
	);

	if (!organizationId) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyTitle>Workspaces</EmptyTitle>
					<EmptyDescription>
						Select an organization to view its workspaces.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<>
			<AnchorDataTable<ProductWorkspaceResponse>
				columns={columns}
				data={items}
				loading={isLoading}
				resourceName="workspaces"
				error={error}
				onRetry={() => {
					void refetch();
				}}
				total={data?.total ?? 0}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={setFullTextSearch}
				fullTextSearchPlaceHolder="Search workspaces"
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
		</>
	);
}
