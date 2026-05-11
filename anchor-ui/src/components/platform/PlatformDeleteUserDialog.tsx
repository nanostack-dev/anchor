import { deletePlatformUserMutation } from "@/client/@tanstack/react-query.gen";
import { useMutation } from "@tanstack/react-query";
import { DeleteDialog } from "../common/dialogs/DeleteDialog";

interface PlatformDeleteUserDialogProps {
	userId: string;
	userEmail: string;
	onDeleted: () => void;
	disabled?: boolean;
}

export function PlatformDeleteUserDialog({
	userId,
	userEmail,
	onDeleted,
	disabled = false,
}: PlatformDeleteUserDialogProps) {
	const deleteMutation = useMutation({
		...deletePlatformUserMutation(),
	});

	const handleDelete = async () => {
		await deleteMutation.mutateAsync({
			path: {
				platform_user_id: userId,
			},
		});
	};

	return (
		<DeleteDialog
			entityType="Platform User"
			entityName={userEmail}
			disabled={deleteMutation.isPending || disabled}
			displayFields={[
				{ label: "Email", value: userEmail },
				{ label: "User ID", value: userId },
			]}
			warningMessage="The user will be removed from the platform and will lose access to all resources."
			onDelete={handleDelete}
			onDeleted={onDeleted}
		/>
	);
}
