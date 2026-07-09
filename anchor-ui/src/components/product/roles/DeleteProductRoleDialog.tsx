import type { ProductRoleResponse } from "@/client";
import {
	deleteProductRoleMutation,
	searchProductRolesQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Loader2, Trash2 } from "lucide-react";
import type { ReactElement } from "react";
import { DeleteDialog } from "../../common/dialogs/DeleteDialog";
import { Button } from "../../ui/button";

interface DeleteProductRoleDialogProps {
	productId: string;
	role: ProductRoleResponse;
	trigger?: ReactElement;
	onDeleted?: () => void;
}

export function DeleteProductRoleDialog({
	productId,
	role,
	trigger,
	onDeleted,
}: DeleteProductRoleDialogProps) {
	const queryClient = useQueryClient();
	const rolePermissions = role.permissions ?? [];

	const deleteMutation = useMutation({
		...deleteProductRoleMutation(),
	});

	const handleDelete = async () => {
		await deleteMutation.mutateAsync({
			path: {
				product_id: productId,
				role_id: role.id,
			},
		});
		queryClient.invalidateQueries({
			queryKey: searchProductRolesQueryKey({
				path: { product_id: productId },
				body: {},
			}),
		});
	};

	const defaultTrigger = (
		<Button
			size="icon"
			variant="outlineDestructive"
			disabled={deleteMutation.isPending}
		>
			<span className="sr-only">Delete role</span>
			{deleteMutation.isPending ? (
				<Loader2 className="h-4 w-4 animate-spin" />
			) : (
				<Trash2 className="h-4 w-4" />
			)}
		</Button>
	);

	const warningMessage =
		rolePermissions.length > 0
			? `This role has ${rolePermissions.length} permission${rolePermissions.length !== 1 ? "s" : ""} assigned. Deleting it may affect users who have this role. This will fail if the role is currently assigned to any users.`
			: "This will fail if the role is currently assigned to any users.";

	return (
		<DeleteDialog
			trigger={trigger || defaultTrigger}
			entityType="Product Role"
			entityName={role.name}
			displayFields={[
				{ label: "Role Name", value: role.name },
				{
					label: "Description",
					value: (
						<span className="text-right max-w-[200px] truncate">
							{role.description}
						</span>
					),
					condition: !!role.description,
				},
				{
					label: "Permissions",
					value: (
						<span className="text-sm text-muted-foreground">
							{rolePermissions.length} permission
							{rolePermissions.length !== 1 ? "s" : ""}
						</span>
					),
				},
			]}
			warningMessage={warningMessage}
			onDelete={handleDelete}
			onDeleted={onDeleted}
		/>
	);
}
