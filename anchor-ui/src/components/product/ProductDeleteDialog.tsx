import type { ProductResponse } from "@/client";
import { deleteProductMutation } from "@/client/@tanstack/react-query.gen";
import { Button } from "@nanostackorg/design-system/components/button";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useMutation } from "@tanstack/react-query";
import { Loader2, Trash2 } from "lucide-react";
import { DeleteDialog } from "../common/dialogs/DeleteDialog";

interface ProductDeleteDialogProps {
	product: ProductResponse;
	trigger?: React.ReactElement;
	onDeleted?: () => void;
}

export function ProductDeleteDialog({
	product,
	trigger,
	onDeleted,
}: ProductDeleteDialogProps) {
	const deleteMutation = useMutation({
		...deleteProductMutation(),
	});

	const handleDelete = async () => {
		await deleteMutation.mutateAsync({
			path: { product_id: product.id },
		});
	};

	const defaultTrigger = (
		<Button
			tone="critical"
			size="md"
			variant="outline"
			disabled={deleteMutation.isPending || product.config.protected}
			title={
				product.config.protected
					? "Turn off Protected product in configuration before deleting"
					: undefined
			}
		>
			<Box as="span" className="sr-only">
				{product.config.protected
					? "Product protected from deletion"
					: "Delete product"}
			</Box>
			{deleteMutation.isPending ? (
				<Loader2 className="h-4 w-4 animate-spin" />
			) : (
				<Trash2 className="h-4 w-4" />
			)}
		</Button>
	);

	if (product.config.protected) {
		return defaultTrigger;
	}

	return (
		<DeleteDialog
			trigger={trigger || defaultTrigger}
			entityType="Product"
			entityName={product.name}
			displayFields={[
				{ label: "Product Name", value: product.name },
				{ label: "Product ID", value: product.id },
				{
					label: "Description",
					value: (
						<Box as="span" className="text-right max-w-[200px] truncate">
							{product.description}
						</Box>
					),
					condition: !!product.description,
				},
			]}
			warningMessage="This will permanently remove the product along with all associated data including API keys, users, organizations, and workspaces."
			onDelete={handleDelete}
			onDeleted={onDeleted}
		/>
	);
}
