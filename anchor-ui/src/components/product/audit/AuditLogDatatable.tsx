import {
	AuditLogActorType,
	AuditLogOutcome,
	type AuditLogResponse,
	type Options,
	type SearchAuditLogsData,
	SortDirection,
} from "@/client";
import { searchAuditLogsOptions } from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { DateRangeFilter } from "@/components/common/datatable/DateRangeFilter";
import { AuditLogDetailSheet } from "@/components/product/audit/AuditLogDetailSheet";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<AuditLogResponse>();

type AuditLogFilters = {
	action: string[];
	actor_type: AuditLogActorType[];
	outcome: AuditLogOutcome[];
};

const ACTION_OPTIONS = [
	"product.created",
	"product.updated",
	"product.deleted",
	"organization.created",
	"organization.updated",
	"organization.deleted",
	"organization.member_added",
	"organization.member_removed",
	"organization.member_role_updated",
	"workspace.created",
	"workspace.updated",
	"workspace.deleted",
	"organization_api_key.created",
	"organization_api_key.updated",
	"organization_api_key.deleted",
	"product_api_key.created",
	"product_api_key.updated",
	"product_api_key.deleted",
	"role.created",
	"role.updated",
	"role.deleted",
	"role.permission_assigned",
	"role.permission_unassigned",
	"permission.created",
	"permission.updated",
	"permission.deleted",
	"resource_permission.created",
	"resource_permission.updated",
	"resource_permission.deleted",
	"product_user.created",
	"product_user.deleted",
].map((action) => ({ label: action, value: action }));

const ACTOR_TYPE_LABELS: Record<AuditLogActorType, string> = {
	[AuditLogActorType.PLATFORM_USER]: "Platform user",
	[AuditLogActorType.PRODUCT_API_KEY]: "API key",
	[AuditLogActorType.SYSTEM]: "System",
};

interface AuditLogDatatableProps {
	productId: string;
}

export function AuditLogDatatable({ productId }: AuditLogDatatableProps) {
	const [selectedEntry, setSelectedEntry] = useState<AuditLogResponse | null>(
		null,
	);

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 20,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "created_at", desc: true },
	]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const debouncedFullTextSearch = useDebounce(fullTextSearch, 300);

	const [actionFilter, setActionFilter] = useState<string[]>([]);
	const debouncedActions = useDebounce(actionFilter, 100);
	const [actorTypeFilter, setActorTypeFilter] = useState<AuditLogActorType[]>(
		[],
	);
	const debouncedActorTypes = useDebounce(actorTypeFilter, 100);
	const [outcomeFilter, setOutcomeFilter] = useState<AuditLogOutcome[]>([]);
	const debouncedOutcome = useDebounce(outcomeFilter, 100);
	const [dateRange, setDateRange] = useState<{ from?: string; to?: string }>(
		{},
	);
	const debouncedDateRange = useDebounce(dateRange, 100);

	const searchAuditLogsParams = useMemo<Options<SearchAuditLogsData>>(
		() => ({
			path: { product_id: productId },
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: "created_at",
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					actions: debouncedActions.length > 0 ? debouncedActions : undefined,
					actor_types:
						debouncedActorTypes.length > 0 ? debouncedActorTypes : undefined,
					outcome:
						debouncedOutcome.length === 1 ? debouncedOutcome[0] : undefined,
					created_after: debouncedDateRange.from
						? new Date(`${debouncedDateRange.from}T00:00:00`).toISOString()
						: undefined,
					created_before: debouncedDateRange.to
						? new Date(`${debouncedDateRange.to}T23:59:59.999`).toISOString()
						: undefined,
				},
			},
		}),
		[
			productId,
			pagination,
			sorting,
			debouncedFullTextSearch,
			debouncedActions,
			debouncedActorTypes,
			debouncedOutcome,
			debouncedDateRange,
		],
	);

	const {
		data: auditLogData,
		isLoading,
		error,
	} = useQuery({
		...searchAuditLogsOptions(searchAuditLogsParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total = 0 } = auditLogData ?? {};

	const columns = useMemo(
		() => [
			columnHelper.accessor("created_at", {
				header: () => <span>Time</span>,
				cell: (info) => (
					<span className="whitespace-nowrap">
						{dayjs(info.getValue()).format("D MMM YYYY H:mm:ss")}
					</span>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("action", {
				header: () => <span>Action</span>,
				cell: (info) => (
					<code className="rounded bg-muted px-1.5 py-0.5 text-xs">
						{info.getValue()}
					</code>
				),
				enableSorting: false,
			}),
			columnHelper.display({
				id: "actor",
				header: () => <span>Actor</span>,
				cell: ({ row }) => {
					const { actor_name, actor_id, actor_type } = row.original;
					return (
						<div className="flex flex-col">
							<span className="text-sm">
								{actor_name || actor_id || "System"}
							</span>
							<span className="text-xs text-muted-foreground">
								{ACTOR_TYPE_LABELS[actor_type]}
							</span>
						</div>
					);
				},
			}),
			columnHelper.display({
				id: "target",
				header: () => <span>Target</span>,
				cell: ({ row }) => {
					const { target_name, target_id, target_type } = row.original;
					return (
						<div className="flex flex-col">
							<span className="text-sm max-w-[220px] truncate">
								{target_name || target_id || "—"}
							</span>
							<span className="text-xs text-muted-foreground">
								{target_type}
							</span>
						</div>
					);
				},
			}),
			columnHelper.accessor("outcome", {
				header: () => <span>Outcome</span>,
				cell: (info) => (
					<StatusBadge
						tone={
							info.getValue() === AuditLogOutcome.SUCCESS
								? "success"
								: "destructive"
						}
					>
						{info.getValue()}
					</StatusBadge>
				),
				enableSorting: false,
			}),
		],
		[],
	);

	const actorTypeOptions = useMemo(
		() =>
			Object.values(AuditLogActorType).map((value) => ({
				label: ACTOR_TYPE_LABELS[value],
				value,
			})),
		[],
	);

	const outcomeOptions = useMemo(
		() =>
			Object.values(AuditLogOutcome).map((value) => ({
				label: value === AuditLogOutcome.SUCCESS ? "Success" : "Failure",
				value,
			})),
		[],
	);

	return (
		<>
			<AnchorDataTable<AuditLogResponse, AuditLogFilters>
				columns={columns}
				data={items}
				loading={isLoading}
				total={total}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={setFullTextSearch}
				fullTextSearchPlaceHolder="Search actions, actors, targets"
				filters={[
					{
						key: "action",
						label: "Action",
						type: "select",
						value: actionFilter,
						options: ACTION_OPTIONS,
						placeholder: "Filter by action",
						multi: true,
					},
					{
						key: "actor_type",
						label: "Actor type",
						type: "select",
						value: actorTypeFilter,
						options: actorTypeOptions,
						placeholder: "Filter by actor type",
						multi: true,
					},
					{
						key: "outcome",
						label: "Outcome",
						type: "select",
						value: outcomeFilter,
						options: outcomeOptions,
						placeholder: "Filter by outcome",
						multi: false,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setActionFilter(Array.isArray(filters.action) ? filters.action : []);
					setActorTypeFilter(
						Array.isArray(filters.actor_type) ? filters.actor_type : [],
					);
					setOutcomeFilter(
						Array.isArray(filters.outcome) ? filters.outcome : [],
					);
				}}
				enableRowSelection={false}
				onRowClick={(row) => setSelectedEntry(row)}
			>
				<DateRangeFilter
					label="Date"
					value={dateRange}
					onChange={(value) => {
						setPagination((p) => ({ ...p, pageIndex: 0 }));
						setDateRange(value);
					}}
				/>
			</AnchorDataTable>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load audit logs: {error.message}
				</div>
			)}
			<AuditLogDetailSheet
				entry={selectedEntry}
				onClose={() => setSelectedEntry(null)}
			/>
		</>
	);
}
