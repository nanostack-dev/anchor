import type { WebhookEndpointResponse } from "@/client";
import {
	listWebhookEndpointsQueryKey,
	rotateWebhookEndpointSecretMutation,
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

interface RotateWebhookSecretDialogProps {
	productId: string;
	endpoint: WebhookEndpointResponse;
	trigger: ReactElement;
	/** Receives the one-time plaintext secret from a successful rotation. */
	onRotated: (secret: string) => void;
}

export function RotateWebhookSecretDialog({
	productId,
	endpoint,
	trigger,
	onRotated,
}: RotateWebhookSecretDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);

	const rotateMutation = useMutation({
		...rotateWebhookEndpointSecretMutation(),
		onSuccess: (data) => {
			setOpen(false);
			queryClient.invalidateQueries({
				queryKey: listWebhookEndpointsQueryKey({
					path: { product_id: productId },
				}),
			});
			onRotated(data.secret);
		},
		onError: (error: unknown) => {
			console.error("Failed to rotate webhook signing secret:", error);
			toast.error(
				getApiErrorMessage(error) ??
					"Failed to rotate the signing secret. Please try again.",
			);
		},
	});

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Rotate Signing Secret</DialogTitle>
					<DialogDescription>
						Issue a new signing secret for "{endpoint.url}"? The new secret is
						shown once, immediately after rotation, and never again.
					</DialogDescription>
				</DialogHeader>

				<div className="rounded-lg border border-border bg-muted p-3">
					<p className="text-sm text-muted-foreground">
						The current secret keeps signing for <strong>24 hours</strong>.
						Until it expires both signatures ride in the space-delimited{" "}
						<span className="font-mono">webhook-signature</span> header, so your
						receiver can accept either while you roll it over — no coordination
						and no downtime required.
					</p>
				</div>

				{rotateMutation.error ? (
					<FormAlert
						variant="default"
						message={
							getApiErrorMessage(rotateMutation.error) ??
							"Failed to rotate the signing secret"
						}
					/>
				) : null}

				<DialogFooter>
					<Button
						variant="outline"
						onClick={() => setOpen(false)}
						disabled={rotateMutation.isPending}
					>
						Cancel
					</Button>
					<Button
						onClick={() =>
							rotateMutation.mutate({
								path: {
									product_id: productId,
									webhook_endpoint_id: endpoint.id,
								},
							})
						}
						disabled={rotateMutation.isPending}
					>
						{rotateMutation.isPending ? (
							<>
								<Spinner className="mr-2 text-current" />
								Rotating...
							</>
						) : (
							"Rotate Secret"
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
