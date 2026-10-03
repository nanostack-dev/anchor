import { deleteProductApiKey } from "@/client";
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
import { ButtonLink } from "@nanostackorg/design-system/components/button";
import { Box } from "@nanostackorg/design-system/layout/box";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { useDebounce } from "@uidotdev/usehooks";
import dayjs from "dayjs";
import { PenLine, Plus } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { AnchorDataTable } from "../../common/datatable/AnchorDataTable";

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
	const navigate = useNavigate();
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
		isFetching,
		error,
		refetch,
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
						<Box
							as="span"
							className="text-sm text-muted-foreground max-w-[200px] truncate block"
						>
							{description || "No description"}
						</Box>
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
						status === ProductApiKeyStatus.ACTIVE
							? "success"
							: status === ProductApiKeyStatus.INACTIVE
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
					<Box className={"flex gap-2"}>
						<ButtonLink
							variant="outline"
							size="md"
							href={ROUTE_PATHS.PRODUCT_API_KEY_EDIT.replace(
								"$apiKeyId",
								encodeURIComponent(row.original.id),
							)}
						>
							<Box as="span" className="sr-only">
								Edit API key
							</Box>
							<PenLine className="h-4 w-4" />
						</ButtonLink>
						<DeleteProductAPIKeyDialog
							productId={productId}
							apiKey={row.original}
						/>
					</Box>
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
			{ label: "Active", value: ProductApiKeyStatus.ACTIVE },
			{ label: "Inactive", value: ProductApiKeyStatus.INACTIVE },
		],
		[],
	);

	return (
		<>
			<Box className="flex items-center justify-between mb-4">
				<Box className="flex items-center gap-2">
					<ButtonLink
						variant="solid"
						tone="brand"
						href={ROUTE_PATHS.PRODUCT_API_KEY_NEW}
					>
						<Plus />
						Create API Key
					</ButtonLink>
				</Box>
			</Box>
			<AnchorDataTable<ProductApiKeyResponse, ProductApiKeyFilters>
				columns={columns}
				data={items}
				onRowClick={(apiKey) => {
					void navigate({
						to: ROUTE_PATHS.PRODUCT_API_KEY_EDIT,
						params: { apiKeyId: apiKey.id },
					});
				}}
				loading={isLoading || isFetching}
				resourceName="API keys"
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
				getRowId={(row) => row.id}
				getRowLabel={(row) => row.name}
				selectionScope={productId}
				bulkActions={[
					{
						id: "delete",
						label: "Delete selected",
						description:
							"This permanently deletes the selected API keys and revokes their access.",
						destructive: true,
						removesRows: true,
						run: (row) =>
							deleteProductApiKey({
								path: { product_id: productId, api_key_id: row.id },
								throwOnError: true,
							}),
					},
				]}
				onBulkActionComplete={async () => {
					await refetch({ throwOnError: true });
				}}
			/>
		</>
	);
}
