import {
	type Options,
	type ProductApiKeyResponse,
	type ProductApiKeySearchRequest,
	ProductApiKeyStatus,
	type SearchProductApiKeysData,
	SortDirection,
} from "@/client";
import { searchProductApiKeysOptions } from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { DeleteProductAPIKeyDialog } from "@/components/product/apikey/DeleteProductApiKeyDialog";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { mapSortingToApiField } from "@/utils/datatable-sorting";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { PenLine, Plus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";
import { Button } from "../../ui/button";

const columnHelper = createColumnHelper<ProductApiKeyResponse>();

type ProductApiKeyFilters = {
	name: string[];
	status: ProductApiKeyStatus[];
};

interface ProductApiKeyDatatableProps {
	productId: string;
}

export function ProductApiKeyDatatable({
	productId,
}: ProductApiKeyDatatableProps) {
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

	const [nameFilter, setNameFilter] = useState<string[]>([]);
	const debouncedName = useDebounce(nameFilter, 100);
	const [statusFilter, setStatusFilter] = useState<ProductApiKeyStatus[]>([]);
	const debouncedStatus = useDebounce(statusFilter, 100);
	const [
		searchProductApiKeysOptionsParams,
		setSearchProductApiKeysOptionsParams,
	] = useState<Options<SearchProductApiKeysData>>(() => ({
		path: {
			product_id: productId,
		},
		body: {
			pagination: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
			},
			sort_by: mapSortingToApiField<ProductApiKeySearchRequest["sort_by"]>(
				sorting[0]?.id,
				"created_at",
			),
			sort_direction: sorting[0]?.desc ? SortDirection.DESC : SortDirection.ASC,
			full_text_search: debouncedFullTextSearch || undefined,
			filter: {
				names: debouncedName.length > 0 ? debouncedName : undefined,
				status: debouncedStatus.length > 0 ? debouncedStatus : undefined,
			},
		},
	}));
	const {
		data: apiKeyData,
		isLoading,
		error,
	} = useQuery({
		...searchProductApiKeysOptions(searchProductApiKeysOptionsParams),
		placeholderData: keepPreviousData,
	});

	const { items = [], total: fetchedTotal = 0 } = apiKeyData ?? {};
	useMemo(() => {
		setTotal(fetchedTotal);
	}, [fetchedTotal]);

	useEffect(() => {
		setSearchProductApiKeysOptionsParams({
			path: {
				product_id: productId,
			},
			body: {
				pagination: {
					limit: pagination.pageSize,
					offset: pagination.pageIndex * pagination.pageSize,
				},
				sort_by: mapSortingToApiField<
					"id" | "name" | "created_at" | "last_used_at" | "status"
				>(sorting[0]?.id, "created_at"),
				sort_direction: sorting[0]?.desc
					? SortDirection.DESC
					: SortDirection.ASC,
				full_text_search: debouncedFullTextSearch || undefined,
				filter: {
					names: debouncedName.length > 0 ? debouncedName : undefined,
					status: debouncedStatus.length > 0 ? debouncedStatus : undefined,
				},
			},
		});
	}, [
		productId,
		pagination,
		sorting,
		debouncedFullTextSearch,
		debouncedName,
		debouncedStatus,
	]);

	const columns = useMemo(
		() => [
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => info.getValue(),
				enableSorting: true,
			}),
			columnHelper.accessor("description", {
				header: () => <span>Description</span>,
				cell: (info) => {
					const description = info.getValue();
					return (
						<span className="text-sm text-muted-foreground max-w-[200px] truncate block">
							{description || "No description"}
						</span>
					);
				},
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created At</span>,
				cell: (info) => dayjs(info.getValue()).format("D MMMM YYYY H:mm"),
				enableSorting: true,
			}),
			columnHelper.accessor("last_used_at", {
				header: () => <span>Last Used</span>,
				cell: (info) => {
					const lastUsed = info.getValue();
					return lastUsed
						? dayjs(lastUsed).format("D MMMM YYYY H:mm")
						: "Never";
				},
				enableSorting: true,
			}),
			columnHelper.accessor("status", {
				header: () => <span>Status</span>,
				cell: (info) => {
					const status = info.getValue();
					const tone =
						status === ProductApiKeyStatus.PRODUCT_API_KEY_STATUS_ACTIVE
							? "success"
							: status === ProductApiKeyStatus.PRODUCT_API_KEY_STATUS_INACTIVE
								? "neutral"
								: "warning";
					return <StatusBadge tone={tone}>{status}</StatusBadge>;
				},
				enableSorting: true,
			}),
			columnHelper.accessor("mutable", {
				header: () => <span>Mutable</span>,
				cell: (info) => (
					<StatusBadge tone={info.getValue() ? "info" : "neutral"}>
						{info.getValue() ? "Yes" : "No"}
					</StatusBadge>
				),
				enableSorting: false,
			}),
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<div className={"flex gap-2"}>
						<Button
							variant="outline"
							size="icon"
							render={
								<Link
									to={ROUTE_PATHS.PRODUCT_API_KEY_EDIT}
									params={{ apiKeyId: row.original.id }}
								/>
							}
						>
							<span className="sr-only">Edit API key</span>
							<PenLine className="h-4 w-4" />
						</Button>
						<DeleteProductAPIKeyDialog
							productId={productId}
							apiKey={row.original}
						/>
					</div>
				),
			}),
		],
		[productId],
	);

	const nameOptions = useMemo(
		() =>
			Array.from(
				new Set((items as ProductApiKeyResponse[]).map((item) => item.name)),
			).map((name) => ({ label: name, value: name })),
		[items],
	);

	const statusOptions = useMemo(
		() => [
			{
				label: "Active",
				value: ProductApiKeyStatus.PRODUCT_API_KEY_STATUS_ACTIVE,
			},
			{
				label: "Inactive",
				value: ProductApiKeyStatus.PRODUCT_API_KEY_STATUS_INACTIVE,
			},
		],
		[],
	);

	return (
		<>
			<div className="flex items-center justify-between mb-4">
				<div className="flex items-center gap-2">
					<Button render={<Link to={ROUTE_PATHS.PRODUCT_API_KEY_NEW} />}>
						<Plus />
						Create API Key
					</Button>
				</div>
			</div>
			<AnchorDataTable<ProductApiKeyResponse, ProductApiKeyFilters>
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
				fullTextSearchPlaceHolder="Search API keys"
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
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setNameFilter(Array.isArray(filters.name) ? filters.name : []);
					setStatusFilter(Array.isArray(filters.status) ? filters.status : []);
				}}
				enableRowSelection={false}
			/>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load API keys: {error.message}
				</div>
			)}
		</>
	);
}
