import type {
	EmailSendRecordResponse,
	EmailSendStatus,
	ListEmailSendsData,
	Options,
} from "@/client";
import { listEmailSendsOptions } from "@/client/@tanstack/react-query.gen";
import { StatusBadge, type StatusTone } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { useProduct } from "@/context/product/ProductContext";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import dayjs from "dayjs";
import { useMemo, useState } from "react";

const columnHelper = createColumnHelper<EmailSendRecordResponse>();

const statusTones: Record<string, StatusTone> = {
	SENT: "success",
	SUPPRESSED: "warning",
	FAILED: "destructive",
	QUEUED: "neutral",
	SENDING: "info",
};

export function EmailSendsDatatable() {
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id ?? "";

	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 20,
	});
	const [sorting, setSorting] = useState<SortingState>([]);
	const [statusFilter, setStatusFilter] = useState<string>("");

	const queryOptions = useMemo((): Options<ListEmailSendsData> => {
		return {
			path: { product_id: productId },
			query: {
				limit: pagination.pageSize,
				offset: pagination.pageIndex * pagination.pageSize,
				status: statusFilter ? (statusFilter as EmailSendStatus) : undefined,
			},
		};
	}, [pagination, productId, statusFilter]);

	const { data, isLoading, error } = useQuery({
		...listEmailSendsOptions(queryOptions),
		placeholderData: keepPreviousData,
		enabled: !!currentProduct,
	});

	const statusOptions = useMemo(
		() => [
			{ label: "Sent", value: "SENT" },
			{ label: "Suppressed", value: "SUPPRESSED" },
			{ label: "Failed", value: "FAILED" },
			{ label: "Queued", value: "QUEUED" },
			{ label: "Sending", value: "SENDING" },
		],
		[],
	);

	const columns = useMemo(
		() => [
			columnHelper.accessor("to_address", {
				header: () => <span>To</span>,
				cell: (info) => <span className="text-sm">{info.getValue()}</span>,
				enableSorting: false,
			}),
			columnHelper.accessor("subject", {
				header: () => <span>Subject</span>,
				cell: (info) => (
					<span className="text-sm text-muted-foreground max-w-[200px] truncate block">
						{info.getValue()}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("status", {
				header: () => <span>Status</span>,
				cell: (info) => (
					<StatusBadge tone={statusTones[info.getValue()] ?? "neutral"}>
						{info.getValue()}
					</StatusBadge>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("from_address", {
				header: () => <span>From</span>,
				cell: (info) => (
					<span className="text-sm text-muted-foreground">
						{info.getValue()}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("sent_at", {
				header: () => <span>Sent At</span>,
				cell: (info) => (
					<span className="text-sm text-muted-foreground">
						{info.getValue()
							? dayjs(info.getValue()).format("D MMM YYYY H:mm")
							: "—"}
					</span>
				),
				enableSorting: false,
			}),
			columnHelper.accessor("created_at", {
				header: () => <span>Created</span>,
				cell: (info) => (
					<span className="text-sm text-muted-foreground">
						{dayjs(info.getValue()).format("D MMM YYYY H:mm")}
					</span>
				),
				enableSorting: false,
			}),
		],
		[],
	);

	if (!currentProduct) {
		return (
			<div className="flex items-center justify-center p-8">
				<p className="text-muted-foreground">
					Select a product to view email sends
				</p>
			</div>
		);
	}

	return (
		<>
			<AnchorDataTable
				columns={columns}
				data={data?.items ?? []}
				loading={isLoading}
				total={data?.count ?? 0}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				filters={[
					{
						key: "status",
						label: "Status",
						type: "select",
						value: statusFilter ? [statusFilter] : [],
						options: statusOptions,
						placeholder: "Filter by status",
						multi: false,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					const s = filters.status;
					setStatusFilter(Array.isArray(s) ? (s[0] ?? "") : (s ?? ""));
				}}
				enableRowSelection={false}
			/>
			{error && (
				<p className="text-sm text-destructive mt-2">
					Failed to load sends: {error.message}
				</p>
			)}
		</>
	);
}
