import {
	type WebhookEndpointRequest,
	type WebhookEndpointResponse,
	type WebhookEndpointUpdateRequest,
	zWebhookEndpointRequest,
	zWebhookEndpointUpdateRequest,
} from "@/client";
import {
	createWebhookEndpointMutation,
	listWebhookEndpointsQueryKey,
	updateWebhookEndpointMutation,
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ReactElement, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { EventTypePicker } from "./EventTypePicker";
import { classifyWebhookUrl } from "./webhook-url";

const createFormSchema = zWebhookEndpointRequest.superRefine((val, ctx) => {
	const urlVerdict = classifyWebhookUrl(val.url ?? "");
	if (urlVerdict.error) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: urlVerdict.error,
			path: ["url"],
		});
	}
	if (!val.event_types || val.event_types.length === 0) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Subscribe to at least one event type",
			path: ["event_types"],
		});
	}
});

const updateFormSchema = zWebhookEndpointUpdateRequest.superRefine(
	(val, ctx) => {
		const urlVerdict = classifyWebhookUrl(val.url ?? "");
		if (urlVerdict.error) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: urlVerdict.error,
				path: ["url"],
			});
		}
		if (!val.event_types || val.event_types.length === 0) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: "Subscribe to at least one event type",
				path: ["event_types"],
			});
		}
	},
);

interface WebhookEndpointDialogProps {
	productId: string;
	trigger: ReactElement;
	mode?: "create" | "edit";
	existingEndpoint?: WebhookEndpointResponse;
	/** Receives the one-time plaintext secret from a successful creation. */
	onCreated?: (endpoint: WebhookEndpointResponse, secret: string) => void;
}

interface EndpointFormState {
	url: string;
	description: string;
	eventTypes: string[];
}

const emptyForm = (): EndpointFormState => ({
	url: "",
	description: "",
	eventTypes: [],
});

export function WebhookEndpointDialog({
	productId,
	trigger,
	mode = "create",
	existingEndpoint,
	onCreated,
}: WebhookEndpointDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [formData, setFormData] = useState<EndpointFormState>(emptyForm);
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

	const isEditMode = mode === "edit";

	useEffect(() => {
		if (!open) return;
		if (isEditMode && existingEndpoint) {
			setFormData({
				url: existingEndpoint.url,
				description: existingEndpoint.description ?? "",
				eventTypes: [...existingEndpoint.event_types],
			});
		} else {
			setFormData(emptyForm());
		}
		setFieldErrors({});
	}, [open, isEditMode, existingEndpoint]);

	const urlWarning = useMemo(
		() => classifyWebhookUrl(formData.url).warning,
		[formData.url],
	);

	const invalidateEndpoints = () => {
		queryClient.invalidateQueries({
			queryKey: listWebhookEndpointsQueryKey({
				path: { product_id: productId },
			}),
		});
	};

	const handleError = (error: unknown) => {
		console.error(
			`Failed to ${isEditMode ? "update" : "create"} webhook endpoint:`,
			error,
		);
		toast.error(
			getApiErrorMessage(error) ??
				`Failed to ${isEditMode ? "update" : "create"} the webhook endpoint. Please try again.`,
		);
	};

	const createMutation = useMutation({
		...createWebhookEndpointMutation(),
		onSuccess: (data) => {
			setOpen(false);
			invalidateEndpoints();
			// The secret exists in memory exactly once; hand it straight to the
			// reveal dialog rather than showing a toast that can be missed.
			onCreated?.(data.endpoint, data.secret);
		},
		onError: handleError,
	});

	const updateMutation = useMutation({
		...updateWebhookEndpointMutation(),
		onSuccess: () => {
			toast.success("Webhook endpoint updated.");
			setOpen(false);
			invalidateEndpoints();
		},
		onError: handleError,
	});

	const isLoading = createMutation.isPending || updateMutation.isPending;

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();

		const errors: Record<string, string> = {};
		const description = formData.description.trim() || null;
		const url = formData.url.trim();

		if (isEditMode && existingEndpoint) {
			const payload: WebhookEndpointUpdateRequest = {
				url,
				description,
				event_types: formData.eventTypes,
			};
			const result = updateFormSchema.safeParse(payload);
			if (!result.success) {
				for (const issue of result.error.issues) {
					const field = String(issue.path[0] ?? "form");
					errors[field] ??= issue.message;
				}
			}
			setFieldErrors(errors);
			if (Object.keys(errors).length > 0) return;

			updateMutation.mutate({
				path: {
					product_id: productId,
					webhook_endpoint_id: existingEndpoint.id,
				},
				body: payload,
			});
			return;
		}

		const payload: WebhookEndpointRequest = {
			url,
			description,
			event_types: formData.eventTypes,
		};
		const result = createFormSchema.safeParse(payload);
		if (!result.success) {
			for (const issue of result.error.issues) {
				const field = String(issue.path[0] ?? "form");
				errors[field] ??= issue.message;
			}
		}
		setFieldErrors(errors);
		if (Object.keys(errors).length > 0) return;

		createMutation.mutate({ path: { product_id: productId }, body: payload });
	};

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-[640px]">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>
							{isEditMode ? "Edit Webhook Endpoint" : "Create Webhook Endpoint"}
						</DialogTitle>
						<DialogDescription>
							{isEditMode
								? "Update where events are delivered and which event types this endpoint subscribes to. The signing secret is unaffected."
								: "Anchor POSTs signed events to this URL. The signing secret is shown once, right after creation."}
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<div className="grid gap-2">
							<Label htmlFor="webhook-url">Endpoint URL</Label>
							<Input
								id="webhook-url"
								value={formData.url}
								onChange={(e) =>
									setFormData((prev) => ({ ...prev, url: e.target.value }))
								}
								placeholder="https://example.com/webhooks/anchor"
								className="font-mono"
								disabled={isLoading}
							/>
							<p className="text-xs text-muted-foreground">
								HTTPS only. Redirects are never followed, and loopback, private
								and cloud-metadata addresses are refused after DNS resolution.
							</p>
							{fieldErrors.url && (
								<p className="text-sm text-destructive">{fieldErrors.url}</p>
							)}
						</div>

						{urlWarning && !fieldErrors.url && (
							<FormAlert variant="warning" message={urlWarning} />
						)}

						<div className="grid gap-2">
							<Label htmlFor="webhook-description">Description</Label>
							<Textarea
								id="webhook-description"
								value={formData.description}
								onChange={(e) =>
									setFormData((prev) => ({
										...prev,
										description: e.target.value,
									}))
								}
								placeholder="What this endpoint is for (optional)"
								disabled={isLoading}
							/>
							{fieldErrors.description && (
								<p className="text-sm text-destructive">
									{fieldErrors.description}
								</p>
							)}
						</div>

						<div className="grid gap-2">
							<Label>Event types</Label>
							<EventTypePicker
								value={formData.eventTypes}
								onChange={(eventTypes) =>
									setFormData((prev) => ({ ...prev, eventTypes }))
								}
								disabled={isLoading}
							/>
							{fieldErrors.event_types && (
								<p className="text-sm text-destructive">
									{fieldErrors.event_types}
								</p>
							)}
						</div>
					</div>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
							disabled={isLoading}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={isLoading}>
							{isLoading ? (
								<>
									<Spinner className="mr-2 text-current" />
									{isEditMode ? "Saving..." : "Creating..."}
								</>
							) : isEditMode ? (
								"Save Changes"
							) : (
								"Create Endpoint"
							)}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
