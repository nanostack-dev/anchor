import {
	getWebhookDeliveryOptions,
	listWebhookDeliveriesQueryKey,
	retryWebhookDeliveryMutation,
} from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import { getApiError, getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RefreshCw } from "lucide-react";
import type { ReactNode } from "react";
import { toast } from "sonner";
import {
	deliveryStatusTone,
	formatDateTime,
	formatDuration,
} from "./webhook-display";

interface WebhookDeliveryDetailDialogProps {
	productId: string;
	endpointId: string;
	/** Delivery to inspect; `null` closes the dialog. */
	deliveryId: string | null;
	onOpenChange: (open: boolean) => void;
}

/**
 * Attempt-level view of one delivery: the append-only attempt log with status
 * codes, errors, durations and the 2KB response snippet, plus the frozen
 * request body that was actually signed.
 */
export function WebhookDeliveryDetailDialog({
	productId,
	endpointId,
	deliveryId,
	onOpenChange,
}: WebhookDeliveryDetailDialogProps) {
	const queryClient = useQueryClient();

	const { data, isLoading, error } = useQuery({
		...getWebhookDeliveryOptions({
			path: {
				product_id: productId,
				webhook_endpoint_id: endpointId,
				delivery_id: deliveryId ?? "",
			},
		}),
		enabled: deliveryId !== null,
	});

	const retryMutation = useMutation({
		...retryWebhookDeliveryMutation(),
		onSuccess: () => {
			toast.success("Replay queued.", {
				description:
					"A new delivery was created for this event; it appears at the top of the log.",
			});
			queryClient.invalidateQueries({
				queryKey: listWebhookDeliveriesQueryKey({
					path: { product_id: productId, webhook_endpoint_id: endpointId },
				}),
			});
		},
		onError: (retryError: unknown) => {
			console.error("Failed to retry webhook delivery:", retryError);
			const apiError = getApiError(retryError);
			if (apiError?.code === "WEBHOOK_DELIVERY_STILL_PENDING") {
				toast.error(
					"This delivery has not finished its own retry ladder yet. Wait for it to settle before replaying.",
				);
				return;
			}
			if (apiError?.code === "WEBHOOK_DELIVERY_NOT_REPLAYABLE") {
				toast.error(
					"This delivery is itself a replay. Retry the original delivery instead.",
				);
				return;
			}
			toast.error(
				getApiErrorMessage(retryError) ??
					"Failed to queue a replay. Please try again.",
			);
		},
	});

	const delivery = data?.delivery;
	// A replay is queued once per press: keep the button dead after success so
	// an impatient double-click cannot fan out duplicate deliveries.
	const retryDisabled = retryMutation.isPending || retryMutation.isSuccess;

	const summaryRows: Array<{ label: string; value: ReactNode }> = delivery
		? [
				{ label: "Event type", value: delivery.event_type },
				{
					label: "Status",
					value: (
						<StatusBadge tone={deliveryStatusTone[delivery.status]}>
							{delivery.status}
						</StatusBadge>
					),
				},
				{
					label: "Attempts",
					value: `${delivery.attempt_count} of ${delivery.max_attempts}`,
				},
				{
					label: "Target",
					value: (
						<span className="font-mono text-xs">{delivery.target_url}</span>
					),
				},
				{
					label: "Event ID",
					value: <span className="font-mono text-xs">{delivery.event_id}</span>,
				},
				{
					label: "Last status code",
					value: delivery.last_status_code ?? "—",
				},
				{ label: "Last error", value: delivery.last_error || "—" },
				{ label: "Created", value: formatDateTime(delivery.created_at) },
				{ label: "Completed", value: formatDateTime(delivery.completed_at) },
				{
					label: "Replay of",
					value: delivery.replay_of_delivery_id ? (
						<span className="font-mono text-xs">
							{delivery.replay_of_delivery_id}
						</span>
					) : (
						"—"
					),
				},
			]
		: [];

	return (
		<Dialog
			open={deliveryId !== null}
			onOpenChange={(nextOpen) => {
				if (!nextOpen) {
					retryMutation.reset();
					onOpenChange(false);
				}
			}}
		>
			<DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[760px]">
				<DialogHeader>
					<DialogTitle>Delivery Detail</DialogTitle>
					<DialogDescription>
						Every attempt Anchor made for this (event × endpoint) pair, and the
						exact bytes that were signed and sent.
					</DialogDescription>
				</DialogHeader>

				{error ? (
					<FormAlert
						variant="default"
						message={
							getApiErrorMessage(error) ?? "Failed to load the delivery detail"
						}
					/>
				) : null}

				{isLoading ? (
					<div className="flex flex-col gap-2">
						<Skeleton className="h-4 w-full" />
						<Skeleton className="h-4 w-2/3" />
						<Skeleton className="h-24 w-full" />
					</div>
				) : delivery ? (
					<div className="flex flex-col gap-6">
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
							<h3 className="text-sm font-medium">Attempts</h3>
							{data.attempts.length === 0 ? (
								<p className="text-sm text-muted-foreground">
									No attempt has been made yet.
								</p>
							) : (
								<Table>
									<TableHeader>
										<TableRow>
											<TableHead>#</TableHead>
											<TableHead>Status code</TableHead>
											<TableHead>Duration</TableHead>
											<TableHead>Attempted at</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{data.attempts.map((attempt) => (
											<TableRow key={attempt.id}>
												<TableCell>{attempt.attempt_number}</TableCell>
												<TableCell>
													{attempt.status_code ?? (
														<span className="text-muted-foreground">—</span>
													)}
												</TableCell>
												<TableCell>
													{formatDuration(attempt.duration_ms)}
												</TableCell>
												<TableCell>
													{formatDateTime(attempt.attempted_at)}
												</TableCell>
											</TableRow>
										))}
									</TableBody>
								</Table>
							)}
							{data.attempts.map((attempt) =>
								attempt.error || attempt.response_snippet ? (
									<div
										key={`detail-${attempt.id}`}
										className="flex flex-col gap-1 rounded-lg border border-border p-3"
									>
										<p className="text-xs font-medium text-muted-foreground">
											Attempt {attempt.attempt_number}
										</p>
										{attempt.error && (
											<p className="text-sm text-destructive">
												{attempt.error}
											</p>
										)}
										{attempt.response_snippet && (
											<pre className="max-h-40 overflow-auto rounded bg-muted p-2 font-mono text-xs whitespace-pre-wrap">
												{attempt.response_snippet}
											</pre>
										)}
									</div>
								) : null,
							)}
						</div>

						<div className="flex flex-col gap-2">
							<h3 className="text-sm font-medium">Signed request body</h3>
							<pre className="max-h-64 overflow-auto rounded-lg bg-muted p-3 font-mono text-xs whitespace-pre-wrap">
								{data.payload}
							</pre>
						</div>
					</div>
				) : null}

				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						Close
					</Button>
					<Button
						disabled={!delivery || retryDisabled}
						onClick={() => {
							if (!delivery) return;
							retryMutation.mutate({
								path: {
									product_id: productId,
									webhook_endpoint_id: endpointId,
									delivery_id: delivery.id,
								},
							});
						}}
					>
						{retryMutation.isPending ? (
							<>
								<Spinner className="mr-2 text-current" />
								Queueing...
							</>
						) : (
							<>
								<RefreshCw className="mr-2 size-4" />
								{retryMutation.isSuccess ? "Replay queued" : "Retry delivery"}
							</>
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
