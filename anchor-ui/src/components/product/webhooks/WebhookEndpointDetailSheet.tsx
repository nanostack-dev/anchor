import {
	type WebhookDeliveryStatus,
	type WebhookEndpointResponse,
	WebhookEndpointStatus,
} from "@/client";
import {
	listWebhookDeliveriesOptions,
	listWebhookEventTypesOptions,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { FacetedFilter } from "@/components/common/datatable/FacetedFilter";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
	SheetTrigger,
} from "@/components/ui/sheet";
import { Skeleton } from "@/components/ui/skeleton";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { getApiErrorMessage } from "@/lib/api-error";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { AlertTriangle } from "lucide-react";
import { type ReactElement, type ReactNode, useMemo, useState } from "react";
import { EventTypeChips } from "./EventTypeChips";
import { WebhookDeliveryDetailDialog } from "./WebhookDeliveryDetailDialog";
import { WebhookTestEventPanel } from "./WebhookTestEventPanel";
import {
	deliveryStatusOptions,
	deliveryStatusTone,
	endpointStatusLabel,
	endpointStatusTone,
	formatDateTime,
	isEventTypeCovered,
} from "./webhook-display";

const PAGE_SIZE = 20;

interface WebhookEndpointDetailSheetProps {
	productId: string;
	endpoint: WebhookEndpointResponse;
	trigger: ReactElement;
}

/**
 * Endpoint detail: health summary plus the delivery log. The log is the single
 * largest support-cost reducer in a webhook system — without it every "we
 * didn't get the event" question is an engineer reading production logs.
 */
export function WebhookEndpointDetailSheet({
	productId,
	endpoint,
	trigger,
}: WebhookEndpointDetailSheetProps) {
	const [open, setOpen] = useState(false);
	const [statusFilter, setStatusFilter] = useState<string[]>([]);
	const [eventTypeFilter, setEventTypeFilter] = useState<string[]>([]);
	const [pageIndex, setPageIndex] = useState(0);
	const [openDeliveryId, setOpenDeliveryId] = useState<string | null>(null);

	const { data: catalog } = useQuery({
		...listWebhookEventTypesOptions(),
		enabled: open,
	});

	// Only offer event types this endpoint can actually receive; filtering by a
	// type it never subscribes to can only ever return nothing.
	const eventTypeOptions = useMemo(
		() =>
			(catalog?.items ?? [])
				.filter((descriptor) =>
					isEventTypeCovered(endpoint.event_types, descriptor),
				)
				.map((descriptor) => ({
					label: descriptor.type,
					value: descriptor.type,
				})),
		[catalog?.items, endpoint.event_types],
	);

	const deliveryQuery = useQuery({
		...listWebhookDeliveriesOptions({
			path: { product_id: productId, webhook_endpoint_id: endpoint.id },
			query: {
				status: statusFilter[0] as WebhookDeliveryStatus | undefined,
				event_type: eventTypeFilter[0],
				// One row over the page size tells us whether a next page exists;
				// the list response carries items only, no total.
				limit: PAGE_SIZE + 1,
				offset: pageIndex * PAGE_SIZE,
			},
		}),
		enabled: open,
		placeholderData: keepPreviousData,
	});

	const fetched = deliveryQuery.data?.items ?? [];
	const hasNextPage = fetched.length > PAGE_SIZE;
	const deliveries = fetched.slice(0, PAGE_SIZE);

	const summaryRows: Array<{ label: string; value: ReactNode }> = [
		{
			label: "URL",
			value: <span className="font-mono text-xs">{endpoint.url}</span>,
		},
		{ label: "Description", value: endpoint.description?.trim() || "—" },
		{
			label: "Status",
			value: (
				<StatusBadge tone={endpointStatusTone[endpoint.status]}>
					{endpointStatusLabel[endpoint.status]}
				</StatusBadge>
			),
		},
		{
			label: "Consecutive failures",
			value: endpoint.consecutive_failure_count,
		},
		{
			label: "First failure",
			value: formatDateTime(endpoint.first_failure_at),
		},
		{ label: "Last failure", value: formatDateTime(endpoint.last_failure_at) },
		{ label: "Last success", value: formatDateTime(endpoint.last_success_at) },
		{ label: "Created", value: formatDateTime(endpoint.created_at) },
	];

	const resetPaging = () => setPageIndex(0);

	return (
		<Sheet open={open} onOpenChange={setOpen}>
			<SheetTrigger render={trigger} />
			<SheetContent
				side="right"
				className="w-full overflow-y-auto data-[side=right]:sm:max-w-4xl"
			>
				<SheetHeader>
					<SheetTitle>Webhook Endpoint</SheetTitle>
					<SheetDescription>
						Health of this subscription and every delivery Anchor attempted for
						it.
					</SheetDescription>
				</SheetHeader>

				<div className="flex flex-col gap-6 px-4 pb-6">
					{endpoint.status ===
						WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_AUTODISABLED && (
						<Alert variant="warning">
							<AlertTriangle />
							<AlertTitle>Anchor disabled this endpoint</AlertTitle>
							<AlertDescription>
								<p>
									{endpoint.disabled_reason ||
										"Sustained delivery failures, or a 410 Gone from the receiver."}
								</p>
								<p>
									No new deliveries are being created. Fix the receiver, then
									enable the endpoint to resume — the failure streak is cleared
									on enable.
								</p>
							</AlertDescription>
						</Alert>
					)}

					<dl className="grid grid-cols-[auto_1fr] items-center gap-x-6 gap-y-2 text-sm">
						{summaryRows.map((row) => (
							<div key={row.label} className="contents">
								<dt className="font-medium text-muted-foreground">
									{row.label}
								</dt>
								<dd className="min-w-0 truncate text-right">{row.value}</dd>
							</div>
						))}
					</dl>

					<div className="flex flex-col gap-2">
						<h3 className="text-sm font-medium">Subscribed event types</h3>
						<EventTypeChips eventTypes={endpoint.event_types} max={20} />
					</div>

					<WebhookTestEventPanel
						productId={productId}
						endpointId={endpoint.id}
						endpointEnabled={
							endpoint.status ===
							WebhookEndpointStatus.WEBHOOK_ENDPOINT_STATUS_ENABLED
						}
						descriptors={catalog?.items ?? []}
					/>

					<div className="flex flex-col gap-3">
						<h3 className="text-sm font-medium">Delivery log</h3>

						<div className="flex flex-wrap items-center gap-2">
							<FacetedFilter
								label="Status"
								options={deliveryStatusOptions}
								selected={statusFilter}
								multi={false}
								placeholder="Filter by status"
								onChange={(selected) => {
									resetPaging();
									setStatusFilter(selected);
								}}
							/>
							<FacetedFilter
								label="Event type"
								options={eventTypeOptions}
								selected={eventTypeFilter}
								multi={false}
								placeholder="Filter by event type"
								onChange={(selected) => {
									resetPaging();
									setEventTypeFilter(selected);
								}}
							/>
							{(statusFilter.length > 0 || eventTypeFilter.length > 0) && (
								<Button
									variant="ghost"
									size="sm"
									className="text-muted-foreground hover:text-foreground"
									onClick={() => {
										resetPaging();
										setStatusFilter([]);
										setEventTypeFilter([]);
									}}
								>
									Clear all
								</Button>
							)}
						</div>

						{deliveryQuery.error ? (
							<p className="text-sm text-destructive">
								Failed to load deliveries:{" "}
								{getApiErrorMessage(deliveryQuery.error) ??
									deliveryQuery.error.message}
							</p>
						) : null}

						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>Event type</TableHead>
									<TableHead>Status</TableHead>
									<TableHead>Attempts</TableHead>
									<TableHead>Created</TableHead>
									<TableHead>Last attempt</TableHead>
									<TableHead className="sr-only">Actions</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{deliveryQuery.isLoading ? (
									Array.from({ length: 5 }).map((_, index) => (
										<TableRow
											// biome-ignore lint/suspicious/noArrayIndexKey: static skeleton placeholder rows have no stable id.
											key={`delivery-skeleton-${index}`}
										>
											<TableCell colSpan={6}>
												<Skeleton className="h-4 w-full" />
											</TableCell>
										</TableRow>
									))
								) : deliveries.length === 0 ? (
									<TableRow>
										<TableCell
											colSpan={6}
											className="h-24 text-center text-sm text-muted-foreground"
										>
											No deliveries yet. Send a test event to check the
											receiver.
										</TableCell>
									</TableRow>
								) : (
									deliveries.map((delivery) => (
										<TableRow key={delivery.id}>
											<TableCell className="font-mono text-xs">
												<span className="flex items-center gap-1.5">
													{delivery.event_type}
													{delivery.test && (
														<Badge variant="outline" className="font-sans">
															Test
														</Badge>
													)}
												</span>
											</TableCell>
											<TableCell>
												<StatusBadge tone={deliveryStatusTone[delivery.status]}>
													{delivery.status}
												</StatusBadge>
											</TableCell>
											<TableCell className="text-sm">
												{delivery.attempt_count} / {delivery.max_attempts}
											</TableCell>
											<TableCell className="text-sm">
												{formatDateTime(delivery.created_at)}
											</TableCell>
											<TableCell className="text-sm">
												{formatDateTime(
													delivery.completed_at ?? delivery.updated_at,
												)}
											</TableCell>
											<TableCell className="text-right">
												<Button
													variant="outline"
													size="sm"
													onClick={() => setOpenDeliveryId(delivery.id)}
												>
													Inspect
												</Button>
											</TableCell>
										</TableRow>
									))
								)}
							</TableBody>
						</Table>

						<div className="flex items-center justify-end gap-2">
							<span className="mr-auto text-sm text-muted-foreground">
								Page {pageIndex + 1}
							</span>
							<Button
								variant="outline"
								size="sm"
								disabled={pageIndex === 0 || deliveryQuery.isFetching}
								onClick={() => setPageIndex((page) => Math.max(0, page - 1))}
							>
								Previous
							</Button>
							<Button
								variant="outline"
								size="sm"
								disabled={!hasNextPage || deliveryQuery.isFetching}
								onClick={() => setPageIndex((page) => page + 1)}
							>
								Next
							</Button>
						</div>
					</div>
				</div>

				<WebhookDeliveryDetailDialog
					productId={productId}
					endpointId={endpoint.id}
					deliveryId={openDeliveryId}
					onOpenChange={(nextOpen) => {
						if (!nextOpen) setOpenDeliveryId(null);
					}}
				/>
			</SheetContent>
		</Sheet>
	);
}
