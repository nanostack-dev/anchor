import type { WebhookEndpointResponse } from "@/client";
import {
	disableWebhookEndpointMutation,
	enableWebhookEndpointMutation,
	getWebhookEndpointQueryKey,
	listWebhookEndpointsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { toast } from "sonner";

export type EndpointStatusAction = "enable" | "disable";

const actionCopy: Record<
	EndpointStatusAction,
	{ title: string; verb: string; description: string; success: string }
> = {
	enable: {
		title: "Enable Endpoint",
		verb: "Enable",
		description:
			"The endpoint starts receiving deliveries again and its failure streak is cleared. If the receiver is still broken it can be auto-disabled again after sustained failures.",
		success: "Webhook endpoint enabled.",
	},
	disable: {
		title: "Disable Endpoint",
		verb: "Disable",
		description:
			"The endpoint stops accruing new deliveries. Nothing is deleted — the delivery log stays queryable and the endpoint can be enabled again at any time.",
		success: "Webhook endpoint disabled.",
	},
};

interface WebhookEndpointStatusDialogProps {
	productId: string;
	endpoint: WebhookEndpointResponse;
	action: EndpointStatusAction;
	trigger: ReactElement;
	onDone?: () => void;
}

export function WebhookEndpointStatusDialog({
	productId,
	endpoint,
	action,
	trigger,
	onDone,
}: WebhookEndpointStatusDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const copy = actionCopy[action];

	const handleSuccess = () => {
		toast.success(copy.success);
		setOpen(false);
		queryClient.invalidateQueries({
			queryKey: listWebhookEndpointsQueryKey({
				path: { product_id: productId },
			}),
		});
		queryClient.invalidateQueries({
			queryKey: getWebhookEndpointQueryKey({
				path: { product_id: productId, webhook_endpoint_id: endpoint.id },
			}),
		});
		onDone?.();
	};

	const handleError = (error: unknown) => {
		console.error(`Failed to ${action} webhook endpoint:`, error);
		toast.error(
			getApiErrorMessage(error) ??
				`Failed to ${action} the webhook endpoint. Please try again.`,
		);
	};

	const enableMutation = useMutation({
		...enableWebhookEndpointMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});
	const disableMutation = useMutation({
		...disableWebhookEndpointMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});

	const mutation = action === "enable" ? enableMutation : disableMutation;

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent>
				<DialogHeader>
					<DialogTitle>{copy.title}</DialogTitle>
					<DialogDescription>
						{copy.verb} deliveries to "{endpoint.url}"? {copy.description}
					</DialogDescription>
				</DialogHeader>

				{mutation.error ? (
					<FormAlert
						variant="default"
						message={
							getApiErrorMessage(mutation.error) ??
							`Failed to ${action} the endpoint`
						}
					/>
				) : null}

				<DialogFooter>
					<Button
						variant="outline"
						onClick={() => setOpen(false)}
						disabled={mutation.isPending}
					>
						Cancel
					</Button>
					<Button
						onClick={() =>
							mutation.mutate({
								path: {
									product_id: productId,
									webhook_endpoint_id: endpoint.id,
								},
							})
						}
						disabled={mutation.isPending}
					>
						{mutation.isPending ? (
							<>
								<Spinner className="mr-2 text-current" />
								{copy.verb}...
							</>
						) : (
							copy.verb
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
