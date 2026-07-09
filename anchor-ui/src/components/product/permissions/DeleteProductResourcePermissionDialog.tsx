import type { ProductPermissionResponse } from "@/client";
import {
	deleteProductResourcePermissionMutation,
	searchProductResourcePermissionsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader2, Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
	AlertDialogTrigger,
} from "../../ui/alert-dialog";
import { Button } from "../../ui/button";

interface DeleteProductPermissionDialogProps {
	productId: string;
	permission: ProductPermissionResponse;
	trigger?: React.ReactElement;
	onDeleted?: () => void;
}

export function DeleteProductResourcePermissionDialog({
	productId,
	permission,
	trigger,
	onDeleted,
}: DeleteProductPermissionDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);

	const deleteMutation = useMutation({
		...deleteProductResourcePermissionMutation(),
		onSuccess: () => {
			toast.success("Product resource permission deleted successfully!");
			setOpen(false);

			// Invalidate the search query for this product
			queryClient.invalidateQueries({
				queryKey: searchProductResourcePermissionsQueryKey({
					path: { product_id: productId },
					body: {},
				}),
			});

			onDeleted?.();
		},
		onError: (error) => {
			console.error("Failed to delete product permission:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.error(errorMessage);
			} else {
				toast.error("Failed to delete product permission. Please try again.");
			}
		},
	});

	const handleDelete = () => {
		deleteMutation.mutate({
			path: {
				product_id: productId,
				permission_name: permission.name,
			},
		});
	};

	const defaultTrigger = (
		<Button
			size="icon"
			variant="outlineDestructive"
			disabled={deleteMutation.isPending}
		>
			<span className="sr-only">Delete permission</span>
			{deleteMutation.isPending ? (
				<Loader2 className="h-4 w-4 animate-spin" />
			) : (
				<Trash2 className="h-4 w-4" />
			)}
		</Button>
	);

	return (
		<AlertDialog open={open} onOpenChange={setOpen}>
			<AlertDialogTrigger render={trigger || defaultTrigger} />
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete Product Permission</AlertDialogTitle>
					<AlertDialogDescription>
						Are you sure you want to delete the permission "{permission.name}"?
						This action cannot be undone and will permanently remove the
						permission. Any roles or API keys currently using it will lose this
						permission immediately.
					</AlertDialogDescription>
				</AlertDialogHeader>

				<div className="my-4 p-4 bg-muted rounded-lg">
					<div className="space-y-2">
						<div className="flex justify-between">
							<span className="font-medium">Permission Name:</span>
							<span className="font-mono text-sm">{permission.name}</span>
						</div>
						{permission.description && (
							<div className="flex justify-between">
								<span className="font-medium">Description:</span>
								<span className="text-right max-w-[200px] truncate">
									{permission.description}
								</span>
							</div>
						)}
					</div>
				</div>

				<AlertDialogFooter>
					<AlertDialogCancel disabled={deleteMutation.isPending}>
						Cancel
					</AlertDialogCancel>
					<AlertDialogAction
						onClick={handleDelete}
						disabled={deleteMutation.isPending}
					>
						{deleteMutation.isPending ? (
							<>
								<Loader2 className="mr-2 h-4 w-4 animate-spin" />
								Deleting...
							</>
						) : (
							<>
								<Trash2 className="mr-2 h-4 w-4" />
								Delete Permission
							</>
						)}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
