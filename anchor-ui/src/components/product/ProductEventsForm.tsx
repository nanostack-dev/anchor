import type { ProductRequest, ProductResponse } from "@/client";
import {
	getProductQueryKey,
	updateProductMutation,
} from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
	Field,
	FieldDescription,
	FieldGroup,
	FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { useForm } from "@tanstack/react-form";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check, Copy } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { z } from "zod";

const eventsFormSchema = z.object({
	eventsEndpointUrl: z.string(),
});

type EventsFormData = z.infer<typeof eventsFormSchema>;

interface ProductEventsFormProps {
	product: ProductResponse;
	productId: string;
	onSaved?: () => void;
}

export function ProductEventsForm({
	product,
	productId,
	onSaved,
}: ProductEventsFormProps) {
	const queryClient = useQueryClient();
	const [revealedSecret, setRevealedSecret] = React.useState<string | null>(
		null,
	);
	const [secretCopied, setSecretCopied] = React.useState(false);
	const hadEvents = Boolean(product.config.events?.endpoint_url);

	const form = useForm({
		defaultValues: {
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
		} as EventsFormData,
		onSubmit: async ({ value }) => {
			const result = eventsFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(result.data);
		},
		validators: {
			onChange: eventsFormSchema,
			onSubmit: eventsFormSchema,
		},
	});

	const updateMutation = useMutation({
		...updateProductMutation(),
		onSuccess: (updated) => {
			void queryClient.invalidateQueries({
				queryKey: getProductQueryKey({
					path: { product_id: productId },
				}),
			});
			onSaved?.();
			const generatedSecret = updated.config.events?.signing_secret;
			if (generatedSecret) {
				setRevealedSecret(generatedSecret);
				toast.success(
					"Store the event signing secret now. It is not shown again.",
				);
				return;
			}
			if (updated.config.events?.endpoint_url) {
				toast.success("Event endpoint saved.");
				return;
			}
			toast.success("Event endpoint cleared.");
		},
		onError: (error) => {
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to save the event endpoint. Please try again.");
			}
		},
	});

	const onSubmit = async (values: EventsFormData) => {
		const endpointUrl = values.eventsEndpointUrl.trim();
		if (!endpointUrl && !hadEvents) {
			return;
		}
		const updateData: ProductRequest = {
			name: product.name,
			config: {
				organization_api_keys: {
					prefix: product.config.organization_api_keys.prefix,
				},
				events: {
					endpoint_url: endpointUrl,
				},
			},
		};

		await updateMutation.mutateAsync({
			path: { product_id: productId },
			body: updateData,
		});
	};

	React.useEffect(() => {
		form.reset({
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
		});
	}, [product, form]);

	return (
		<form
			onSubmit={(e) => {
				e.preventDefault();
				e.stopPropagation();
				form.handleSubmit();
			}}
			className="flex w-full flex-col gap-6"
		>
			<FieldGroup>
				{revealedSecret ? (
					<Alert>
						<AlertTitle>Signing secret</AlertTitle>
						<AlertDescription>
							Store this secret now. Later reads return only the obfuscated
							marker.
							<div className="mt-2 flex items-center gap-2">
								<code className="truncate font-mono text-xs">
									{revealedSecret}
								</code>
								<Button
									type="button"
									variant="ghost"
									size="sm"
									className="size-6 shrink-0 p-0"
									onClick={() => {
										void navigator.clipboard.writeText(revealedSecret);
										setSecretCopied(true);
										window.setTimeout(() => setSecretCopied(false), 1500);
									}}
								>
									{secretCopied ? (
										<Check className="size-3 text-success" />
									) : (
										<Copy className="size-3 text-muted-foreground" />
									)}
								</Button>
							</div>
						</AlertDescription>
					</Alert>
				) : null}
				<form.Field name="eventsEndpointUrl">
					{(field) => (
						<Field
							data-disabled={updateMutation.isPending}
							data-invalid={field.state.meta.errors.length > 0}
						>
							<FieldLabel htmlFor="events-endpoint-url">
								Event endpoint URL
							</FieldLabel>
							<Input
								id="events-endpoint-url"
								placeholder="https://example.com/anchor/events"
								value={field.state.value}
								onChange={(e) => field.handleChange(e.target.value)}
								onBlur={field.handleBlur}
								disabled={updateMutation.isPending}
								aria-invalid={field.state.meta.errors.length > 0}
							/>
							<FieldDescription>
								Anchor POSTs signed product events here. Leave empty to clear
								the endpoint. Production requires HTTPS. Anchor mints the
								signing secret on first save.
								{product.config.events?.signing_secret_obfuscated
									? ` Stored secret: ${product.config.events.signing_secret_obfuscated}.`
									: ""}
							</FieldDescription>
							<FormValidationError field={field} />
						</Field>
					)}
				</form.Field>
			</FieldGroup>

			<div className="flex justify-end">
				<form.Subscribe
					selector={(state) => [
						state.canSubmit,
						state.isSubmitting,
						state.isDirty,
						state.isValidating,
						state.isValid,
					]}
				>
					{([canSubmit, isSubmitting, isDirty, isValidating, isValid]) => (
						<Button
							type="submit"
							disabled={
								!canSubmit ||
								isSubmitting ||
								!isValid ||
								isValidating ||
								!isDirty ||
								updateMutation.isPending
							}
						>
							{updateMutation.isPending || isSubmitting ? (
								<>
									<Spinner data-icon="inline-start" />
									Saving...
								</>
							) : (
								"Save endpoint"
							)}
						</Button>
					)}
				</form.Subscribe>
			</div>
		</form>
	);
}
