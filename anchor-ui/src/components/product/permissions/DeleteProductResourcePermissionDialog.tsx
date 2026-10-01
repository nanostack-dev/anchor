import type { ProductPermissionResponse } from "@/client";
import {
	deleteProductResourcePermissionMutation,
	searchProductResourcePermissionsQueryKey,
	searchProductRolesOptions,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Trash2, TriangleAlert } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";
import { Alert, AlertDescription } from "../../ui/alert";
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
import { Spinner } from "../../ui/spinner";

const LISTED_ROLES_LIMIT = 10;

function AffectedRoles({
	productId,
	permissionName,
	open,
}: {
	productId: string;
	permissionName: string;
	open: boolean;
}) {
	const roles = useQuery({
		...searchProductRolesOptions({
			path: { product_id: productId },
			body: {
				filter: { permissions: [permissionName] },
				pagination: { limit: LISTED_ROLES_LIMIT, offset: 0 },
			},
		}),
		enabled: open,
	});

	if (roles.isError) {
		return (
			<p className="text-sm text-muted-foreground">
				Any roles or API keys currently using it will lose this permission
				immediately.
			</p>
		);
	}

	if (!roles.data) {
		return (
			<div className="flex items-center gap-2 text-sm text-muted-foreground">
				<Spinner />
				Checking which roles hold it...
			</div>
		);
	}

	const { items, total } = roles.data;
	const hiddenCount = total - items.length;
	return (
		<div className="space-y-2">
			{items.length === 0 ? (
				<p className="text-sm text-muted-foreground">
					No role holds this permission.
				</p>
			) : (
				<Alert variant="destructive">
					<TriangleAlert />
					<AlertDescription>
						These roles lose it: {items.map((role) => role.name).join(", ")}
						{hiddenCount > 0 && `, and ${hiddenCount} more`}
					</AlertDescription>
				</Alert>
			)}
			<p className="text-sm text-muted-foreground">
				API keys currently using it will lose this permission immediately.
			</p>
		</div>
	);
}

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
			{deleteMutation.isPending ? <Spinner /> : <Trash2 className="size-4" />}
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
						permission.
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

				<AffectedRoles
					productId={productId}
					permissionName={permission.name}
					open={open}
				/>

				<AlertDialogFooter>
					<AlertDialogCancel disabled={deleteMutation.isPending}>
						Cancel
					</AlertDialogCancel>
					<AlertDialogAction
						variant="destructive"
						onClick={handleDelete}
						disabled={deleteMutation.isPending}
					>
						{deleteMutation.isPending ? (
							<>
								<Spinner className="text-current" />
								Deleting...
							</>
						) : (
							<>
								<Trash2 className="size-4" />
								Delete Permission
							</>
						)}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
