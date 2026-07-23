import type { LicenseResponse } from "@/client";
import {
	getOrganizationLicenseQueryKey,
	listLicensesQueryKey,
	reinstateOrganizationLicenseMutation,
	revokeOrganizationLicenseMutation,
	suspendOrganizationLicenseMutation,
} from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ReactElement } from "react";
import { useState } from "react";
import { toast } from "sonner";

export type LicenseStatusAction = "suspend" | "reinstate" | "revoke";

const actionCopy: Record<
	LicenseStatusAction,
	{
		title: string;
		verb: string;
		description: string;
		successMessage: string;
		destructive: boolean;
	}
> = {
	suspend: {
		title: "Suspend License",
		verb: "Suspend",
		description:
			"Tokens keep being issued with status SUSPENDED so the consumer can hard-block with clear UX. The license can be reinstated at any time.",
		successMessage: "License suspended.",
		destructive: false,
	},
	reinstate: {
		title: "Reinstate License",
		verb: "Reinstate",
		description:
			"Sets the license back to ACTIVE. The organization receives normal license tokens at its next refresh.",
		successMessage: "License reinstated.",
		destructive: false,
	},
	revoke: {
		title: "Revoke License",
		verb: "Revoke",
		description:
			"Revoked organizations stop receiving license tokens at their next refresh (worst case one token TTL). Revocation can be undone by reinstating the license.",
		successMessage: "License revoked.",
		destructive: true,
	},
};

interface LicenseStatusActionDialogProps {
	productId: string;
	license: LicenseResponse;
	action: LicenseStatusAction;
	organizationName?: string;
	trigger: ReactElement;
	onDone?: () => void;
}

export function LicenseStatusActionDialog({
	productId,
	license,
	action,
	organizationName,
	trigger,
	onDone,
}: LicenseStatusActionDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const copy = actionCopy[action];

	const handleSuccess = () => {
		toast.success(copy.successMessage);
		setOpen(false);
		queryClient.invalidateQueries({
			queryKey: listLicensesQueryKey({ path: { product_id: productId } }),
		});
		queryClient.invalidateQueries({
			queryKey: getOrganizationLicenseQueryKey({
				path: {
					product_id: productId,
					organization_id: license.organization_id,
				},
			}),
		});
		onDone?.();
	};

	const handleError = (error: unknown) => {
		console.error(`Failed to ${action} license:`, error);
		toast.error(
			getApiErrorMessage(error) ??
				`Failed to ${action} the license. Please try again.`,
		);
	};

	const suspendMutation = useMutation({
		...suspendOrganizationLicenseMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});
	const reinstateMutation = useMutation({
		...reinstateOrganizationLicenseMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});
	const revokeMutation = useMutation({
		...revokeOrganizationLicenseMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});

	const mutation =
		action === "suspend"
			? suspendMutation
			: action === "reinstate"
				? reinstateMutation
				: revokeMutation;

	const handleConfirm = () => {
		mutation.mutate({
			path: {
				product_id: productId,
				organization_id: license.organization_id,
			},
		});
	};

	const orgLabel = organizationName ?? license.organization_id;

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent>
				<DialogHeader>
					<DialogTitle>{copy.title}</DialogTitle>
					<DialogDescription>
						{copy.verb} the license of "{orgLabel}"? {copy.description}
					</DialogDescription>
				</DialogHeader>

				{mutation.error ? (
					<FormAlert
						variant="default"
						message={
							getApiErrorMessage(mutation.error) ??
							`Failed to ${action} the license`
						}
					/>
				) : null}

				<DialogFooter>
					<Button
						variant="outline"
						onClick={() => setOpen(false)}
						disabled={mutation.isPending}
					>
						Cancel
					</Button>
					<Button
						variant={copy.destructive ? "destructive" : "default"}
						onClick={handleConfirm}
						disabled={mutation.isPending}
					>
						{mutation.isPending ? (
							<>
								<Spinner className="mr-2 text-current" />
								{copy.verb}...
							</>
						) : (
							copy.verb
						)}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	);
}
