import type { ProductApiKeyResponse } from "@/client";
import { deleteProductApiKeyMutation } from "@/client/@tanstack/react-query.gen";
import { DeleteDialog } from "@/components/common/dialogs/DeleteDialog";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

interface DeleteProductAPIKeyDialogProps {
	productId: string;
	apiKey: ProductApiKeyResponse;
	onDeleted?: () => void;
}

export function DeleteProductAPIKeyDialog({
	productId,
	apiKey,
	onDeleted,
}: DeleteProductAPIKeyDialogProps) {
	const queryClient = useQueryClient();

	const deleteMutation = useMutation({
		...deleteProductApiKeyMutation(),
	});

	const handleDelete = async () => {
		await deleteMutation.mutateAsync({
			path: {
				product_id: productId,
				api_key_id: apiKey.id,
			},
		});

		queryClient.invalidateQueries({ queryKey: ["searchProductAPIKeys"] });

		toast.success("🗑️ API Key deleted successfully!", {
			description: `${apiKey.name} has been permanently removed`,
		});
	};

	return (
		<DeleteDialog
			entityType="API Key"
			entityName={apiKey.name}
			displayFields={[
				{ label: "Name", value: apiKey.name },
				{
					label: "Description",
					value: apiKey.description,
					condition: !!apiKey.description,
				},
				{ label: "Key ID", value: apiKey.id },
				{
					label: "Created At",
					value: new Date(apiKey.created_at).toLocaleString(),
				},
				{
					label: "Permissions",
					value:
						apiKey.permissions.map((v) => v.permission_name).join(", ") ||
						"None",
				},
			]}
			warningMessage="This will permanently delete the API key and revoke all access."
			onDelete={handleDelete}
			onDeleted={onDeleted}
		/>
	);
}
