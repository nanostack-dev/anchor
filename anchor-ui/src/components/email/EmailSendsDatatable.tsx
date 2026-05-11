import { useMemo, useState } from "react";
import dayjs from "dayjs";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import { listEmailSendsOptions } from "@/client/@tanstack/react-query.gen";
import type { EmailSendRecordResponse, EmailSendStatus, ListEmailSendsData, Options } from "@/client";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { useProduct } from "@/context/product/ProductContext";

const columnHelper = createColumnHelper<EmailSendRecordResponse>();

const statusColors: Record<string, string> = {
  SENT: "bg-green-100 text-green-800",
  SUPPRESSED: "bg-yellow-100 text-yellow-800",
  FAILED: "bg-red-100 text-red-800",
  QUEUED: "bg-gray-100 text-gray-600",
  SENDING: "bg-blue-100 text-blue-800",
};

export function EmailSendsDatatable() {
  const { currentProduct } = useProduct();

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  });
  const [sorting, setSorting] = useState<SortingState>([]);
  const [statusFilter, setStatusFilter] = useState<string>("");

  const queryOptions = useMemo((): Options<ListEmailSendsData> | null => {
    if (!currentProduct) return null;
    return {
      path: { product_id: currentProduct.id },
      query: {
        limit: pagination.pageSize,
        offset: pagination.pageIndex * pagination.pageSize,
        status: statusFilter ? (statusFilter as EmailSendStatus) : undefined,
      },
    };
  }, [currentProduct, pagination, statusFilter]);

  const { data, isLoading, error } = useQuery({
    ...listEmailSendsOptions(queryOptions!),
    placeholderData: keepPreviousData,
    enabled: !!queryOptions,
  });

  const statusOptions = useMemo(
    () => [
      { label: "Sent", value: "SENT" },
      { label: "Suppressed", value: "SUPPRESSED" },
      { label: "Failed", value: "FAILED" },
      { label: "Queued", value: "QUEUED" },
      { label: "Sending", value: "SENDING" },
    ],
    []
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor("to_address", {
        header: () => <span>To</span>,
        cell: info => <span className="text-sm">{info.getValue()}</span>,
        enableSorting: false,
      }),
      columnHelper.accessor("subject", {
        header: () => <span>Subject</span>,
        cell: info => <span className="text-sm text-muted-foreground max-w-[200px] truncate block">{info.getValue()}</span>,
        enableSorting: false,
      }),
      columnHelper.accessor("status", {
        header: () => <span>Status</span>,
        cell: info => (
          <span className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${statusColors[info.getValue()] ?? "bg-gray-100 text-gray-600"}`}>
            {info.getValue()}
          </span>
        ),
        enableSorting: false,
      }),
      columnHelper.accessor("from_address", {
        header: () => <span>From</span>,
        cell: info => <span className="text-sm text-muted-foreground">{info.getValue()}</span>,
        enableSorting: false,
      }),
      columnHelper.accessor("sent_at", {
        header: () => <span>Sent At</span>,
        cell: info => (
          <span className="text-sm text-muted-foreground">
            {info.getValue() ? dayjs(info.getValue()).format("D MMM YYYY H:mm") : "—"}
          </span>
        ),
        enableSorting: false,
      }),
      columnHelper.accessor("created_at", {
        header: () => <span>Created</span>,
        cell: info => <span className="text-sm text-muted-foreground">{dayjs(info.getValue()).format("D MMM YYYY H:mm")}</span>,
        enableSorting: false,
      }),
    ],
    []
  );

  if (!currentProduct) {
    return (
      <div className="flex items-center justify-center p-8">
        <p className="text-muted-foreground">Select a product to view email sends</p>
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
        onFiltersChange={filters => {
          setPagination(p => ({ ...p, pageIndex: 0 }));
          const s = filters.status;
          setStatusFilter(Array.isArray(s) ? (s[0] ?? "") : (s ?? ""));
        }}
        enableRowSelection={false}
      />
      {error && <p className="text-sm text-destructive mt-2">Failed to load sends: {error.message}</p>}
    </>
  );
}
