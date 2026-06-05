import {
	type Options,
	type ProductUserResponse,
	type ProductUserSearchRequest,
	ProductUserStatus,
	type SearchProductUsersData,
	SortDirection,
} from "@/client";
import { searchProductUsersOptions } from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { useProduct } from "@/context/product/ProductContext";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

const columnHelper = createColumnHelper<ProductUserResponse>();

export function ProductUserDatatable() {
	const { currentProduct } = useProduct();
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
	const debouncedEmails = useDebounce(emailFilter, 100);

	const [statusFilter, setStatusFilter] = useState<ProductUserStatus[]>([]);
	const debouncedStatuses = useDebounce(statusFilter, 100);

	const [idFilter, setIdFilter] = useState<string[]>([]);
	const debouncedIds = useDebounce(idFilter, 100);

	const searchProductUsersOptionsParams = useMemo<
		Options<SearchProductUsersData>
	>(
		() => ({
			path: {
				product_id: currentProduct?.id ?? "",
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<ProductUserSearchRequest["sort_by"]>(
					sorting[0]?.id,
					"created_at",
				),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					emails: debouncedEmails.length > 0 ? debouncedEmails : undefined,
					statuses:
						debouncedStatuses.length > 0 ? debouncedStatuses : undefined,
					ids: debouncedIds.length > 0 ? debouncedIds : undefined,
				},
			},
		}),
		[
			currentProduct?.id,
			pagination,
			sorting,
			debouncedFullTextSearch,
			debouncedEmails,
			debouncedStatuses,
			debouncedIds,
		],
	);

	const {
		data: productUserData,
		isLoading,
		error,
	} = useQuery({
		...searchProductUsersOptions(searchProductUsersOptionsParams),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct?.id,
	});

	const { items = [], total: fetchedTotal = 0 } = productUserData ?? {};
	useEffect(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	const columns = useMemo(
		() => [
			columnHelper.accessor("email", {
				header: () => <span>Email</span>,
				cell: (info) => (
					<span className="text-sm font-medium">{info.getValue()}</span>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => {
					const name = info.getValue();
					return (
						<span className="text-sm text-muted-foreground">
							{name || "No name"}
						</span>
					);
				},
				enableSorting: true,
			}),
			columnHelper.accessor("status", {
				header: () => <span>Status</span>,
				cell: (info) => {
					const status = info.getValue();
					const statusTones: Record<
						ProductUserStatus,
						"success" | "warning" | "destructive"
					> = {
						[ProductUserStatus.ACTIVE]: "success",
						[ProductUserStatus.INVITED]: "warning",
						[ProductUserStatus.SUSPENDED]: "destructive",
					};
					return <StatusBadge tone={statusTones[status]}>{status}</StatusBadge>;
				},
				enableSorting: true,
			}),
			columnHelper.accessor("id", {
				header: () => <span>ID</span>,
				cell: (info) => (
					<span className="text-sm font-mono text-muted-foreground max-w-[150px] truncate block">
						{info.getValue()}
					</span>
				),
				enableSorting: false,
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
		],
		[],
	);

	// Build filter options from current data
	const emailOptions = useMemo(
		() =>
			Array.from(
				new Set((items as ProductUserResponse[]).map((item) => item.email)),
			).map((email) => ({ label: email, value: email })),
		[items],
	);

	const statusOptions = useMemo(
		() => [
			{ label: "Active", value: ProductUserStatus.ACTIVE },
			{ label: "Invited", value: ProductUserStatus.INVITED },
			{ label: "Suspended", value: ProductUserStatus.SUSPENDED },
		],
		[],
	);

	const idOptions = useMemo(
		() =>
			(items as ProductUserResponse[]).map((item) => ({
				label: item.id,
				value: item.id,
			})),
		[items],
	);

	if (!currentProduct) {
		return (
			<div className="flex items-center justify-center p-8">
				<div className="text-center">
					<p className="text-muted-foreground">
						Please select a product to view users
					</p>
				</div>
			</div>
		);
	}

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
						key: "status",
						label: "Status",
						type: "select",
						value: statusFilter,
						options: statusOptions,
						placeholder: "Filter by status",
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
					setEmailFilter(Array.isArray(filters.email) ? filters.email : []);
					setStatusFilter(
						Array.isArray(filters.status)
							? (filters.status as ProductUserStatus[])
							: [],
					);
					setIdFilter(Array.isArray(filters.id) ? filters.id : []);
				}}
				enableRowSelection={false}
			/>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load users: {error.message}
				</div>
			)}
		</>
	);
}
