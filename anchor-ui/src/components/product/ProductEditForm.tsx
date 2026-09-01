import {
	type ProductRequest,
	type ProductResponse,
	zProductOrganizationApiKeysConfigRequest,
	zProductRequest,
} from "@/client";
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage } from "@/lib/api-error";
import { useForm } from "@tanstack/react-form";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Check, Copy } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { z } from "zod";

const productFormSchema = zProductRequest
	.pick({ name: true, description: true })
	.extend({
		organizationApiKeyPrefix:
			zProductOrganizationApiKeysConfigRequest.shape.prefix,
		eventsEndpointUrl: z.string(),
	})
	.superRefine((value, ctx) => {
		if (!value.name?.trim()) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: "Product name is required",
				path: ["name"],
			});
		}
		if (!value.organizationApiKeyPrefix?.trim()) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: "Organization API key prefix is required",
				path: ["organizationApiKeyPrefix"],
			});
		}
	});

type ProductFormData = z.infer<typeof productFormSchema>;

interface ProductEditFormProps extends React.ComponentPropsWithoutRef<"div"> {
	product: ProductResponse;
	productId: string;
	onSuccess?: () => void;
	onCancel?: () => void;
}

export function ProductEditForm({
	className,
	product,
	productId,
	onSuccess,
	onCancel,
	...props
}: ProductEditFormProps) {
	const queryClient = useQueryClient();
	const [revealedSecret, setRevealedSecret] = React.useState<string | null>(
		null,
	);
	const [secretCopied, setSecretCopied] = React.useState(false);
	const hadEvents = Boolean(product.config.events?.endpoint_url);

	const form = useForm({
		defaultValues: {
			name: product.name || "",
			description: product.description || "",
			organizationApiKeyPrefix:
				product.config.organization_api_keys.prefix || "anchor",
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
		} as ProductFormData,
		onSubmit: async ({ value }) => {
			const result = productFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(result.data);
		},
		validators: {
			onChange: productFormSchema,
			onSubmit: productFormSchema,
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
			const generatedSecret = updated.config.events?.signing_secret;
			if (generatedSecret) {
				setRevealedSecret(generatedSecret);
				toast.success(
					"Store the event signing secret now. It is not shown again.",
				);
				return;
			}
			toast.success("Product updated successfully!");
			onSuccess?.();
		},
		onError: (error) => {
			console.error("Failed to update product:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to update product. Please try again.");
			}
		},
	});

	const onSubmit = async (values: ProductFormData) => {
		const endpointUrl = values.eventsEndpointUrl.trim();
		const updateData: ProductRequest = {
			name: values.name,
			description: values.description || "",
			config: {
				organization_api_keys: {
					prefix: values.organizationApiKeyPrefix,
				},
				...(endpointUrl || hadEvents
					? {
							events: {
								endpoint_url: endpointUrl,
							},
						}
					: {}),
			},
		};

		await updateMutation.mutateAsync({
			path: { product_id: productId },
			body: updateData,
		});
	};

	React.useEffect(() => {
		form.reset({
			name: product.name || "",
			description: product.description || "",
			organizationApiKeyPrefix:
				product.config.organization_api_keys.prefix || "anchor",
			eventsEndpointUrl: product.config.events?.endpoint_url || "",
		});
	}, [product, form]);

	return (
		<div className={className} {...props}>
			<form
				onSubmit={(e) => {
					e.preventDefault();
					e.stopPropagation();
					form.handleSubmit();
				}}
				className="flex flex-col gap-6"
			>
				<Tabs defaultValue="details" className="gap-6">
					<TabsList>
						<TabsTrigger value="details">Details</TabsTrigger>
						<TabsTrigger value="config">Config</TabsTrigger>
					</TabsList>

					<TabsContent value="details">
						<FieldGroup>
							<form.Field name="name">
								{(field) => (
									<Field
										data-disabled={updateMutation.isPending}
										data-invalid={field.state.meta.errors.length > 0}
									>
										<FieldLabel htmlFor="product-name">Product Name</FieldLabel>
										<Input
											id="product-name"
											placeholder="Enter product name"
											value={field.state.value}
											onChange={(e) => field.handleChange(e.target.value)}
											onBlur={field.handleBlur}
											disabled={updateMutation.isPending}
											aria-invalid={field.state.meta.errors.length > 0}
										/>
										<FormValidationError field={field} />
									</Field>
								)}
							</form.Field>

							<form.Field name="description">
								{(field) => (
									<Field
										data-disabled={updateMutation.isPending}
										data-invalid={field.state.meta.errors.length > 0}
									>
										<FieldLabel htmlFor="product-description">
											Description
										</FieldLabel>
										<Textarea
											id="product-description"
											placeholder="Enter product description (optional)"
											rows={4}
											value={field.state.value || ""}
											onChange={(e) => field.handleChange(e.target.value)}
											onBlur={field.handleBlur}
											disabled={updateMutation.isPending}
											aria-invalid={field.state.meta.errors.length > 0}
										/>
										<FormValidationError field={field} />
									</Field>
								)}
							</form.Field>
						</FieldGroup>
					</TabsContent>

					<TabsContent value="config">
						<Tabs defaultValue="organization-api-keys" className="gap-6">
							<TabsList>
								<TabsTrigger value="organization-api-keys">
									Organization API keys
								</TabsTrigger>
								<TabsTrigger value="events">Events</TabsTrigger>
							</TabsList>
							<TabsContent value="organization-api-keys">
								<FieldGroup>
									<form.Field name="organizationApiKeyPrefix">
										{(field) => (
											<Field
												data-disabled={updateMutation.isPending}
												data-invalid={field.state.meta.errors.length > 0}
											>
												<FieldLabel htmlFor="organization-api-key-prefix">
													Organization API key prefix
												</FieldLabel>
												<Input
													id="organization-api-key-prefix"
													placeholder="anchor"
													value={field.state.value}
													onChange={(e) => field.handleChange(e.target.value)}
													onBlur={field.handleBlur}
													disabled={updateMutation.isPending}
													aria-invalid={field.state.meta.errors.length > 0}
												/>
												<FieldDescription>
													Changing this prefix only affects newly generated
													organization API keys. Organization keys created with
													a previous prefix remain valid.
												</FieldDescription>
												<FormValidationError field={field} />
											</Field>
										)}
									</form.Field>
								</FieldGroup>
							</TabsContent>
							<TabsContent value="events">
								<FieldGroup>
									{revealedSecret ? (
										<Alert>
											<AlertTitle>Signing secret</AlertTitle>
											<AlertDescription>
												Store this secret now. Later reads return only the
												obfuscated marker.
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
															void navigator.clipboard.writeText(
																revealedSecret,
															);
															setSecretCopied(true);
															window.setTimeout(
																() => setSecretCopied(false),
																1500,
															);
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
													Anchor POSTs signed product events here. Leave empty
													to clear the endpoint. Production requires HTTPS.
													Anchor mints the signing secret on first save.
													{product.config.events?.signing_secret_obfuscated
														? ` Stored secret: ${product.config.events.signing_secret_obfuscated}.`
														: ""}
												</FieldDescription>
												<FormValidationError field={field} />
											</Field>
										)}
									</form.Field>
								</FieldGroup>
							</TabsContent>
						</Tabs>
					</TabsContent>
				</Tabs>

				<div className="flex justify-end gap-4">
					<Button
						type="button"
						variant="outline"
						onClick={onCancel}
						disabled={updateMutation.isPending}
					>
						Cancel
					</Button>
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
										Updating...
									</>
								) : (
									"Update Product"
								)}
							</Button>
						)}
					</form.Subscribe>
				</div>
			</form>
		</div>
	);
}
