import type { WebhookEventTypeDescriptor } from "@/client";
import {
	getWebhookDeliveryOptions,
	listWebhookDeliveriesQueryKey,
	sendWebhookTestEventMutation,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Button } from "@/components/ui/button";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronDown, Send } from "lucide-react";
import { useId, useMemo, useState } from "react";
import {
	deliveryStatusTone,
	formatDuration,
	isTerminalDeliveryStatus,
} from "./webhook-display";

/** How often the panel re-reads a delivery that has not finished yet. */
const POLL_INTERVAL_MS = 1000;

const PING_EVENT_TYPE = "ping";

interface WebhookTestEventPanelProps {
	productId: string;
	endpointId: string;
	/** A disabled endpoint accrues no deliveries, so it cannot be tested. */
	endpointEnabled: boolean;
	descriptors: WebhookEventTypeDescriptor[];
}

/**
 * Test surface for one endpoint: pick a registered event type, see the exact
 * sample payload it will transmit, send it, and watch the outcome of that
 * specific delivery without leaving the panel.
 *
 * The outcome is polled by delivery id rather than by re-reading the log,
 * because the send returns the id it created. That is what makes repeated
 * sends safe: each one replaces the id being watched instead of racing to
 * guess which row in the log was its own.
 */
export function WebhookTestEventPanel({
	productId,
	endpointId,
	endpointEnabled,
	descriptors,
}: WebhookTestEventPanelProps) {
	const queryClient = useQueryClient();
	const selectId = useId();
	const [eventType, setEventType] = useState(PING_EVENT_TYPE);
	const [deliveryId, setDeliveryId] = useState<string | null>(null);

	const selected = useMemo(
		() => descriptors.find((descriptor) => descriptor.type === eventType),
		[descriptors, eventType],
	);

	const sendMutation = useMutation({
		...sendWebhookTestEventMutation(),
		onSuccess: (data) => {
			setDeliveryId(data.delivery_ids[0] ?? null);
			queryClient.invalidateQueries({
				queryKey: listWebhookDeliveriesQueryKey({
					path: { product_id: productId, webhook_endpoint_id: endpointId },
				}),
			});
		},
	});

	const deliveryQuery = useQuery({
		...getWebhookDeliveryOptions({
			path: {
				product_id: productId,
				webhook_endpoint_id: endpointId,
				delivery_id: deliveryId ?? "",
			},
		}),
		enabled: deliveryId !== null,
		refetchInterval: (query) => {
			const status = query.state.data?.delivery.status;
			return status && isTerminalDeliveryStatus(status)
				? false
				: POLL_INTERVAL_MS;
		},
	});

	const delivery = deliveryQuery.data?.delivery;
	const attempts = deliveryQuery.data?.attempts ?? [];
	const lastAttempt = attempts.at(-1);
	const settled = delivery ? isTerminalDeliveryStatus(delivery.status) : false;
	const inFlight = sendMutation.isPending || (deliveryId !== null && !settled);
	const sendError = sendMutation.error
		? (getApiErrorMessage(sendMutation.error) ??
			"Failed to queue the test event.")
		: null;

	return (
		<div className="flex flex-col gap-3">
			<div className="flex flex-col gap-1">
				<h3 className="text-sm font-medium">Testing</h3>
				<p className="text-xs text-muted-foreground">
					Sends a real, signed delivery down the same path as a business event.
					The envelope carries <span className="font-mono">"test": true</span>,
					so a receiver can log it and refuse to act on it.
				</p>
			</div>

			<div className="flex flex-wrap items-end gap-2">
				<div className="flex min-w-56 flex-1 flex-col gap-1.5">
					<Label htmlFor={selectId}>Event type</Label>
					<Select
						items={descriptors.map((descriptor) => ({
							label: descriptor.type,
							value: descriptor.type,
						}))}
						value={eventType}
						onValueChange={(value) => setEventType(value as string)}
					>
						<SelectTrigger
							id={selectId}
							className="w-full font-mono"
							disabled={!endpointEnabled || inFlight}
							aria-label="Test event type"
						>
							<SelectValue>{eventType}</SelectValue>
						</SelectTrigger>
						<SelectContent>
							{descriptors.map((descriptor) => (
								<SelectItem
									key={descriptor.type}
									value={descriptor.type}
									className="font-mono"
								>
									{descriptor.type}
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>
				<Button
					disabled={!endpointEnabled || inFlight}
					onClick={() => {
						sendMutation.reset();
						setDeliveryId(null);
						sendMutation.mutate({
							path: {
								product_id: productId,
								webhook_endpoint_id: endpointId,
							},
							body: { event_type: eventType },
						});
					}}
				>
					{inFlight ? (
						<Spinner className="text-current" />
					) : (
						<Send className="size-4" />
					)}
					Send test event
				</Button>
			</div>

			{selected?.description && (
				<p className="text-xs text-muted-foreground">{selected.description}</p>
			)}

			{selected?.sample_payload && (
				<Collapsible>
					<CollapsibleTrigger
						render={
							<Button
								type="button"
								variant="ghost"
								size="sm"
								className="h-auto gap-1 px-1 py-1 text-xs text-muted-foreground hover:text-foreground"
							/>
						}
					>
						<ChevronDown className="size-3.5 transition-transform data-[panel-open]:rotate-180" />
						Payload preview
					</CollapsibleTrigger>
					<CollapsibleContent>
						<pre className="mt-2 max-h-56 overflow-auto rounded-lg bg-muted p-3 font-mono text-xs whitespace-pre-wrap">
							{selected.sample_payload}
						</pre>
						<p className="mt-1 text-xs text-muted-foreground">
							This is the envelope's <span className="font-mono">data</span>{" "}
							object. Identifiers in it are illustrative and resolve to nothing.
						</p>
					</CollapsibleContent>
				</Collapsible>
			)}

			{sendError && <p className="text-sm text-destructive">{sendError}</p>}

			{!endpointEnabled && (
				<p className="text-xs text-muted-foreground">
					Enable the endpoint to send a test event.
				</p>
			)}

			{deliveryId && (
				<div className="flex flex-col gap-2 rounded-lg border border-border p-3">
					<div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
						{delivery ? (
							<StatusBadge tone={deliveryStatusTone[delivery.status]}>
								{delivery.status}
							</StatusBadge>
						) : (
							<Spinner className="text-muted-foreground" />
						)}
						<span className="font-mono text-xs text-muted-foreground">
							{deliveryId}
						</span>
						{lastAttempt?.status_code != null && (
							<span>
								HTTP{" "}
								<span className="font-medium">{lastAttempt.status_code}</span>
							</span>
						)}
						{lastAttempt && (
							<span className="text-muted-foreground">
								{formatDuration(lastAttempt.duration_ms)}
							</span>
						)}
						{delivery && (
							<span className="text-muted-foreground">
								attempt {delivery.attempt_count} / {delivery.max_attempts}
							</span>
						)}
					</div>

					{!settled && (
						<p className="text-xs text-muted-foreground">
							Waiting for the receiver…
						</p>
					)}
					{lastAttempt?.error && (
						<p className="text-sm text-destructive">{lastAttempt.error}</p>
					)}
					{lastAttempt?.response_snippet && (
						<pre className="max-h-40 overflow-auto rounded bg-muted p-2 font-mono text-xs whitespace-pre-wrap">
							{lastAttempt.response_snippet}
						</pre>
					)}
					{deliveryQuery.error && (
						<p className="text-sm text-destructive">
							{getApiErrorMessage(deliveryQuery.error) ??
								"Failed to read the delivery back."}
						</p>
					)}
				</div>
			)}
		</div>
	);
}
