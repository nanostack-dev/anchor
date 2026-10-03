import type {
	OrganizationInvitationResponse,
	OrganizationInvitationSearchRequest,
	OrganizationInvitationStatus,
	SearchOrganizationInvitationsData,
} from "@/client";
import { type Options, SortDirection } from "@/client";
import {
	deleteOrganizationInvitationMutation,
	searchOrganizationInvitationsOptions,
	searchOrganizationInvitationsQueryKey,
	searchProductRolesOptions,
} from "@/client/@tanstack/react-query.gen";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import {
	keepPreviousData,
	useMutation,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { useMemo, useState } from "react";
import { OrganizationInvitationTable } from "./OrganizationInvitationTable";

const ROLE_LOOKUP_LIMIT = 100;

type OrganizationInvitationDatatableProps = {
	organizationId: string;
};

export function OrganizationInvitationDatatable({
	organizationId,
}: OrganizationInvitationDatatableProps) {
	const { currentProduct } = useProduct();
	const queryClient = useQueryClient();
	const productId = currentProduct?.id as string;

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "expires_at", desc: true },
	]);
	const [statusFilter, setStatusFilter] = useState<
		OrganizationInvitationStatus[]
	>([]);

	const searchOptions: Options<SearchOrganizationInvitationsData> = {
		path: { product_id: productId, organization_id: organizationId },
		body: {
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<
				OrganizationInvitationSearchRequest["sort_by"]
			>(sorting[0]?.id ?? "expires_at", "expires_at"),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			filter: statusFilter.length > 0 ? { statuses: statusFilter } : undefined,
		},
	};

	const invitationsQuery = useQuery({
		...searchOrganizationInvitationsOptions(searchOptions),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct?.id && !!organizationId,
	});

	const rolesQuery = useQuery({
		...searchProductRolesOptions({
			path: { product_id: productId },
			body: { pagination: { limit: ROLE_LOOKUP_LIMIT, offset: 0 } },
		}),
		enabled: !!currentProduct?.id,
	});

	const roleNames = useMemo(
		() =>
			Object.fromEntries(
				(rolesQuery.data?.items ?? []).map((role) => [role.id, role.name]),
			),
		[rolesQuery.data],
	);

	const refreshInvitations = () =>
		queryClient.invalidateQueries({
			queryKey: searchOrganizationInvitationsQueryKey(searchOptions),
		});

	const deleteMutation = useMutation(deleteOrganizationInvitationMutation());

	const handleDelete = async (invitation: OrganizationInvitationResponse) => {
		await deleteMutation.mutateAsync({
			path: {
				product_id: productId,
				organization_id: organizationId,
				invitation_id: invitation.id,
			},
		});
		const deletedLastRowOfPage =
			(invitationsQuery.data?.items.length ?? 0) === 1 &&
			pagination.pageIndex > 0;
		if (deletedLastRowOfPage) {
			setPagination((current) => ({
				...current,
				pageIndex: current.pageIndex - 1,
			}));
		}
		await refreshInvitations();
	};

	return (
		<OrganizationInvitationTable
			invitations={invitationsQuery.data?.items ?? []}
			total={invitationsQuery.data?.total ?? 0}
			roleNames={roleNames}
			onDelete={handleDelete}
			selectionScope={`${productId}:${organizationId}`}
			onBulkDelete={(invitation) =>
				deleteMutation.mutateAsync({
					path: {
						product_id: productId,
						organization_id: organizationId,
						invitation_id: invitation.id,
					},
				})
			}
			onBulkDeleted={() => invitationsQuery.refetch({ throwOnError: true })}
			statusFilter={statusFilter}
			onStatusFilterChange={setStatusFilter}
			pagination={pagination}
			onPaginationChange={setPagination}
			sorting={sorting}
			onSortingChange={setSorting}
			loading={invitationsQuery.isLoading || invitationsQuery.isFetching}
			error={invitationsQuery.error}
			onRetry={() => {
				void invitationsQuery.refetch();
			}}
		/>
	);
}
