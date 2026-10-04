import {
	type ProductRequest,
	type ProductResponse,
	zProductConfigRequest,
	zProductOrganizationApiKeysConfigRequest,
	zProductRequest,
} from "@/client";
import {
	getProductQueryKey,
	updateProductMutation,
} from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { getApiErrorMessage } from "@/lib/api-error";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Field,
	FieldContent,
	FieldDescription,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Switch } from "@nanostackorg/design-system/components/switch";
import {
	Tabs,
	TabsContent,
	TabsList,
	TabsTrigger,
} from "@nanostackorg/design-system/components/tabs";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useForm } from "@tanstack/react-form";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import * as React from "react";
import { z } from "zod";

const productFormSchema = zProductRequest
	.pick({ name: true, description: true })
	.extend({
		protected: zProductConfigRequest.shape.protected,
		organizationApiKeyPrefix:
			zProductOrganizationApiKeysConfigRequest.shape.prefix,
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

interface ProductEditFormProps {
	product: ProductResponse;
	productId: string;
	onSuccess?: () => void;
	onCancel?: () => void;
}

export function ProductEditForm({
	product,
	productId,
	onSuccess,
	onCancel,
}: ProductEditFormProps) {
	const queryClient = useQueryClient();

	const form = useForm({
		defaultValues: {
			protected: product.config.protected ?? false,
			name: product.name || "",
			description: product.description || "",
			organizationApiKeyPrefix:
				product.config.organization_api_keys.prefix || "anchor",
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
			void queryClient.invalidateQueries({
				queryKey: getProductQueryKey({
					path: { product_id: productId },
				}),
			});
			toast.add({ type: "success", title: "Product updated successfully!" });
			onSuccess?.();
		},
		onError: (error) => {
			console.error("Failed to update product:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: "Failed to update product. Please try again.",
				});
			}
		},
	});

	const onSubmit = async (values: ProductFormData) => {
		const updateData: ProductRequest = {
			name: values.name,
			description: values.description || "",
			config: {
				protected: values.protected,
				organization_api_keys: {
					prefix: values.organizationApiKeyPrefix,
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
			protected: product.config.protected ?? false,
			name: product.name || "",
			description: product.description || "",
			organizationApiKeyPrefix:
				product.config.organization_api_keys.prefix || "anchor",
		});
	}, [product, form]);

	return (
		<Box>
			<Box
				as="form"
				onSubmit={(e) => {
					e.preventDefault();
					e.stopPropagation();
					form.handleSubmit();
				}}
				className="flex flex-col gap-6"
			>
				<Tabs defaultValue="details">
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
						<FieldGroup>
							<form.Field name="protected">
								{(field) => (
									<Field
										orientation="horizontal"
										data-disabled={updateMutation.isPending}
									>
										<FieldContent>
											<FieldLabel htmlFor="product-protected">
												Protected product
											</FieldLabel>
											<FieldDescription id="product-protected-description">
												Prevents this product and its associated data from being
												deleted. Turn this off and save before deleting the
												product.
											</FieldDescription>
										</FieldContent>
										<Switch
											id="product-protected"
											checked={field.state.value ?? false}
											aria-describedby="product-protected-description"
											onCheckedChange={field.handleChange}
											onBlur={field.handleBlur}
											disabled={updateMutation.isPending}
										/>
									</Field>
								)}
							</form.Field>
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
											organization API keys. Organization keys created with a
											previous prefix remain valid.
										</FieldDescription>
										<FormValidationError field={field} />
									</Field>
								)}
							</form.Field>
						</FieldGroup>
					</TabsContent>
				</Tabs>

				<Box className="flex justify-end gap-4">
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
								variant="solid"
								tone="brand"
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
				</Box>
			</Box>
		</Box>
	);
}
