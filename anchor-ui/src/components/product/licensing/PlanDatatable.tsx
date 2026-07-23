import type { PlanResponse } from "@/client";
import { listPlansOptions } from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { DeletePlanDialog } from "@/components/product/licensing/DeletePlanDialog";
import { PlanDialog } from "@/components/product/licensing/PlanDialog";
import { Button } from "@/components/ui/button";
import { useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import dayjs from "dayjs";
import { Copy, PenLine, Plus, Trash2 } from "lucide-react";
import { useMemo, useState } from "react";
import { toast } from "sonner";

const columnHelper = createColumnHelper<PlanResponse>();

interface PlanDatatableProps {
	productId: string;
}

export function PlanDatatable({ productId }: PlanDatatableProps) {
	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "created_at", desc: true },
	]);
	const [fullTextSearch, setFullTextSearch] = useState("");

	const {
		data: planData,
		isLoading,
		error,
	} = useQuery(listPlansOptions({ path: { product_id: productId } }));

	const allPlans = useMemo(() => planData?.items ?? [], [planData]);

	// listPlans has no server-side search/pagination: filter, sort and slice
	// client-side while keeping the shared AnchorDataTable UX.
	const filteredPlans = useMemo(() => {
		const search = fullTextSearch.trim().toLowerCase();
		const filtered = search
			? allPlans.filter((plan) =>
					[plan.key, plan.name, plan.description ?? ""]
						.join(" ")
						.toLowerCase()
						.includes(search),
				)
			: allPlans;

		const sort = sorting[0];
		if (!sort) return filtered;
		const sorted = [...filtered].sort((a, b) => {
			switch (sort.id) {
				case "key":
					return a.key.localeCompare(b.key);
				case "name":
					return a.name.localeCompare(b.name);
				default:
					return dayjs(a.created_at).valueOf() - dayjs(b.created_at).valueOf();
			}
		});
		return sort.desc ? sorted.reverse() : sorted;
	}, [allPlans, fullTextSearch, sorting]);

	const pagePlans = useMemo(
		() =>
			filteredPlans.slice(
				pagination.pageIndex * pagination.pageSize,
				(pagination.pageIndex + 1) * pagination.pageSize,
			),
		[filteredPlans, pagination],
	);

	const columns = useMemo(
		() => [
			columnHelper.accessor("key", {
				header: () => <span>Key</span>,
				cell: (info) => (
					<span className="font-mono text-sm">{info.getValue()}</span>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("name", {
				header: () => <span>Name</span>,
				cell: (info) => info.getValue(),
				enableSorting: true,
			}),
			columnHelper.accessor("is_default", {
				header: () => <span>Default</span>,
				cell: (info) =>
					info.getValue() ? (
						<StatusBadge tone="info">Default</StatusBadge>
					) : (
						<span className="text-sm text-muted-foreground">—</span>
					),
				enableSorting: false,
			}),
			columnHelper.accessor("entitlements", {
				id: "entitlements",
				header: () => <span>Entitlements</span>,
				cell: (info) => {
					const count = Object.keys(info.getValue()).length;
					return (
						<span className="text-sm text-muted-foreground">
							{count} entitlement{count === 1 ? "" : "s"}
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
			columnHelper.display({
				id: "actions",
				header: () => <span>Actions</span>,
				cell: ({ row }) => (
					<div className="flex gap-2">
						<Button
							variant="outline"
							size="icon"
							onClick={() => {
								navigator.clipboard.writeText(row.original.id);
								toast.success("ID copied to clipboard");
							}}
						>
							<span className="sr-only">Copy ID</span>
							<Copy className="size-4" />
						</Button>
						<PlanDialog
							productId={productId}
							mode="edit"
							existingPlan={row.original}
							trigger={
								<Button variant="outline" size="icon">
									<span className="sr-only">Edit plan</span>
									<PenLine className="size-4" />
								</Button>
							}
						/>
						<DeletePlanDialog
							productId={productId}
							plan={row.original}
							trigger={
								<Button variant="outlineDestructive" size="icon">
									<span className="sr-only">Delete plan</span>
									<Trash2 className="size-4" />
								</Button>
							}
						/>
					</div>
				),
			}),
		],
		[productId],
	);

	return (
		<>
			<div className="flex items-center justify-between mb-4">
				<div className="flex items-center gap-2">
					<PlanDialog
						productId={productId}
						mode="create"
						trigger={
							<Button>
								<Plus />
								Create Plan
							</Button>
						}
					/>
				</div>
			</div>
			<AnchorDataTable
				columns={columns}
				data={pagePlans}
				loading={isLoading}
				total={filteredPlans.length}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={(search) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setFullTextSearch(search);
				}}
				fullTextSearchPlaceHolder="Search plans"
				enableRowSelection={false}
			/>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load plans: {error.message}
				</div>
			)}
		</>
	);
}
