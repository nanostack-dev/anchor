import type { ProductRequest, ProductResponse } from "@/client";
import { updateProductMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage } from "@/lib/api-error";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { Loader2 } from "lucide-react";
import * as React from "react";
import { toast } from "sonner";
import { z } from "zod";

const productFormSchema = z.object({
	name: z
		.string()
		.min(1, "Product name is required")
		.max(255, "Product name must be less than 255 characters"),
	description: z
		.string()
		.max(1000, "Description must be less than 1000 characters")
		.optional()
		.nullable(),
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
		} as ProductFormData,
		onSubmit: async ({ value }) => {
			const result = productFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(value);
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
		};

		updateMutation.mutate({
			path: { product_id: productId },
			body: updateData,
		});
	};

	React.useEffect(() => {
		form.reset({
			name: product.name || "",
			description: product.description || "",
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
				className="space-y-6"
			>
				<form.Field name="name">
					{(field) => (
						<div className="space-y-2">
							<Label>Product Name</Label>
							<Input
								placeholder="Enter product name"
								value={field.state.value}
								onChange={(e) => field.handleChange(e.target.value)}
								onBlur={field.handleBlur}
								disabled={updateMutation.isPending}
							/>
							<FormValidationError field={field} />
						</div>
					)}
				</form.Field>

				<form.Field name="description">
					{(field) => (
						<div className="space-y-2">
							<Label>Description</Label>
							<Textarea
								placeholder="Enter product description (optional)"
								rows={4}
								value={field.state.value || ""}
								onChange={(e) => field.handleChange(e.target.value)}
								onBlur={field.handleBlur}
								disabled={updateMutation.isPending}
							/>
							<FormValidationError field={field} />
						</div>
					)}
				</form.Field>

				<div className="flex justify-end space-x-4">
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
										<Loader2 className="mr-2 h-4 w-4 animate-spin" />
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
