import { createPlatformInvitationMutation } from "@/client/@tanstack/react-query.gen";
import { FormValidationError } from "@/components/common/FormValidationError";
import { generateInvitationLink } from "@/components/platform/invitationUtils";
import { getApiErrorMessage } from "@/lib/api-error";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@nanostackorg/design-system/components/dialog";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import { Spinner } from "@nanostackorg/design-system/components/spinner";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useForm } from "@tanstack/react-form";
import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { z } from "zod";

const invitationFormSchema = z.object({
	email: z
		.string()
		.min(1, "Email is required")
		.email("Please enter a valid email address")
		.trim(),
});

type InvitationFormData = z.infer<typeof invitationFormSchema>;

interface PlatformAddInvitationDialogProps {
	open: boolean;
	onOpenChange: (open: boolean) => void;
	onSuccess?: (invitation: { code: string; email: string }) => void;
}

export function PlatformAddInvitationDialog({
	open,
	onOpenChange,
	onSuccess,
}: PlatformAddInvitationDialogProps) {
	const [copied, setCopied] = useState(false);

	const form = useForm({
		defaultValues: {
			email: "",
		} as InvitationFormData,
		onSubmit: async ({ value }) => {
			const result = invitationFormSchema.safeParse(value);
			if (!result.success) {
				return;
			}
			await onSubmit(value);
		},
		validators: {
			onChange: invitationFormSchema,
			onSubmit: invitationFormSchema,
		},
	});

	const {
		mutate: createInvitation,
		isPending: isCreating,
		isSuccess,
		data,
		reset,
	} = useMutation({
		...createPlatformInvitationMutation(),
		onSuccess: (resp) => {
			toast.add({ type: "success", title: "Invitation created successfully!" });
			if (onSuccess) {
				onSuccess({ code: resp.code, email: form.state.values.email });
			}
		},
		onError: (error) => {
			console.error("Failed to create invitation:", error);
			const errorMessage = getApiErrorMessage(error);
			if (errorMessage) {
				toast.add({ type: "error", title: errorMessage });
			} else {
				toast.add({
					type: "error",
					title: "Failed to create invitation. Please try again.",
				});
			}
		},
	});

	const onSubmit = async (values: InvitationFormData) => {
		createInvitation({
			body: { email: values.email },
		});
	};

	const invitationUrl =
		isSuccess && data
			? generateInvitationLink(
					data.tenant_id,
					form.state.values.email,
					data.code,
				)
			: null;

	const handleCopy = () => {
		if (invitationUrl) {
			navigator.clipboard.writeText(invitationUrl);
			setCopied(true);
			setTimeout(() => setCopied(false), 1500);
		}
	};

	const handleOpenChange = (newOpen: boolean) => {
		onOpenChange(newOpen);
		if (!newOpen) {
			form.reset();
			setCopied(false);
			reset();
		}
	};

	const handleClose = () => {
		form.reset();
		setCopied(false);
		reset();
		onOpenChange(false);
	};

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>Invite User</DialogTitle>
					<DialogDescription>
						Send an invitation link to a user by entering their email address.
					</DialogDescription>
				</DialogHeader>
				{isSuccess && invitationUrl ? (
					<Box className="space-y-4">
						<Box className="space-y-2">
							<Label>Invitation link generated:</Label>
							<Box className="flex gap-2 items-center">
								<Input value={invitationUrl} readOnly onClick={handleCopy} />
								<Button type="button" variant="outline" onClick={handleCopy}>
									{copied ? "Copied!" : "Copy"}
								</Button>
							</Box>
						</Box>
						<DialogFooter>
							<Button type="button" variant="outline" onClick={handleClose}>
								Close
							</Button>
						</DialogFooter>
					</Box>
				) : (
					<form
						onSubmit={(e) => {
							e.preventDefault();
							e.stopPropagation();
							form.handleSubmit();
						}}
					>
						<Box className="space-y-6 py-4">
							<form.Field name="email">
								{(field) => (
									<Box className="space-y-2">
										<Label htmlFor="invite-email">Email</Label>
										<Input
											id="invite-email"
											type="email"
											placeholder="Enter email address"
											value={field.state.value}
											onChange={(e) => field.handleChange(e.target.value)}
											onBlur={field.handleBlur}
											disabled={isCreating}
										/>
										<FormValidationError field={field} />
									</Box>
								)}
							</form.Field>
						</Box>
						<DialogFooter>
							<Button
								type="button"
								variant="outline"
								onClick={handleClose}
								disabled={isCreating}
							>
								Cancel
							</Button>
							<form.Subscribe
								selector={(state) => [
									state.canSubmit,
									state.isSubmitting,
									state.isDirty,
									state.isValidating,
									state.isValid,
								]}
							>
								{([
									canSubmit,
									isSubmitting,
									isDirty,
									isValidating,
									isValid,
								]) => (
									<Button
										variant="solid"
										tone="brand"
										type="submit"
										disabled={
											!canSubmit ||
											isSubmitting ||
											!isValid ||
											isValidating ||
											!isDirty ||
											isCreating
										}
									>
										{isCreating || isSubmitting ? (
											<Box className="flex items-center gap-2">
												<Spinner />
												<span>Sending...</span>
											</Box>
										) : (
											<span>Send Invitation</span>
										)}
									</Button>
								)}
							</form.Subscribe>
						</DialogFooter>
					</form>
				)}
			</DialogContent>
		</Dialog>
	);
}
