import { deletePlatformInvitationMutation } from "@/client/@tanstack/react-query.gen";
import { useMutation } from "@tanstack/react-query";
import { Trash2 } from "lucide-react";
import { useState } from "react";
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

type PlatformDeleteInvitationDialogProps = {
	invitationId: string;
	onDeleted?: () => void;
	trigger?: React.ReactNode;
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

	const { mutate, isPending, isError, error } = useMutation({
		...deletePlatformInvitationMutation(),
		onSuccess: () => {
			setOpen(false);
			onDeleted?.();
		},
	});

	const onDeleteInvitation = async () => {
		mutate({ path: { invitation_id: invitationId } });
	};

	return (
		<AlertDialog open={open} onOpenChange={setOpen}>
			<AlertDialogTrigger asChild>
				{trigger ? (
					<button type="button" onClick={() => setOpen(true)}>
						{trigger}
					</button>
				) : (
					<Button size="icon" variant="outlineDestructive">
						<Trash2 />
					</Button>
				)}
			</AlertDialogTrigger>
			<AlertDialogContent>
				<AlertDialogHeader>
					<AlertDialogTitle>Delete Invitation?</AlertDialogTitle>
					<AlertDialogDescription>
						This action cannot be undone. This will permanently delete this
						invitation.
					</AlertDialogDescription>
				</AlertDialogHeader>
				{isError && (
					<div className="text-destructive text-sm mb-2">
						{error instanceof Error
							? error.message
							: "Failed to delete invitation."}
					</div>
				)}
				<AlertDialogFooter>
					<AlertDialogCancel disabled={isPending}>Cancel</AlertDialogCancel>
					<AlertDialogAction
						disabled={isPending}
						onClick={onDeleteInvitation}
						className="bg-destructive text-white hover:bg-destructive/90"
					>
						{isPending ? "Deleting..." : "Delete"}
					</AlertDialogAction>
				</AlertDialogFooter>
			</AlertDialogContent>
		</AlertDialog>
	);
}
