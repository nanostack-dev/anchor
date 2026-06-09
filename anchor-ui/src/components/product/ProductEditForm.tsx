import {
	type ProductRequest,
	type ProductResponse,
	zProductApiKeysConfigRequest,
	zProductRequest,
} from "@/client";
import { updateProductMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
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
import { useMutation } from "@tanstack/react-query";
import * as React from "react";
import { toast } from "sonner";
import { z } from "zod";

const productFormSchema = zProductRequest
	.pick({ name: true, description: true })
	.extend({
		apiKeyPrefix: zProductApiKeysConfigRequest.shape.prefix,
	})
	.superRefine((value, ctx) => {
		if (!value.name?.trim()) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: "Product name is required",
				path: ["name"],
			});
		}
		if (!value.apiKeyPrefix?.trim()) {
			ctx.addIssue({
				code: z.ZodIssueCode.custom,
				message: "API key prefix is required",
				path: ["apiKeyPrefix"],
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
	const form = useForm({
		defaultValues: {
			name: product.name || "",
			description: product.description || "",
			apiKeyPrefix: product.config.api_keys.prefix || "anchor",
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
		onSuccess: () => {
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
		const updateData: ProductRequest = {
			name: values.name,
			description: values.description || "",
			config: {
				api_keys: {
					prefix: values.apiKeyPrefix,
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
			name: product.name || "",
			description: product.description || "",
			apiKeyPrefix: product.config.api_keys.prefix || "anchor",
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
						<Tabs defaultValue="api-keys" className="gap-6">
							<TabsList>
								<TabsTrigger value="api-keys">API keys</TabsTrigger>
							</TabsList>
							<TabsContent value="api-keys">
								<FieldGroup>
									<form.Field name="apiKeyPrefix">
										{(field) => (
											<Field
												data-disabled={updateMutation.isPending}
												data-invalid={field.state.meta.errors.length > 0}
											>
												<FieldLabel htmlFor="api-key-prefix">
													API key prefix
												</FieldLabel>
												<Input
													id="api-key-prefix"
													placeholder="anchor"
													value={field.state.value}
													onChange={(e) => field.handleChange(e.target.value)}
													onBlur={field.handleBlur}
													disabled={updateMutation.isPending}
													aria-invalid={field.state.meta.errors.length > 0}
												/>
												<FieldDescription>
													New keys will start with{" "}
													<code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
														{field.state.value || "anchor"}_prd_apikey_
													</code>{" "}
													or{" "}
													<code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">
														{field.state.value || "anchor"}_org_apikey_
													</code>
													. Existing keys remain valid.
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
