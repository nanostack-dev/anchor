import type { ProductRequest } from "@/client";
import { createProductMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { getApiErrorMessage } from "@/lib/api-error";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@nanostackorg/design-system/components/dialog";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { z } from "zod";

const productFormSchema = z.object({
	name: z
		.string()
		.min(1, "Product name is required")
		.min(2, "Product name must be at least 2 characters")
		.max(100, "Product name must be less than 100 characters")
		.trim(),
	description: z
		.string()
		.max(500, "Description must be less than 500 characters")
		.trim()
		.optional(),
});

type ProductFormData = z.infer<typeof productFormSchema>;

interface ProductCreateDialogProps {
	trigger: React.ReactElement;
	onCreated?: () => void;
}

export function ProductCreateDialog({
	trigger,
	onCreated,
}: ProductCreateDialogProps) {
	const [open, setOpen] = useState(false);

	const form = useForm({
		defaultValues: {
			name: "",
			description: "",
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

	const { mutate: createProduct, isPending: isCreating } = useMutation({
		...createProductMutation(),
		onSuccess: () => {
			toast.add({ type: "success", title: "Product created successfully!" });
			setOpen(false);
			form.reset();
			onCreated?.();
		},
		onError: (error) => {
			console.error("Failed to create product:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: "Failed to create product. Please try again.",
				});
			}
		},
	});

	const onSubmit = async (values: ProductFormData) => {
		const requestData: ProductRequest = {
			name: values.name,
			description: values.description || "",
		};

		createProduct({
			body: requestData,
		});
	};

	const handleOpenChange = (newOpen: boolean) => {
		setOpen(newOpen);
		if (!newOpen) {
			form.reset();
		}
	};

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger render={trigger} />
			<DialogContent>
				<form
					onSubmit={(e) => {
						e.preventDefault();
						e.stopPropagation();
						form.handleSubmit();
					}}
				>
					<DialogHeader>
						<DialogTitle>Create Product</DialogTitle>
						<DialogDescription>
							Create a new product. Fill in the details below.
						</DialogDescription>
					</DialogHeader>
					<Box className="space-y-6 py-4">
						<form.Field name="name">
							{(field) => (
								<Box className="space-y-2">
									<Label htmlFor="name">Product Name</Label>
									<Input
										id="name"
										placeholder="Enter product name"
										value={field.state.value}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isCreating}
									/>
									<FormValidationError field={field} />
								</Box>
							)}
						</form.Field>

						<form.Field name="description">
							{(field) => (
								<Box className="space-y-2">
									<Label htmlFor="description">Description</Label>
									<Textarea
										id="description"
										placeholder="Product description (optional)"
										rows={3}
										value={field.state.value || ""}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isCreating}
									/>
									<FormValidationError field={field} />
								</Box>
							)}
						</form.Field>
					</Box>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => handleOpenChange(false)}
							disabled={isCreating}
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
										isCreating
									}
								>
									{isCreating || isSubmitting ? (
										<Box className="flex items-center gap-2">
											<Spinner />
											<span>Creating...</span>
										</Box>
									) : (
										<span>Create Product</span>
									)}
								</Button>
							)}
						</form.Subscribe>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
