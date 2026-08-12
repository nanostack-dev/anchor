import { deletePlatformInvitationMutation } from "@/client/@tanstack/react-query.gen";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { type ReactElement, useState } from "react";
import { toast } from "sonner";
import { FormAlert } from "../common/FormAlert";
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
} from "../ui/alert-dialog";
import { Button } from "../ui/button";
import { Spinner } from "../ui/spinner";

type PlatformDeleteInvitationDialogProps = {
	invitationId: string;
	onDeleted?: () => void;
	/**
	 * Rendered *as* the trigger, so it has to be a single element that forwards
	 * DOM props — a `Button`, not a `Tooltip` or another composite. Base UI hands
	 * the trigger's own props to whatever it renders, and a component that
	 * ignores them silently drops the open behaviour.
	 */
	trigger?: ReactElement;
};

export function PlatformDeleteInvitationDialog({
	invitationId,
	onDeleted,
	trigger,
}: PlatformDeleteInvitationDialogProps) {
	if (!invitationId) {
		throw new Error("invitationId is required");
	}
	const [open, setOpen] = useState(false);

	const { mutate, isPending, error } = useMutation({
		...deletePlatformInvitationMutation(),
		onSuccess: () => {
			toast.success("Invitation deleted successfully!");
			setOpen(false);
			onDeleted?.();
		},
	});

	const defaultTrigger = (
		<Button size="icon" variant="outlineDestructive">
			<span className="sr-only">Delete invitation</span>
			<Trash2 />
		</Button>
	);

	return (
		<AlertDialog open={open} onOpenChange={setOpen}>
			<AlertDialogTrigger render={trigger ?? defaultTrigger} />
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete Invitation?</AlertDialogTitle>
					<AlertDialogDescription>
						This action cannot be undone. This will permanently delete this
						invitation.
					</AlertDialogDescription>
				</AlertDialogHeader>

				<FormAlert
					message={
						error
							? getApiErrorMessage(error) || "Failed to delete invitation."
							: null
					}
				/>

				<AlertDialogFooter>
					<AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
					<AlertDialogAction
						variant="destructive"
						disabled={isPending}
						onClick={() => mutate({ path: { invitation_id: invitationId } })}
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
