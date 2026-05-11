import type { ProductResponse } from "@/client";
import { deleteProductMutation } from "@/client/@tanstack/react-query.gen";
import { useMutation } from "@tanstack/react-query";
import { Loader2, Trash2 } from "lucide-react";
import { DeleteDialog } from "../common/dialogs/DeleteDialog";
import { Button } from "../ui/button";

interface ProductDeleteDialogProps {
	product: ProductResponse;
	trigger?: React.ReactNode;
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
			size="icon"
			variant="outlineDestructive"
			disabled={deleteMutation.isPending}
		>
			<span className="sr-only">Delete product</span>
			{deleteMutation.isPending ? (
				<Loader2 className="h-4 w-4 animate-spin" />
			) : (
				<Trash2 className="h-4 w-4" />
			)}
		</Button>
	);

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
						<span className="text-right max-w-[200px] truncate">
							{product.description}
						</span>
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
