import { FormAlert } from "@/components/common/FormAlert";
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
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { useState } from "react";
import { toast } from "sonner";

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
			toast.success(`Invitation for ${email} deleted.`);
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
					<Button size="icon" variant="outlineDestructive">
						<span className="sr-only">Delete invitation for {email}</span>
						<Trash2 />
					</Button>
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
						variant="destructive"
						disabled={isPending}
						onClick={() => mutate()}
					>
						{isPending ? (
							<>
								<Spinner className="text-current" />
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
