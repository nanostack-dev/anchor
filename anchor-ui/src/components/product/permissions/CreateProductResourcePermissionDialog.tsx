import type { CreateProductResourcePermissionRequest } from "@/client";
import {
	createProductResourcePermissionMutation,
	searchProductResourcePermissionsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { Button } from "../../ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "../../ui/dialog";
import { Input } from "../../ui/input";
import { Label } from "../../ui/label";
import { Textarea } from "../../ui/textarea";

interface CreateProductPermissionDialogProps {
	productId: string;
	trigger: React.ReactNode;
	onCreated?: () => void;
}

export function CreateProductResourcePermissionDialog({
	productId,
	trigger,
	onCreated,
}: CreateProductPermissionDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [formData, setFormData] =
		useState<CreateProductResourcePermissionRequest>({
			name: "",
			description: "",
		});

	const createMutation = useMutation({
		...createProductResourcePermissionMutation(),
		onSuccess: () => {
			toast.success("Product resource permission created successfully!");
			setOpen(false);
			setFormData({ name: "", description: "" });

			// Invalidate the search query for this product
			queryClient.invalidateQueries({
				queryKey: searchProductResourcePermissionsQueryKey({
					path: { product_id: productId },
					body: {},
				}),
			});

			onCreated?.();
		},
		onError: (error) => {
			console.error("Failed to create product permission:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to create product permission. Please try again.");
			}
		},
	});

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();

		createMutation.mutate({
			path: { product_id: productId },
			body: formData,
		});
	};

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>{trigger}</DialogTrigger>
			<DialogContent className="sm:max-w-[450px]">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>Create Product Permission</DialogTitle>
						<DialogDescription>
							Create a new product permission. Fill in the details below.
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<div className="grid grid-cols-4 items-center gap-4">
							<Label htmlFor="name" className="text-right">
								Name
							</Label>
							<Input
								id="name"
								value={formData.name}
								onChange={(e) =>
									setFormData((prev) => ({ ...prev, name: e.target.value }))
								}
								className="col-span-3"
								placeholder="Permission name (e.g., users:read)"
								required
							/>
						</div>
						<div className="grid grid-cols-4 items-center gap-4">
							<Label htmlFor="description" className="text-right">
								Description
							</Label>
							<Textarea
								id="description"
								value={formData.description || ""}
								onChange={(e) =>
									setFormData((prev) => ({
										...prev,
										description: e.target.value,
									}))
								}
								className="col-span-3"
								placeholder="Permission description (optional)"
								rows={3}
							/>
						</div>
					</div>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
						>
							Cancel
						</Button>
						<Button
							type="submit"
							disabled={createMutation.isPending || !formData.name.trim()}
						>
							{createMutation.isPending ? "Creating..." : "Create Permission"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
