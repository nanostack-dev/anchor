import type { ProductPermissionResponse } from "@/client";
import {
	deleteProductResourcePermissionMutation,
	searchProductResourcePermissionsQueryKey,
	searchProductRolesOptions,
} from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import {
	Alert,
	AlertDescription,
} from "@nanostackorg/design-system/components/alert";
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
} from "@nanostackorg/design-system/components/alert-dialog";
import { Button } from "@nanostackorg/design-system/components/button";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Trash2, TriangleAlert } from "lucide-react";
import { useState } from "react";

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
			<Box as="p" className="text-sm text-muted-foreground">
				Any roles or API keys currently using it will lose this permission
				immediately.
			</Box>
		);
	}

	if (!roles.data) {
		return (
			<Box className="flex items-center gap-2 text-sm text-muted-foreground">
				<Spinner />
				Checking which roles hold it...
			</Box>
		);
	}

	const { items, total } = roles.data;
	const hiddenCount = total - items.length;
	return (
		<Box className="space-y-2">
			{items.length === 0 ? (
				<Box as="p" className="text-sm text-muted-foreground">
					No role holds this permission.
				</Box>
			) : (
				<Alert tone="critical">
					<TriangleAlert />
					<AlertDescription>
						These roles lose it: {items.map((role) => role.name).join(", ")}
						{hiddenCount > 0 && `, and ${hiddenCount} more`}
					</AlertDescription>
				</Alert>
			)}
			<Box as="p" className="text-sm text-muted-foreground">
				API keys currently using it will lose this permission immediately.
			</Box>
		</Box>
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
			toast.add({
				type: "success",
				title: "Product resource permission deleted successfully!",
			});
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
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: "Failed to delete product permission. Please try again.",
				});
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
			tone="critical"
			size="md"
			variant="outline"
			disabled={deleteMutation.isPending}
		>
			<Box as="span" className="sr-only">
				Delete permission
			</Box>
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

				<Box className="my-4 p-4 bg-muted rounded-lg">
					<Box className="space-y-2">
						<Box className="flex justify-between">
							<Box as="span" className="font-medium">
								Permission Name:
							</Box>
							<Box as="span" className="font-mono text-sm">
								{permission.name}
							</Box>
						</Box>
						{permission.description && (
							<Box className="flex justify-between">
								<Box as="span" className="font-medium">
									Description:
								</Box>
								<Box as="span" className="text-right max-w-[200px] truncate">
									{permission.description}
								</Box>
							</Box>
						)}
					</Box>
				</Box>

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
						variant="solid"
						tone="critical"
						onClick={handleDelete}
						disabled={deleteMutation.isPending}
					>
						{deleteMutation.isPending ? (
							<>
								<Spinner />
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
