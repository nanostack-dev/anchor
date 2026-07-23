import type { LicenseResponse } from "@/client";
import {
	getProductOrganizationOptions,
	listLicensesOptions,
	listPlansOptions,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import {
	LicenseDetailSheet,
	licenseStatusTone,
} from "@/components/product/licensing/LicenseDetailSheet";
import { LicenseDialog } from "@/components/product/licensing/LicenseDialog";
import { LicenseStatusActionDialog } from "@/components/product/licensing/LicenseStatusActionDialog";
import { Button } from "@/components/ui/button";
import { useQueries, useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import dayjs from "dayjs";
import { Ban, Eye, Pause, PenLine, Play, Plus } from "lucide-react";
import { useMemo, useState } from "react";

const columnHelper = createColumnHelper<LicenseResponse>();

const statusFilterOptions = [
	{ label: "Active", value: "ACTIVE" },
	{ label: "Suspended", value: "SUSPENDED" },
	{ label: "Revoked", value: "REVOKED" },
];

interface LicenseDatatableProps {
	productId: string;
}

export function LicenseDatatable({ productId }: LicenseDatatableProps) {
	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "created_at", desc: true },
	]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState<string[]>([]);

	const {
		data: licenseData,
		isLoading,
		error,
	} = useQuery(listLicensesOptions({ path: { product_id: productId } }));

	const { data: planData } = useQuery(
		listPlansOptions({ path: { product_id: productId } }),
	);

	const planById = useMemo(
		() => new Map((planData?.items ?? []).map((plan) => [plan.id, plan])),
		[planData],
	);

	const allLicenses = useMemo(() => licenseData?.items ?? [], [licenseData]);

	// Resolve organization names for the organizations present in the list.
	const uniqueOrganizationIds = useMemo(
		() => [...new Set(allLicenses.map((license) => license.organization_id))],
		[allLicenses],
	);
	const organizationQueries = useQueries({
		queries: uniqueOrganizationIds.map((organizationId) => ({
			...getProductOrganizationOptions({
				path: { product_id: productId, organization_id: organizationId },
			}),
			staleTime: 60_000,
		})),
	});
	const organizationNameById = useMemo(() => {
		const map = new Map<string, string>();
		organizationQueries.forEach((query, index) => {
			const name = query.data?.name;
			if (name) map.set(uniqueOrganizationIds[index], name);
		});
		return map;
	}, [organizationQueries, uniqueOrganizationIds]);

	// listLicenses has no server-side search/pagination: filter, sort and
	// slice client-side while keeping the shared AnchorDataTable UX.
	const filteredLicenses = useMemo(() => {
		const search = fullTextSearch.trim().toLowerCase();
		const filtered = allLicenses.filter((license) => {
			if (statusFilter.length > 0 && !statusFilter.includes(license.status)) {
				return false;
			}
			if (!search) return true;
			const plan = planById.get(license.plan_id);
			return [
				organizationNameById.get(license.organization_id) ?? "",
				license.organization_id,
				plan?.name ?? "",
				plan?.key ?? "",
				license.status,
			]
				.join(" ")
				.toLowerCase()
				.includes(search);
		});

		const sort = sorting[0];
		if (!sort) return filtered;
		const sorted = [...filtered].sort((a, b) => {
			switch (sort.id) {
				case "status":
					return a.status.localeCompare(b.status);
				case "expires_at":
					return (
						dayjs(a.expires_at ?? "2999-01-01").valueOf() -
						dayjs(b.expires_at ?? "2999-01-01").valueOf()
					);
				default:
					return dayjs(a.created_at).valueOf() - dayjs(b.created_at).valueOf();
			}
		});
		return sort.desc ? sorted.reverse() : sorted;
	}, [
		allLicenses,
		statusFilter,
		fullTextSearch,
		sorting,
		planById,
		organizationNameById,
	]);

	const pageLicenses = useMemo(
		() =>
			filteredLicenses.slice(
				pagination.pageIndex * pagination.pageSize,
				(pagination.pageIndex + 1) * pagination.pageSize,
			),
		[filteredLicenses, pagination],
	);

	const columns = useMemo(
		() => [
			columnHelper.accessor("organization_id", {
				id: "organization",
				header: () => <span>Organization</span>,
				cell: (info) => {
					const name = organizationNameById.get(info.getValue());
					return name ? (
						<span>{name}</span>
					) : (
						<span className="font-mono text-xs text-muted-foreground">
							{info.getValue()}
						</span>
					);
				},
				enableSorting: false,
			}),
			columnHelper.accessor("plan_id", {
				id: "plan",
				header: () => <span>Plan</span>,
				cell: (info) => {
					const plan = planById.get(info.getValue());
					return plan ? (
						<span>
							{plan.name}{" "}
							<span className="font-mono text-xs text-muted-foreground">
								({plan.key})
							</span>
						</span>
					) : (
						<span className="font-mono text-xs text-muted-foreground">
							{info.getValue()}
						</span>
					);
				},
				enableSorting: false,
			}),
			columnHelper.accessor("status", {
				header: () => <span>Status</span>,
				cell: (info) => (
					<StatusBadge tone={licenseStatusTone[info.getValue()]}>
						{info.getValue()}
					</StatusBadge>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("expires_at", {
				header: () => <span>Expires At</span>,
				cell: (info) => {
					const value = info.getValue();
					return value ? (
						dayjs(value).format("D MMMM YYYY H:mm")
					) : (
						<span className="text-sm text-muted-foreground">Never</span>
					);
				},
				enableSorting: true,
			}),
			columnHelper.accessor("grace_until", {
				header: () => <span>Grace Until</span>,
				cell: (info) => {
					const value = info.getValue();
					return value ? (
						dayjs(value).format("D MMMM YYYY H:mm")
					) : (
						<span className="text-sm text-muted-foreground">—</span>
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
				cell: ({ row }) => {
					const license = row.original;
					const organizationName = organizationNameById.get(
						license.organization_id,
					);
					return (
						<div className="flex gap-2">
							<LicenseDetailSheet
								license={license}
								plan={planById.get(license.plan_id)}
								organizationName={organizationName}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">View license details</span>
										<Eye className="size-4" />
									</Button>
								}
							/>
							<LicenseDialog
								productId={productId}
								existingLicense={license}
								organizationName={organizationName}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">Edit license</span>
										<PenLine className="size-4" />
									</Button>
								}
							/>
							{license.status === "ACTIVE" ? (
								<LicenseStatusActionDialog
									productId={productId}
									license={license}
									action="suspend"
									organizationName={organizationName}
									trigger={
										<Button variant="outline" size="icon">
											<span className="sr-only">Suspend license</span>
											<Pause className="size-4" />
										</Button>
									}
								/>
							) : (
								<LicenseStatusActionDialog
									productId={productId}
									license={license}
									action="reinstate"
									organizationName={organizationName}
									trigger={
										<Button variant="outline" size="icon">
											<span className="sr-only">Reinstate license</span>
											<Play className="size-4" />
										</Button>
									}
								/>
							)}
							{license.status !== "REVOKED" && (
								<LicenseStatusActionDialog
									productId={productId}
									license={license}
									action="revoke"
									organizationName={organizationName}
									trigger={
										<Button variant="outlineDestructive" size="icon">
											<span className="sr-only">Revoke license</span>
											<Ban className="size-4" />
										</Button>
									}
								/>
							)}
						</div>
					);
				},
			}),
		],
		[productId, planById, organizationNameById],
	);

	return (
		<>
			<div className="flex items-center justify-between mb-4">
				<div className="flex items-center gap-2">
					<LicenseDialog
						productId={productId}
						trigger={
							<Button>
								<Plus />
								Assign License
							</Button>
						}
					/>
				</div>
			</div>
			<AnchorDataTable
				columns={columns}
				data={pageLicenses}
				loading={isLoading}
				total={filteredLicenses.length}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={(search) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setFullTextSearch(search);
				}}
				fullTextSearchPlaceHolder="Search licenses"
				filters={[
					{
						key: "status",
						label: "Status",
						type: "select",
						value: statusFilter,
						options: statusFilterOptions,
						placeholder: "Filter by status",
						multi: true,
					},
				]}
				onFiltersChange={(filters) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setStatusFilter(Array.isArray(filters.status) ? filters.status : []);
				}}
				enableRowSelection={false}
			/>
			{error && (
				<div className="mt-2 text-destructive">
					Failed to load licenses: {error.message}
				</div>
			)}
		</>
	);
}
