import type { WebhookEndpointResponse } from "@/client";
import {
	deleteWebhookEndpointMutation,
	listWebhookEndpointsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { DeleteDialog } from "@/components/common/dialogs/DeleteDialog";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { EventTypeChips } from "./EventTypeChips";
import { endpointStatusLabel, endpointStatusTone } from "./webhook-display";

interface DeleteWebhookEndpointDialogProps {
	productId: string;
	endpoint: WebhookEndpointResponse;
	trigger?: ReactElement;
	onDeleted?: () => void;
}

export function DeleteWebhookEndpointDialog({
	productId,
	endpoint,
	trigger,
	onDeleted,
}: DeleteWebhookEndpointDialogProps) {
	const queryClient = useQueryClient();
	const deleteMutation = useMutation({ ...deleteWebhookEndpointMutation() });

	const handleDelete = async () => {
		await deleteMutation.mutateAsync({
			path: { product_id: productId, webhook_endpoint_id: endpoint.id },
		});
		queryClient.invalidateQueries({
			queryKey: listWebhookEndpointsQueryKey({
				path: { product_id: productId },
			}),
		});
	};

	return (
		<DeleteDialog
			trigger={trigger}
			entityType="Webhook Endpoint"
			entityName={endpoint.description?.trim() || endpoint.url}
			displayFields={[
				{
					label: "URL",
					value: <span className="font-mono text-sm">{endpoint.url}</span>,
				},
				{
					label: "Status",
					value: (
						<StatusBadge tone={endpointStatusTone[endpoint.status]}>
							{endpointStatusLabel[endpoint.status]}
						</StatusBadge>
					),
				},
				{
					label: "Event types",
					value: <EventTypeChips eventTypes={endpoint.event_types} max={4} />,
				},
			]}
			warningMessage="Deleting removes the endpoint, its signing secrets and its delivery log. To stop deliveries while keeping the history, disable the endpoint instead."
			onDelete={handleDelete}
			onDeleted={onDeleted}
		/>
	);
}
