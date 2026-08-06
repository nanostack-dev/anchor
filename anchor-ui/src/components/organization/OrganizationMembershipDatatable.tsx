import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

import type {
	OrganizationMemberResponse,
	OrganizationMemberSearchRequest,
	SearchOrganizationMembersData,
} from "@/client";
import { type Options, SortDirection } from "@/client";
import { searchOrganizationMembersOptions } from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyTitle,
} from "@/components/ui/empty";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { AnchorDataTable } from "../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<OrganizationMemberResponse>();

interface OrganizationMembershipDatatableProps {
	organizationId: string;
}

export function OrganizationMembershipDatatable({
	organizationId,
}: OrganizationMembershipDatatableProps) {
	const { currentProduct } = useProduct();

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "joined_at", desc: true },
	]);
	const [globalFilter, setGlobalFilter] = useState("");
	const debouncedGlobalFilter = useDebounce(globalFilter, 300);

	const requestBody: OrganizationMemberSearchRequest = useMemo(() => {
		return {
			limit: pagination.pageSize,
			offset: pagination.pageIndex * pagination.pageSize,
			query: debouncedGlobalFilter || undefined,
			sort_by: mapSortingToApiField<OrganizationMemberSearchRequest["sort_by"]>(
				sorting[0]?.id,
				"joined_at",
			),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
		};
	}, [pagination, sorting, debouncedGlobalFilter]);

	const queryOptions: Options<SearchOrganizationMembersData> = {
		path: {
			product_id: currentProduct?.id as string,
			organization_id: organizationId as string,
		},
		body: requestBody,
	};

	const { data, isLoading, error, refetch } = useQuery({
		...searchOrganizationMembersOptions(queryOptions),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct?.id && !!organizationId,
	});

	const columns = useMemo(
		() => [
			columnHelper.accessor("email", {
				header: "Email",
				cell: (info) => (
					<div className="font-medium">{info.getValue() || "No email"}</div>
				),
			}),
			columnHelper.accessor("name", {
				header: "Name",
				cell: (info) => (
					<div className="text-muted-foreground">
						{info.getValue() || "No name"}
					</div>
				),
			}),
			columnHelper.accessor("role.name", {
				header: "Role",
				cell: (info) => <Badge variant="secondary">{info.getValue()}</Badge>,
			}),
			columnHelper.accessor("joined_at", {
				header: "Joined",
				cell: (info) => (
					<div className="text-muted-foreground">
						{dayjs(info.getValue()).format("MMM D, YYYY")}
					</div>
				),
			}),
		],
		[],
	);

	if (!organizationId) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyTitle>Organization Memberships</EmptyTitle>
					<EmptyDescription>
						Select an organization to view its members.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	return (
		<AnchorDataTable<OrganizationMemberResponse>
			columns={columns}
			data={data?.items ?? []}
			total={data?.total ?? 0}
			pagination={pagination}
			onPaginationChange={setPagination}
			sorting={sorting}
			onSortingChange={setSorting}
			fullTextSearch={globalFilter}
			onFullTextSearchChange={setGlobalFilter}
			loading={isLoading}
			resourceName="members"
			error={error}
			onRetry={() => {
				void refetch();
			}}
			fullTextSearchPlaceHolder="Search by email or name..."
		/>
	);
}
