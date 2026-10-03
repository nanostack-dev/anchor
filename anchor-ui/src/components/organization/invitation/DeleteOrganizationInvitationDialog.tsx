import { FormAlert } from "@/components/common/FormAlert";
import { getApiErrorMessage } from "@/lib/api-error";
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
import { IconButton } from "@nanostackorg/design-system/components/button";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { toast } from "@nanostackorg/design-system/components/toast";
import { useMutation } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { useState } from "react";

type DeleteOrganizationInvitationDialogProps = {
	email: string;
	onConfirm: () => Promise<unknown>;
};

export function DeleteOrganizationInvitationDialog({
	email,
	onConfirm,
}: DeleteOrganizationInvitationDialogProps) {
	const [open, setOpen] = useState(false);

	const { mutate, isPending, error, reset } = useMutation({
		mutationFn: onConfirm,
		onSuccess: () => {
			toast.add({ type: "success", title: `Invitation for ${email} deleted.` });
			setOpen(false);
		},
	});

	return (
		<AlertDialog
			open={open}
			onOpenChange={(nextOpen) => {
				setOpen(nextOpen);
				if (!nextOpen) reset();
			}}
		>
			<AlertDialogTrigger
				render={
					<IconButton
						tone="critical"
						size="md"
						variant="outline"
						icon={Trash2}
						label={`Delete invitation for ${email}`}
					/>
				}
			/>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete invitation?</AlertDialogTitle>
					<AlertDialogDescription>
						The invitation for {email} is deleted permanently. The invited
						person can no longer accept it.
					</AlertDialogDescription>
				</AlertDialogHeader>

				<FormAlert
					message={
						error
							? getApiErrorMessage(error) || "Failed to delete the invitation."
							: null
					}
				/>

				<AlertDialogFooter>
					<AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
					<AlertDialogAction
						variant="solid"
						tone="critical"
						disabled={isPending}
						onClick={() => mutate()}
					>
						{isPending ? (
							<>
								<Spinner />
								Deleting...
							</>
						) : (
							<>
								<Trash2 />
								Delete
							</>
						)}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
