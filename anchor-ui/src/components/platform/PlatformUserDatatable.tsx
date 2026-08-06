import {
	type Options,
	type PlatformUserResponse,
	PlatformUserRole,
	type PlatformUserSearchRequest,
	type SearchPlatformUsersData,
	SortDirection,
} from "@/client";
import {
	searchPlatformUsersOptions,
	searchPlatformUsersQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { Badge } from "@/components/ui/badge";
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
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../common/datatable/AnchorDataTable";
import { PlatformDeleteUserDialog } from "./PlatformDeleteUserDialog";

const columnHelper = createColumnHelper<PlatformUserResponse>();

export function PlatformUserDatatable() {
	const queryClient = useQueryClient();
	const [total, setTotal] = useState(0);

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

	const [roleFilter, setRoleFilter] = useState<PlatformUserRole[]>([]);
	const debouncedRole = useDebounce(roleFilter, 100);

	const [searchParams, setSearchParams] = useState<
		Options<SearchPlatformUsersData>
	>({
		body: {
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<PlatformUserSearchRequest["sort_by"]>(
				sorting[0]?.id,
				"created_at",
			),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			full_text_search: debouncedFullTextSearch || undefined,
			filter: {
				emails: debouncedEmail.length > 0 ? debouncedEmail : undefined,
				roles: debouncedRole.length > 0 ? debouncedRole : undefined,
			},
		},
	});

	const {
		data: userData,
		isLoading,
		error,
		refetch,
	} = useQuery({
		...searchPlatformUsersOptions(searchParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = userData ?? {};
	useMemo(() => {
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
				sort_by: mapSortingToApiField<PlatformUserSearchRequest["sort_by"]>(
					sorting[0]?.id ?? "created_at",
					"created_at",
				),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					emails: debouncedEmail.length > 0 ? debouncedEmail : undefined,
					roles: debouncedRole.length > 0 ? debouncedRole : undefined,
				},
			},
		});
	}, [
		pagination,
		sorting,
		debouncedFullTextSearch,
		debouncedEmail,
		debouncedRole,
	]);

	const columns = useMemo(
		() => [
			columnHelper.accessor("email", {
				header: () => <span>Email</span>,
				cell: (info) => info.getValue(),
				enableSorting: true,
			}),
			columnHelper.accessor("role", {
				header: () => <span>Role</span>,
				cell: (info) => {
					const role = info.getValue();
					return (
						<Badge
							variant={
								role === PlatformUserRole.OWNER ? "default" : "secondary"
							}
						>
							{role}
						</Badge>
					);
				},
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
						<PlatformDeleteUserDialog
							userId={row.original.id}
							userEmail={row.original.email}
							disabled={row.original.role === PlatformUserRole.OWNER}
							onDeleted={async () => {
								await queryClient.invalidateQueries({
									queryKey: searchPlatformUsersQueryKey(searchParams),
								});
								setPagination((p) => ({ ...p }));
							}}
						/>
					</div>
				),
			}),
		],
		[queryClient, searchParams],
	);

	// Build email options from current data
	const emailOptions = useMemo(
		() =>
			Array.from(
				new Set((items as PlatformUserResponse[]).map((item) => item.email)),
			).map((email) => ({ label: email, value: email })),
		[items],
	);

	// Build role options from current data
	const roleOptions = useMemo(
		() =>
			Array.from(
				new Set((items as PlatformUserResponse[]).map((item) => item.role)),
			).map((role) => ({ label: role, value: role })),
		[items],
	);

	return (
		<>
			<AnchorDataTable
				columns={columns}
				data={items}
				loading={isLoading}
				resourceName="users"
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
				fullTextSearchPlaceHolder="Search users"
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
					{
						key: "role",
						label: "Role",
						type: "select",
						value: roleFilter,
						options: roleOptions,
						placeholder: "Filter by role",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setEmailFilter(Array.isArray(filters.email) ? filters.email : []);
					setRoleFilter(
						Array.isArray(filters.role)
							? (filters.role as PlatformUserRole[])
							: [],
					);
				}}
				enableRowSelection={false}
			/>
		</>
	);
}
