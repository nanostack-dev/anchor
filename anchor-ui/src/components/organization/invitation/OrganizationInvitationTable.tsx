import type { OrganizationInvitationResponse } from "@/client";
import { OrganizationInvitationStatus } from "@/client";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import dayjs from "dayjs";
import { useMemo } from "react";
import { DeleteOrganizationInvitationDialog } from "./DeleteOrganizationInvitationDialog";
import {
	InvitationStatusBadge,
	invitationStatusOptions,
} from "./InvitationStatusBadge";

const columnHelper = createColumnHelper<OrganizationInvitationResponse>();

export const EXPIRY_FORMAT = "MMM D, YYYY h:mm A";

type OrganizationInvitationTableProps = {
	invitations: OrganizationInvitationResponse[];
	total: number;
	roleNames: Record<string, string>;
	onDelete: (invitation: OrganizationInvitationResponse) => Promise<unknown>;
	onBulkDelete?: (
		invitation: OrganizationInvitationResponse,
	) => Promise<unknown>;
	onBulkDeleted?: () => Promise<unknown>;
	selectionScope?: string;
	statusFilter: OrganizationInvitationStatus[];
	onStatusFilterChange: (statuses: OrganizationInvitationStatus[]) => void;
	pagination: PaginationState;
	onPaginationChange: (
		updater: PaginationState | ((old: PaginationState) => PaginationState),
	) => void;
	sorting: SortingState;
	onSortingChange: (
		updater: SortingState | ((old: SortingState) => SortingState),
	) => void;
	loading?: boolean;
	error?: unknown;
	onRetry?: () => void;
};

const knownStatuses = new Set<string>(
	Object.values(OrganizationInvitationStatus),
);

export function OrganizationInvitationTable({
	invitations,
	total,
	roleNames,
	onDelete,
	onBulkDelete,
	onBulkDeleted,
	selectionScope,
	statusFilter,
	onStatusFilterChange,
	pagination,
	onPaginationChange,
	sorting,
	onSortingChange,
	loading,
	error,
	onRetry,
}: OrganizationInvitationTableProps) {
	const columns = useMemo(
		() => [
			columnHelper.accessor("status", {
				header: "Status",
				cell: (info) => <InvitationStatusBadge status={info.getValue()} />,
				enableSorting: false,
			}),
			columnHelper.accessor("email", {
				header: "Email",
				cell: (info) => <div className="font-medium">{info.getValue()}</div>,
			}),
			columnHelper.accessor("role_id", {
				header: "Role",
				cell: (info) => roleNames[info.getValue()] ?? info.getValue(),
				enableSorting: false,
			}),
			columnHelper.accessor("expires_at", {
				header: "Expires",
				cell: (info) => (
					<div className="text-muted-foreground">
						{dayjs(info.getValue()).format(EXPIRY_FORMAT)}
					</div>
				),
			}),
			columnHelper.display({
				id: "actions",
				header: "Actions",
				cell: ({ row }) => (
					<div className="flex gap-2">
						<DeleteOrganizationInvitationDialog
							email={row.original.email}
							onConfirm={() => onDelete(row.original)}
						/>
					</div>
				),
			}),
		],
		[roleNames, onDelete],
	);

	return (
		<AnchorDataTable<OrganizationInvitationResponse>
			columns={columns}
			data={invitations}
			total={total}
			pagination={pagination}
			onPaginationChange={onPaginationChange}
			sorting={sorting}
			onSortingChange={onSortingChange}
			loading={loading}
			resourceName="invitations"
			error={error}
			onRetry={onRetry}
			getRowId={(row) => row.id}
			getRowLabel={(row) => row.email}
			selectionScope={selectionScope}
			bulkActions={[
				{
					id: "delete",
					label: "Delete selected",
					description: "This permanently deletes the selected invitations.",
					destructive: true,
					removesRows: true,
					run: onBulkDelete ?? onDelete,
				},
			]}
			onBulkActionComplete={onBulkDeleted}
			filters={[
				{
					key: "status",
					label: "Status",
					type: "select",
					value: statusFilter,
					options: invitationStatusOptions,
					placeholder: "Filter by status",
				},
			]}
			onFiltersChange={(filters) => {
				const selected = Array.isArray(filters.status) ? filters.status : [];
				onStatusFilterChange(
					selected.filter((status): status is OrganizationInvitationStatus =>
						knownStatuses.has(status),
					),
				);
			}}
		/>
	);
}
