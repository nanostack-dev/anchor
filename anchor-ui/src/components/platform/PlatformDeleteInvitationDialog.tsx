import { deletePlatformInvitationMutation } from "@/client/@tanstack/react-query.gen";
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
import { type ReactElement, useState } from "react";
import { FormAlert } from "../common/FormAlert";

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
			toast.add({ type: "success", title: "Invitation deleted successfully!" });
			setOpen(false);
			onDeleted?.();
		},
	});

	const defaultTrigger = (
		<IconButton
			tone="critical"
			size="md"
			variant="outline"
			icon={Trash2}
			label="Delete invitation"
		/>
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
						variant="solid"
						tone="critical"
						disabled={isPending}
						onClick={() => mutate({ path: { invitation_id: invitationId } })}
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
