import type {
	ProductResourcePermissionResponse,
	UpdateProductResourcePermissionRequest,
} from "@/client";
import {
	searchProductPermissionsQueryKey,
	updateProductResourcePermissionMutation,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { PenLine } from "lucide-react";
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

interface EditProductPermissionDialogProps {
	productId: string;
	permission: ProductResourcePermissionResponse;
	trigger?: React.ReactElement;
	onUpdated?: () => void;
}

export function EditProductResourcePermissionDialog({
	productId,
	permission,
	trigger,
	onUpdated,
}: EditProductPermissionDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [formData, setFormData] =
		useState<UpdateProductResourcePermissionRequest>({
			description: permission.description || "",
		});

	const updateMutation = useMutation({
		...updateProductResourcePermissionMutation(),
		onSuccess: () => {
			toast.success("Product resource permission updated successfully!");
			setOpen(false);

			queryClient.invalidateQueries({
				queryKey: searchProductPermissionsQueryKey({
					path: { product_id: productId },
					body: {},
				}),
			});

			onUpdated?.();
		},
		onError: (error) => {
			console.error("Failed to update product permission:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to update product permission. Please try again.");
			}
		},
	});

	const handleSubmit = async (e: React.FormEvent) => {
		e.preventDefault();

		updateMutation.mutate({
			path: {
				product_id: productId,
				permission_name: permission.name,
			},
			body: formData,
		});
	};

	const defaultTrigger = (
		<Button size="icon" variant="outline">
			<span className="sr-only">Edit permission</span>
			<PenLine className="h-4 w-4" />
		</Button>
	);

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger || defaultTrigger} />
			<DialogContent className="sm:max-w-[450px]">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>Edit Product Permission</DialogTitle>
						<DialogDescription>
							Update the permission details. The permission name cannot be
							changed.
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<div className="grid grid-cols-4 items-center gap-4">
							<Label htmlFor="name" className="text-right">
								Name
							</Label>
							<Input
								id="name"
								value={permission.name}
								className="col-span-3"
								disabled
								readOnly
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
						<Button type="submit" disabled={updateMutation.isPending}>
							{updateMutation.isPending ? "Updating..." : "Update Permission"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
