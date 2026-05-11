import type { ProductRequest } from "@/client";
import { createProductMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage } from "@/lib/api-error";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { z } from "zod";
import { Button } from "../ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "../ui/dialog";
import { Input } from "../ui/input";
import { Label } from "../ui/label";

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
	trigger: React.ReactNode;
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
			toast.success("Product created successfully!");
			setOpen(false);
			form.reset();
			onCreated?.();
		},
		onError: (error) => {
			console.error("Failed to create product:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to create product. Please try again.");
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
			<DialogTrigger asChild>{trigger}</DialogTrigger>
			<DialogContent className="sm:max-w-[450px]">
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
					<div className="space-y-6 py-4">
						<form.Field name="name">
							{(field) => (
								<div className="space-y-2">
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
								</div>
							)}
						</form.Field>

						<form.Field name="description">
							{(field) => (
								<div className="space-y-2">
									<Label htmlFor="description">Description</Label>
									<Textarea
										id="description"
										placeholder="Product description (optional)"
										className=""
										rows={3}
										value={field.state.value || ""}
										onChange={(e) => field.handleChange(e.target.value)}
										onBlur={field.handleBlur}
										disabled={isCreating}
									/>
									<FormValidationError field={field} />
								</div>
							)}
						</form.Field>
					</div>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
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
									type="submit"
									disabled={
										!canSubmit ||
										isSubmitting ||
										!isValid ||
										isValidating ||
										!isDirty ||
										isCreating
									}
									className="h-11"
								>
									{isCreating || isSubmitting ? (
										<div className="flex items-center space-x-2">
											<div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
											<span>Creating...</span>
										</div>
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
