import type {
	Options,
	PlatformInvitationResponse,
	PlatformInvitationSearchRequest,
	SearchPlatformInvitationsData,
} from "@/client";
import { SortDirection } from "@/client";
import {
	searchPlatformInvitationsOptions,
	searchPlatformInvitationsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { generateInvitationLink } from "@/components/platform/invitationUtils";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import {
	keepPreviousData,
	useQuery,
	useQueryClient,
} from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { Copy } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../common/datatable/AnchorDataTable";
import { Button } from "../ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "../ui/tooltip";
import { PlatformAddInvitationDialog } from "./PlatformAddInvitationDialog";
import { PlatformDeleteInvitationDialog } from "./PlatformDeleteInvitationDialog";

const columnHelper = createColumnHelper<PlatformInvitationResponse>();

export function PlatformInvitationDatatable() {
	const queryClient = useQueryClient();
	const [total, setTotal] = useState(0);

	const [inviteOpen, setInviteOpen] = useState(false);

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "created_at", desc: true },
	]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const debouncedFullTextSearch = useDebounce(fullTextSearch, 300);

	const [emailFilter, setEmailFilter] = useState<string[]>([]);
	const debouncedEmail = useDebounce(emailFilter, 100);

	const [searchParams, setSearchParams] = useState<
		Options<SearchPlatformInvitationsData>
	>({
		body: {
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<PlatformInvitationSearchRequest["sort_by"]>(
				sorting[0]?.id,
				"created_at",
			),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			full_text_search: debouncedFullTextSearch || undefined,
			filter:
				debouncedEmail && debouncedEmail.length > 0
					? { emails: debouncedEmail }
					: undefined,
		},
	});

	const {
		data: invitationData,
		isLoading,
		error,
	} = useQuery({
		...searchPlatformInvitationsOptions(searchParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = invitationData ?? {};
	useEffect(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	// Update search parameters when pagination, sorting, or filters change
	useEffect(() => {
		setSearchParams({
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<
					PlatformInvitationSearchRequest["sort_by"]
				>(sorting[0]?.id ?? "created_at", "created_at"),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter:
					debouncedEmail && debouncedEmail.length > 0
						? { emails: debouncedEmail }
						: undefined,
			},
		});
	}, [pagination, sorting, debouncedFullTextSearch, debouncedEmail]);

	const handleCopy = useCallback(
		(code: string, tenantId: string, email: string) => {
			void navigator.clipboard.writeText(
				generateInvitationLink(tenantId, email, code),
			);
		},
		[],
	);

	const columns = useMemo(
		() => [
			columnHelper.accessor("email", {
				header: () => <span>Email</span>,
				cell: (info) => info.getValue(),
				enableSorting: true,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
			columnHelper.accessor("updated_at", {
				header: () => <span>Updated At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<div className={"flex gap-2"}>
						<Tooltip>
							<TooltipTrigger
								render={
									<Button
										size="icon"
										variant={"outline"}
										onClick={() =>
											handleCopy(
												row.original.code,
												row.original.tenant_id,
												row.original.email,
											)
										}
									/>
								}
							>
								<Copy />
							</TooltipTrigger>
							<TooltipContent>Copy invitation link</TooltipContent>
						</Tooltip>
						<PlatformDeleteInvitationDialog
							invitationId={row.original.id}
							onDeleted={async () => {
								await queryClient.invalidateQueries({
									queryKey: searchPlatformInvitationsQueryKey(searchParams),
								});
								setPagination((p) => ({ ...p }));
							}}
							trigger={
								<Tooltip>
									<TooltipTrigger
										render={
											<Button size="icon" variant="outlineDestructive" />
										}
									>
										<span className="sr-only">Delete invitation</span>
											<svg
												width="1em"
												height="1em"
												viewBox="0 0 24 24"
												fill="none"
												stroke="currentColor"
												strokeWidth="2"
												strokeLinecap="round"
												strokeLinejoin="round"
												className="lucide lucide-trash"
											>
												<title>Delete invitation</title>
												<polyline points="3 6 5 6 21 6" />
												<path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v2" />
												<line x1="10" x2="10" y1="11" y2="17" />
												<line x1="14" x2="14" y1="11" y2="17" />
											</svg>
									</TooltipTrigger>
									<TooltipContent>Delete invitation</TooltipContent>
								</Tooltip>
							}
						/>
					</div>
				),
			}),
		],
		[handleCopy, queryClient, searchParams],
	);

	// Build email options from current data
	const emailOptions = useMemo(
		() =>
			Array.from(
				new Set(
					(items as PlatformInvitationResponse[]).map((item) => item.email),
				),
			).map((email) => ({ label: email, value: email })),
		[items],
	);

	return (
		<>
			<AnchorDataTable
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
				fullTextSearchPlaceHolder="Search invitations"
				filters={[
					{
						key: "email",
						label: "Email",
						type: "select",
						value: emailFilter,
						options: emailOptions,
						placeholder: "Filter by email",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setEmailFilter(Array.isArray(filters.email) ? filters.email : []);
				}}
				enableRowSelection={false}
			>
				<Button variant="outline" onClick={() => setInviteOpen(true)}>
					Add Invitation
				</Button>
			</AnchorDataTable>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load invitations: {error.message}
				</div>
			)}
			<PlatformAddInvitationDialog
				open={inviteOpen}
				onOpenChange={setInviteOpen}
				onSuccess={async () => {
					await queryClient.invalidateQueries({
						queryKey: searchPlatformInvitationsQueryKey(searchParams),
					});
					setPagination((p) => ({ ...p }));
				}}
			/>
		</>
	);
}
