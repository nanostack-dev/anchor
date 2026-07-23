import {
	type PlanRequest,
	type PlanResponse,
	type PlanUpdateRequest,
	zPlanRequest,
	zPlanUpdateRequest,
} from "@/client";
import {
	createPlanMutation,
	listPlansQueryKey,
	updatePlanMutation,
} from "@/client/@tanstack/react-query.gen";
import {
	type EntitlementRow,
	EntitlementsEditor,
	entitlementsToRows,
	rowsToEntitlements,
	validateEntitlementRows,
} from "@/components/product/licensing/EntitlementsEditor";
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Spinner } from "@/components/ui/spinner";
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ReactElement, useEffect, useState } from "react";
import { toast } from "sonner";
import { z } from "zod";

const createFormSchema = zPlanRequest.superRefine((val, ctx) => {
	if (!val.key?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Key is required",
			path: ["key"],
		});
	} else if (!/^[a-z0-9][a-z0-9_.-]*$/.test(val.key.trim())) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message:
				"Key must start with a lowercase letter or digit and use only lowercase letters, digits, dots, dashes and underscores",
			path: ["key"],
		});
	}
	if (!val.name?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Name is required",
			path: ["name"],
		});
	}
});

const updateFormSchema = zPlanUpdateRequest.superRefine((val, ctx) => {
	if (!val.name?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Name is required",
			path: ["name"],
		});
	}
});

interface PlanDialogProps {
	productId: string;
	trigger: ReactElement;
	mode?: "create" | "edit";
	existingPlan?: PlanResponse;
	onSaved?: () => void;
}

interface PlanFormState {
	key: string;
	name: string;
	description: string;
	isDefault: boolean;
	entitlementRows: EntitlementRow[];
}

const emptyForm = (): PlanFormState => ({
	key: "",
	name: "",
	description: "",
	isDefault: false,
	entitlementRows: [],
});

export function PlanDialog({
	productId,
	trigger,
	mode = "create",
	existingPlan,
	onSaved,
}: PlanDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [formData, setFormData] = useState<PlanFormState>(emptyForm);
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

	const isEditMode = mode === "edit";

	useEffect(() => {
		if (!open) return;
		if (isEditMode && existingPlan) {
			setFormData({
				key: existingPlan.key,
				name: existingPlan.name,
				description: existingPlan.description ?? "",
				isDefault: existingPlan.is_default,
				entitlementRows: entitlementsToRows(existingPlan.entitlements),
			});
		} else {
			setFormData(emptyForm());
		}
		setFieldErrors({});
	}, [open, isEditMode, existingPlan]);

	const handleSuccess = () => {
		setOpen(false);
		queryClient.invalidateQueries({
			queryKey: listPlansQueryKey({ path: { product_id: productId } }),
		});
		onSaved?.();
	};

	const handleError = (error: unknown) => {
		console.error(`Failed to ${isEditMode ? "update" : "create"} plan:`, error);
		toast.error(
			getApiErrorMessage(error) ??
				`Failed to ${isEditMode ? "update" : "create"} plan. Please try again.`,
		);
	};

	const createMutation = useMutation({
		...createPlanMutation(),
		onSuccess: () => {
			toast.success("Plan created successfully!", {
				description: `${formData.name} is ready to be assigned`,
			});
			handleSuccess();
		},
		onError: handleError,
	});

	const updateMutation = useMutation({
		...updatePlanMutation(),
		onSuccess: () => {
			toast.success("Plan updated successfully!", {
				description: `${formData.name} has been updated`,
			});
			handleSuccess();
		},
		onError: handleError,
	});

	const isLoading = createMutation.isPending || updateMutation.isPending;

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();

		const errors: Record<string, string> = {};
		const entitlementError = validateEntitlementRows(formData.entitlementRows);
		if (entitlementError) {
			errors.entitlements = entitlementError;
		}

		const entitlements = rowsToEntitlements(formData.entitlementRows);

		if (isEditMode && existingPlan) {
			const payload: PlanUpdateRequest = {
				name: formData.name.trim(),
				description: formData.description.trim() || null,
				entitlements,
				is_default: formData.isDefault,
			};
			const result = updateFormSchema.safeParse(payload);
			if (!result.success) {
				for (const issue of result.error.issues) {
					const field = String(issue.path[0] ?? "form");
					errors[field] ??= issue.message;
				}
			}
			setFieldErrors(errors);
			if (Object.keys(errors).length > 0) return;

			updateMutation.mutate({
				path: { product_id: productId, plan_id: existingPlan.id },
				body: payload,
			});
		} else {
			const payload: PlanRequest = {
				key: formData.key.trim(),
				name: formData.name.trim(),
				description: formData.description.trim() || null,
				entitlements,
				is_default: formData.isDefault,
			};
			const result = createFormSchema.safeParse(payload);
			if (!result.success) {
				for (const issue of result.error.issues) {
					const field = String(issue.path[0] ?? "form");
					errors[field] ??= issue.message;
				}
			}
			setFieldErrors(errors);
			if (Object.keys(errors).length > 0) return;

			createMutation.mutate({
				path: { product_id: productId },
				body: payload,
			});
		}
	};

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent className="sm:max-w-[640px] max-h-[90vh] overflow-y-auto">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>
							{isEditMode ? "Edit Plan" : "Create Plan"}
						</DialogTitle>
						<DialogDescription>
							{isEditMode
								? "Update the plan's name, description, default flag and entitlements. The key is immutable."
								: "Define a plan with its stable key and the entitlements it grants."}
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<div className="grid gap-2">
							<Label htmlFor="plan-key">Key</Label>
							<Input
								id="plan-key"
								value={formData.key}
								onChange={(e) =>
									setFormData((prev) => ({ ...prev, key: e.target.value }))
								}
								placeholder="e.g. pro"
								className="font-mono"
								disabled={isEditMode}
								readOnly={isEditMode}
							/>
							<p className="text-xs text-muted-foreground">
								Stable identifier, unique per product (future Stripe
								lookup_key).{" "}
								{isEditMode
									? "Immutable after creation."
									: "Cannot be changed later."}
							</p>
							{fieldErrors.key && (
								<p className="text-sm text-destructive">{fieldErrors.key}</p>
							)}
						</div>
						<div className="grid gap-2">
							<Label htmlFor="plan-name">Name</Label>
							<Input
								id="plan-name"
								value={formData.name}
								onChange={(e) =>
									setFormData((prev) => ({ ...prev, name: e.target.value }))
								}
								placeholder="e.g. Pro"
							/>
							{fieldErrors.name && (
								<p className="text-sm text-destructive">{fieldErrors.name}</p>
							)}
						</div>
						<div className="grid gap-2">
							<Label htmlFor="plan-description">Description</Label>
							<Textarea
								id="plan-description"
								value={formData.description}
								onChange={(e) =>
									setFormData((prev) => ({
										...prev,
										description: e.target.value,
									}))
								}
								placeholder="What this plan offers (optional)"
							/>
							{fieldErrors.description && (
								<p className="text-sm text-destructive">
									{fieldErrors.description}
								</p>
							)}
						</div>
						<div className="flex items-center justify-between gap-4">
							<div className="space-y-1">
								<Label htmlFor="plan-default">Default plan</Label>
								<p className="text-xs text-muted-foreground">
									Organizations without a license fall back to this plan. At
									most one default plan per product.
								</p>
							</div>
							<Switch
								id="plan-default"
								checked={formData.isDefault}
								onCheckedChange={(checked) =>
									setFormData((prev) => ({ ...prev, isDefault: checked }))
								}
							/>
						</div>
						<div className="grid gap-2">
							<Label>Entitlements</Label>
							<EntitlementsEditor
								rows={formData.entitlementRows}
								onChange={(rows) =>
									setFormData((prev) => ({ ...prev, entitlementRows: rows }))
								}
								disabled={isLoading}
								emptyHint="No entitlements yet. Boolean entitlements gate features; numeric entitlements carry limits."
							/>
							{fieldErrors.entitlements && (
								<p className="text-sm text-destructive">
									{fieldErrors.entitlements}
								</p>
							)}
						</div>
					</div>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
							disabled={isLoading}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={isLoading}>
							{isLoading ? (
								<>
									<Spinner className="mr-2 text-current" />
									{isEditMode ? "Saving..." : "Creating..."}
								</>
							) : isEditMode ? (
								"Save Changes"
							) : (
								"Create Plan"
							)}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
