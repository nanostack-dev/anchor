import type { ApiErrorResponse, PlanResponse } from "@/client";
import {
	deletePlanMutation,
	listPlansQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { DeleteDialog } from "@/components/common/dialogs/DeleteDialog";
import { getApiError } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ReactElement } from "react";

interface DeletePlanDialogProps {
	productId: string;
	plan: PlanResponse;
	trigger?: ReactElement;
	onDeleted?: () => void;
}

export function DeletePlanDialog({
	productId,
	plan,
	trigger,
	onDeleted,
}: DeletePlanDialogProps) {
	const queryClient = useQueryClient();
	const entitlementCount = Object.keys(plan.entitlements).length;

	const deleteMutation = useMutation({
		...deletePlanMutation(),
	});

	const handleDelete = async () => {
		try {
			await deleteMutation.mutateAsync({
				path: { product_id: productId, plan_id: plan.id },
			});
		} catch (error) {
			const apiError = getApiError(error);
			if (apiError?.code === "PLAN_IN_USE") {
				const licenseCount = Number(apiError.metadata?.license_count ?? 0);
				throw {
					errors: [
						{
							code: apiError.code,
							message: `This plan is still assigned to ${licenseCount} license${licenseCount === 1 ? "" : "s"}. Move those organizations to another plan (or revoke and reassign their licenses) before deleting it.`,
						},
					],
				} satisfies ApiErrorResponse;
			}
			throw error;
		}
		queryClient.invalidateQueries({
			queryKey: listPlansQueryKey({ path: { product_id: productId } }),
		});
	};

	const warningMessage = plan.is_default
		? "This is the product's default plan: organizations without a license fall back to it. Deletion fails while any license still references this plan."
		: "Deletion fails while any license still references this plan.";

	return (
		<DeleteDialog
			trigger={trigger}
			entityType="Plan"
			entityName={plan.name}
			displayFields={[
				{
					label: "Key",
					value: <span className="font-mono text-sm">{plan.key}</span>,
				},
				{ label: "Name", value: plan.name },
				{
					label: "Default",
					value: <StatusBadge tone="info">Default plan</StatusBadge>,
					condition: plan.is_default,
				},
				{
					label: "Entitlements",
					value: (
						<span className="text-sm text-muted-foreground">
							{entitlementCount} entitlement{entitlementCount === 1 ? "" : "s"}
						</span>
					),
				},
			]}
			warningMessage={warningMessage}
			onDelete={handleDelete}
			onDeleted={onDeleted}
		/>
	);
}
