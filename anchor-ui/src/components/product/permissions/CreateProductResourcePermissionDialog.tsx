import type { CreateProductResourcePermissionRequest } from "@/client";
import {
	createProductResourcePermissionMutation,
	searchProductResourcePermissionsQueryKey,
} from "@/client/@tanstack/react-query.gen";
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
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

interface CreateProductPermissionDialogProps {
	productId: string;
	trigger: React.ReactElement;
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
			toast.add({
				type: "success",
				title: "Product resource permission created successfully!",
			});
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
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: "Failed to create product permission. Please try again.",
				});
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
			<DialogTrigger render={trigger} />
			<DialogContent>
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>Create Product Permission</DialogTitle>
						<DialogDescription>
							Create a new product permission. Fill in the details below.
						</DialogDescription>
					</DialogHeader>
					<Box className="grid gap-4 py-4">
						<Box className="grid grid-cols-4 items-center gap-4">
							<Label htmlFor="name">Name</Label>
							<Input
								id="name"
								value={formData.name}
								onChange={(e) =>
									setFormData((prev) => ({ ...prev, name: e.target.value }))
								}
								placeholder="Permission name (e.g., users:read)"
								required
							/>
						</Box>
						<Box className="grid grid-cols-4 items-center gap-4">
							<Label htmlFor="description">Description</Label>
							<Textarea
								id="description"
								value={formData.description || ""}
								onChange={(e) =>
									setFormData((prev) => ({
										...prev,
										description: e.target.value,
									}))
								}
								placeholder="Permission description (optional)"
								rows={3}
							/>
						</Box>
					</Box>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
						>
							Cancel
						</Button>
						<Button
							variant="solid"
							tone="brand"
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
