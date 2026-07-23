import { type WebhookEndpointResponse, WebhookEndpointStatus } from "@/client";
import { listWebhookEndpointsOptions } from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { AnchorDataTable } from "@/components/common/datatable/AnchorDataTable";
import { Button } from "@/components/ui/button";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { useQuery } from "@tanstack/react-query";
import type { PaginationState, SortingState } from "@tanstack/react-table";
import { createColumnHelper } from "@tanstack/react-table";
import dayjs from "dayjs";
import {
	Ban,
	Eye,
	KeyRound,
	PenLine,
	Play,
	Plus,
	Trash2,
	TriangleAlert,
} from "lucide-react";
import { useMemo, useState } from "react";
import { DeleteWebhookEndpointDialog } from "./DeleteWebhookEndpointDialog";
import { EventTypeChips } from "./EventTypeChips";
import { RotateWebhookSecretDialog } from "./RotateWebhookSecretDialog";
import { WebhookEndpointDetailSheet } from "./WebhookEndpointDetailSheet";
import { WebhookEndpointDialog } from "./WebhookEndpointDialog";
import { WebhookEndpointStatusDialog } from "./WebhookEndpointStatusDialog";
import {
	type SecretRevealVariant,
	WebhookSecretRevealDialog,
} from "./WebhookSecretRevealDialog";
import { endpointStatusLabel, endpointStatusTone } from "./webhook-display";

const columnHelper = createColumnHelper<WebhookEndpointResponse>();

const statusFilterOptions = [
	{
		label: "Enabled",
		value: WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_ENABLED,
	},
	{
		label: "Disabled",
		value: WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_DISABLED,
	},
	{
		label: "Auto-disabled",
		value: WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_AUTODISABLED,
	},
];

interface RevealState {
	secret: string;
	variant: SecretRevealVariant;
	endpointUrl: string;
}

interface WebhookEndpointDatatableProps {
	productId: string;
}

export function WebhookEndpointDatatable({
	productId,
}: WebhookEndpointDatatableProps) {
	const [pagination, setPagination] = useState<PaginationState>({
		pageIndex: 0,
		pageSize: 10,
	});
	const [sorting, setSorting] = useState<SortingState>([
		{ id: "created_at", desc: true },
	]);
	const [fullTextSearch, setFullTextSearch] = useState("");
	const [statusFilter, setStatusFilter] = useState<string[]>([]);
	const [reveal, setReveal] = useState<RevealState | null>(null);

	const {
		data: endpointData,
		isLoading,
		error,
	} = useQuery(
		listWebhookEndpointsOptions({ path: { product_id: productId } }),
	);

	const allEndpoints = useMemo(
		() => endpointData?.items ?? [],
		[endpointData?.items],
	);

	// listWebhookEndpoints has no server-side search/pagination: filter, sort
	// and slice client-side while keeping the shared AnchorDataTable UX.
	const filteredEndpoints = useMemo(() => {
		const search = fullTextSearch.trim().toLowerCase();
		const filtered = allEndpoints.filter((endpoint) => {
			if (statusFilter.length > 0 && !statusFilter.includes(endpoint.status)) {
				return false;
			}
			if (!search) return true;
			return [
				endpoint.url,
				endpoint.description ?? "",
				endpoint.event_types.join(" "),
				endpoint.status,
			]
				.join(" ")
				.toLowerCase()
				.includes(search);
		});

		const sort = sorting[0];
		if (!sort) return filtered;
		const sorted = [...filtered].sort((a, b) => {
			switch (sort.id) {
				case "url":
					return a.url.localeCompare(b.url);
				case "status":
					return a.status.localeCompare(b.status);
				case "consecutive_failure_count":
					return a.consecutive_failure_count - b.consecutive_failure_count;
				default:
					return dayjs(a.created_at).valueOf() - dayjs(b.created_at).valueOf();
			}
		});
		return sort.desc ? sorted.reverse() : sorted;
	}, [allEndpoints, statusFilter, fullTextSearch, sorting]);

	const pageEndpoints = useMemo(
		() =>
			filteredEndpoints.slice(
				pagination.pageIndex * pagination.pageSize,
				(pagination.pageIndex + 1) * pagination.pageSize,
			),
		[filteredEndpoints, pagination],
	);

	const columns = useMemo(
		() => [
			columnHelper.accessor("url", {
				header: () => <span>URL</span>,
				cell: (info) => (
					<span className="font-mono text-xs break-all">{info.getValue()}</span>
				),
				enableSorting: true,
			}),
			columnHelper.accessor("description", {
				header: () => <span>Description</span>,
				cell: (info) =>
					info.getValue()?.trim() || (
						<span className="text-sm text-muted-foreground">—</span>
					),
				enableSorting: false,
			}),
			columnHelper.accessor("status", {
				header: () => <span>Status</span>,
				cell: ({ row }) => {
					const endpoint = row.original;
					const badge = (
						<StatusBadge tone={endpointStatusTone[endpoint.status]}>
							{endpointStatusLabel[endpoint.status]}
						</StatusBadge>
					);
					if (
						endpoint.status !==
						WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_AUTODISABLED
					) {
						return badge;
					}
					return (
						<Tooltip>
							<TooltipTrigger
								render={
									<span className="inline-flex items-center gap-1">
										<TriangleAlert className="size-4 text-warning" />
										{badge}
									</span>
								}
							/>
							<TooltipContent>
								{endpoint.disabled_reason ||
									"Anchor disabled this endpoint after sustained delivery failures."}
							</TooltipContent>
						</Tooltip>
					);
				},
				enableSorting: true,
			}),
			columnHelper.accessor("event_types", {
				id: "event_types",
				header: () => <span>Event Types</span>,
				cell: (info) => <EventTypeChips eventTypes={info.getValue()} />,
				enableSorting: false,
			}),
			columnHelper.accessor("consecutive_failure_count", {
				header: () => <span>Failures</span>,
				cell: (info) => {
					const count = info.getValue();
					return count > 0 ? (
						<StatusBadge tone="destructive">{count} in a row</StatusBadge>
					) : (
						<span className="text-sm text-muted-foreground">—</span>
					);
				},
				enableSorting: true,
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
					const endpoint = row.original;
					const isEnabled =
						endpoint.status ===
						WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_ENABLED;
					return (
						<div className="flex gap-2">
							<WebhookEndpointDetailSheet
								productId={productId}
								endpoint={endpoint}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">
											View endpoint and delivery log
										</span>
										<Eye className="size-4" />
									</Button>
								}
							/>
							<WebhookEndpointDialog
								productId={productId}
								mode="edit"
								existingEndpoint={endpoint}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">Edit endpoint</span>
										<PenLine className="size-4" />
									</Button>
								}
							/>
							<WebhookEndpointStatusDialog
								productId={productId}
								endpoint={endpoint}
								action={isEnabled ? "disable" : "enable"}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">
											{isEnabled ? "Disable endpoint" : "Enable endpoint"}
										</span>
										{isEnabled ? (
											<Ban className="size-4" />
										) : (
											<Play className="size-4" />
										)}
									</Button>
								}
							/>
							<RotateWebhookSecretDialog
								productId={productId}
								endpoint={endpoint}
								onRotated={(secret) =>
									setReveal({
										secret,
										variant: "rotated",
										endpointUrl: endpoint.url,
									})
								}
								trigger={
									<Button variant="outline" size="icon">
										<span className="sr-only">Rotate signing secret</span>
										<KeyRound className="size-4" />
									</Button>
								}
							/>
							<DeleteWebhookEndpointDialog
								productId={productId}
								endpoint={endpoint}
								trigger={
									<Button variant="outlineDestructive" size="icon">
										<span className="sr-only">Delete endpoint</span>
										<Trash2 className="size-4" />
									</Button>
								}
							/>
						</div>
					);
				},
			}),
		],
		[productId],
	);

	return (
		<>
			<div className="mb-4 flex items-center justify-between">
				<div className="flex items-center gap-2">
					<WebhookEndpointDialog
						productId={productId}
						mode="create"
						onCreated={(endpoint, secret) =>
							setReveal({
								secret,
								variant: "created",
								endpointUrl: endpoint.url,
							})
						}
						trigger={
							<Button>
								<Plus />
								Create Endpoint
							</Button>
						}
					/>
				</div>
			</div>
			<AnchorDataTable
				columns={columns}
				data={pageEndpoints}
				loading={isLoading}
				total={filteredEndpoints.length}
				pagination={pagination}
				onPaginationChange={setPagination}
				sorting={sorting}
				onSortingChange={setSorting}
				fullTextSearch={fullTextSearch}
				onFullTextSearchChange={(search) => {
					setPagination((p) => ({ ...p, pageIndex: 0 }));
					setFullTextSearch(search);
				}}
				fullTextSearchPlaceHolder="Search endpoints"
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
					Failed to load webhook endpoints: {error.message}
				</div>
			)}

			<WebhookSecretRevealDialog
				open={reveal !== null}
				secret={reveal?.secret ?? null}
				endpointUrl={reveal?.endpointUrl}
				variant={reveal?.variant ?? "created"}
				onAcknowledged={() => setReveal(null)}
			/>
		</>
	);
}
