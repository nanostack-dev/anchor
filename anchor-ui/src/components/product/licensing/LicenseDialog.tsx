import {
	type LicenseAssignRequest,
	type LicenseResponse,
	LicenseStatus,
	type ProductOrganizationResponse,
	zLicenseAssignRequest,
} from "@/client";
import {
	getOrganizationLicenseQueryKey,
	listLicensesQueryKey,
	listPlansOptions,
	putOrganizationLicenseMutation,
} from "@/client/@tanstack/react-query.gen";
import {
	type EntitlementRow,
	EntitlementsEditor,
	entitlementsToRows,
	rowsToEntitlements,
	validateEntitlementRows,
} from "@/components/product/licensing/EntitlementsEditor";
import { OrganizationCombobox } from "@/components/product/licensing/OrganizationCombobox";
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
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Spinner } from "@/components/ui/spinner";
import { getApiErrorMessage } from "@/lib/api-error";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import dayjs from "dayjs";
import { type ReactElement, useEffect, useState } from "react";
import { toast } from "sonner";
import { z } from "zod";

const formSchema = zLicenseAssignRequest.superRefine((val, ctx) => {
	if (!val.plan_id?.trim()) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Plan is required",
			path: ["plan_id"],
		});
	}
	if (val.grace_until && !val.expires_at) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "A grace period requires an expiry date",
			path: ["grace_until"],
		});
	}
	if (
		val.grace_until &&
		val.expires_at &&
		dayjs(val.grace_until).isBefore(dayjs(val.expires_at))
	) {
		ctx.addIssue({
			code: z.ZodIssueCode.custom,
			message: "Grace must not end before the expiry date",
			path: ["grace_until"],
		});
	}
});

const statusItems = [
	{ value: LicenseStatus.ACTIVE, label: "Active" },
	{ value: LicenseStatus.SUSPENDED, label: "Suspended" },
	{ value: LicenseStatus.REVOKED, label: "Revoked" },
];

const isoToLocalInput = (iso: string | null | undefined): string =>
	iso ? dayjs(iso).format("YYYY-MM-DDTHH:mm") : "";

const localInputToIso = (value: string): string | null =>
	value ? new Date(value).toISOString() : null;

interface LicenseDialogProps {
	productId: string;
	trigger: ReactElement;
	/** When set, the dialog edits (fully replaces) this license. */
	existingLicense?: LicenseResponse;
	/** Display name of the license's organization in edit mode. */
	organizationName?: string;
	onSaved?: () => void;
}

interface LicenseFormState {
	organization: ProductOrganizationResponse | null;
	planId: string;
	status: LicenseStatus;
	expiresAt: string;
	graceUntil: string;
	tokenTtlSeconds: string;
	overrideRows: EntitlementRow[];
}

const emptyForm = (): LicenseFormState => ({
	organization: null,
	planId: "",
	status: LicenseStatus.ACTIVE,
	expiresAt: "",
	graceUntil: "",
	tokenTtlSeconds: "86400",
	overrideRows: [],
});

/**
 * Assigns a license to an organization or edits an existing one. The API is
 * a full-replacement PUT: in edit mode every field is pre-loaded from the
 * current license so saving never silently clears values.
 */
export function LicenseDialog({
	productId,
	trigger,
	existingLicense,
	organizationName,
	onSaved,
}: LicenseDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const [formData, setFormData] = useState<LicenseFormState>(emptyForm);
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

	const isEditMode = !!existingLicense;

	const { data: planData } = useQuery({
		...listPlansOptions({ path: { product_id: productId } }),
		enabled: open,
	});
	const plans = planData?.items ?? [];

	useEffect(() => {
		if (!open) return;
		if (existingLicense) {
			setFormData({
				organization: null,
				planId: existingLicense.plan_id,
				status: existingLicense.status,
				expiresAt: isoToLocalInput(existingLicense.expires_at),
				graceUntil: isoToLocalInput(existingLicense.grace_until),
				tokenTtlSeconds: String(existingLicense.token_ttl_seconds),
				overrideRows: entitlementsToRows(existingLicense.entitlement_overrides),
			});
		} else {
			setFormData(emptyForm());
		}
		setFieldErrors({});
	}, [open, existingLicense]);

	const putMutation = useMutation({
		...putOrganizationLicenseMutation(),
		onSuccess: (_, variables) => {
			toast.success(
				isEditMode
					? "License updated successfully!"
					: "License assigned successfully!",
			);
			setOpen(false);
			queryClient.invalidateQueries({
				queryKey: listLicensesQueryKey({ path: { product_id: productId } }),
			});
			queryClient.invalidateQueries({
				queryKey: getOrganizationLicenseQueryKey({
					path: {
						product_id: productId,
						organization_id: variables.path.organization_id,
					},
				}),
			});
			onSaved?.();
		},
		onError: (error) => {
			console.error("Failed to save license:", error);
			toast.error(
				getApiErrorMessage(error) ??
					"Failed to save the license. Please try again.",
			);
		},
	});

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();

		const errors: Record<string, string> = {};

		const organizationId = isEditMode
			? existingLicense.organization_id
			: formData.organization?.id;
		if (!organizationId) {
			errors.organization = "Organization is required";
		}

		const overrideError = validateEntitlementRows(formData.overrideRows);
		if (overrideError) {
			errors.entitlement_overrides = overrideError;
		}

		const tokenTtl = Number(formData.tokenTtlSeconds);
		const payload: LicenseAssignRequest = {
			plan_id: formData.planId,
			status: formData.status,
			expires_at: localInputToIso(formData.expiresAt),
			grace_until: localInputToIso(formData.graceUntil),
			entitlement_overrides: rowsToEntitlements(formData.overrideRows),
			token_ttl_seconds: Number.isNaN(tokenTtl) ? undefined : tokenTtl,
		};

		const result = formSchema.safeParse(payload);
		if (!result.success) {
			for (const issue of result.error.issues) {
				const field = String(issue.path[0] ?? "form");
				errors[field] ??= issue.message;
			}
		}

		setFieldErrors(errors);
		if (Object.keys(errors).length > 0 || !organizationId) return;

		putMutation.mutate({
			path: { product_id: productId, organization_id: organizationId },
			body: payload,
		});
	};

	const planItems = plans.map((plan) => ({
		value: plan.id,
		label: `${plan.name} (${plan.key})`,
	}));

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger render={trigger} />
			<DialogContent className="sm:max-w-[640px] max-h-[90vh] overflow-y-auto">
				<form onSubmit={handleSubmit}>
					<DialogHeader>
						<DialogTitle>
							{isEditMode ? "Edit License" : "Assign License"}
						</DialogTitle>
						<DialogDescription>
							{isEditMode
								? "Saving fully replaces the license: every field below is applied as-is."
								: "Assign a plan to an organization, optionally with expiry, grace and per-organization overrides."}
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4 py-4">
						<div className="grid gap-2">
							<Label>Organization</Label>
							{isEditMode ? (
								<Input
									value={organizationName ?? existingLicense.organization_id}
									disabled
									readOnly
								/>
							) : (
								<OrganizationCombobox
									productId={productId}
									value={formData.organization}
									onChange={(organization) =>
										setFormData((prev) => ({ ...prev, organization }))
									}
									disabled={putMutation.isPending}
								/>
							)}
							{fieldErrors.organization && (
								<p className="text-sm text-destructive">
									{fieldErrors.organization}
								</p>
							)}
						</div>
						<div className="grid gap-2">
							<Label>Plan</Label>
							<Select
								items={planItems}
								value={formData.planId || null}
								onValueChange={(value) =>
									setFormData((prev) => ({
										...prev,
										planId: (value as string) ?? "",
									}))
								}
							>
								<SelectTrigger className="w-full" aria-label="Plan">
									<SelectValue>
										{formData.planId
											? (planItems.find(
													(item) => item.value === formData.planId,
												)?.label ?? formData.planId)
											: "Select a plan"}
									</SelectValue>
								</SelectTrigger>
								<SelectContent>
									{planItems.map((item) => (
										<SelectItem key={item.value} value={item.value}>
											{item.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							{plans.length === 0 && (
								<p className="text-xs text-muted-foreground">
									No plans yet — create a plan first.
								</p>
							)}
							{fieldErrors.plan_id && (
								<p className="text-sm text-destructive">
									{fieldErrors.plan_id}
								</p>
							)}
						</div>
						<div className="grid gap-2">
							<Label>Status</Label>
							<Select
								items={statusItems}
								value={formData.status}
								onValueChange={(value) =>
									setFormData((prev) => ({
										...prev,
										status: value as LicenseStatus,
									}))
								}
							>
								<SelectTrigger className="w-full" aria-label="Status">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{statusItems.map((item) => (
										<SelectItem key={item.value} value={item.value}>
											{item.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
							{fieldErrors.status && (
								<p className="text-sm text-destructive">{fieldErrors.status}</p>
							)}
						</div>
						<div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
							<div className="grid gap-2">
								<Label htmlFor="license-expires-at">Expires at</Label>
								<Input
									id="license-expires-at"
									type="datetime-local"
									value={formData.expiresAt}
									onChange={(e) =>
										setFormData((prev) => ({
											...prev,
											expiresAt: e.target.value,
										}))
									}
								/>
								<p className="text-xs text-muted-foreground">
									Leave empty for no expiry.
								</p>
								{fieldErrors.expires_at && (
									<p className="text-sm text-destructive">
										{fieldErrors.expires_at}
									</p>
								)}
							</div>
							<div className="grid gap-2">
								<Label htmlFor="license-grace-until">Grace until</Label>
								<Input
									id="license-grace-until"
									type="datetime-local"
									value={formData.graceUntil}
									onChange={(e) =>
										setFormData((prev) => ({
											...prev,
											graceUntil: e.target.value,
										}))
									}
								/>
								<p className="text-xs text-muted-foreground">
									Past expiry but within grace, tokens carry status GRACE.
								</p>
								{fieldErrors.grace_until && (
									<p className="text-sm text-destructive">
										{fieldErrors.grace_until}
									</p>
								)}
							</div>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="license-token-ttl">Token TTL (seconds)</Label>
							<Input
								id="license-token-ttl"
								type="number"
								min={60}
								max={2592000}
								value={formData.tokenTtlSeconds}
								onChange={(e) =>
									setFormData((prev) => ({
										...prev,
										tokenTtlSeconds: e.target.value,
									}))
								}
							/>
							<p className="text-xs text-muted-foreground">
								Lifetime of issued license tokens (60s to 30 days). Shorter TTLs
								mean faster revocation.
							</p>
							{fieldErrors.token_ttl_seconds && (
								<p className="text-sm text-destructive">
									{fieldErrors.token_ttl_seconds}
								</p>
							)}
						</div>
						<div className="grid gap-2">
							<Label>Entitlement overrides</Label>
							<EntitlementsEditor
								rows={formData.overrideRows}
								onChange={(rows) =>
									setFormData((prev) => ({ ...prev, overrideRows: rows }))
								}
								disabled={putMutation.isPending}
								emptyHint="No overrides: the organization gets exactly the plan's entitlements. Overrides replace the plan value for the same key."
							/>
							{fieldErrors.entitlement_overrides && (
								<p className="text-sm text-destructive">
									{fieldErrors.entitlement_overrides}
								</p>
							)}
						</div>
					</div>
					<DialogFooter>
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
							disabled={putMutation.isPending}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={putMutation.isPending}>
							{putMutation.isPending ? (
								<>
									<Spinner className="mr-2 text-current" />
									Saving...
								</>
							) : isEditMode ? (
								"Save Changes"
							) : (
								"Assign License"
							)}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
