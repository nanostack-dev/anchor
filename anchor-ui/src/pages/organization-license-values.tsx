import type { LicenseTemplateValues } from "@/client";
import {
	adjustOrganizationLicenseMutation,
	getLicenseSchemaOptions,
	listLicenseTemplatesOptions,
} from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
import { LicenseValueFields } from "@/components/license/LicenseValueFields";
import {
	formatFieldValue,
	isFieldValueSet,
} from "@/components/license/license-field-format";
import { useOrganizationLicenseQuery } from "@/components/license/use-organization-license";
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { useProduct } from "@/context/product/ProductContext";
import { getApiErrorMessage, getApiFieldErrors } from "@/lib/api-error";
import { organizationLicenseDetailRoute } from "@/routes/organizations/organization-license.$organizationId";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useBlocker } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

const invalidatedByAdjustment = new Set([
	"getOrganizationLicense",
	"getOrganizationLicenseHistory",
	"searchOrganizationLicenses",
]);

function sameValue(a: unknown, b: unknown): boolean {
	if (typeof a === "number" || typeof b === "number") {
		return Number(a) === Number(b);
	}
	return a === b;
}

/**
 * Every license field this product declares, and what this organization holds
 * for it — editable in place.
 *
 * Adjusting is a merge: only the fields actually changed are sent, and one
 * left alone keeps its value. So the form submits its own diff rather than
 * the whole set, which is also what keeps an adjustment's history entry down
 * to the fields an operator meant to touch.
 */
export default function OrganizationLicenseValuesPage() {
	const { organizationId } = organizationLicenseDetailRoute.useParams();
	const { currentProduct } = useProduct();
	const productId = currentProduct?.id;
	const queryClient = useQueryClient();

	const [draft, setDraft] = useState<LicenseTemplateValues>({});
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

	const licenseQuery = useOrganizationLicenseQuery(productId, organizationId);
	const schemaQuery = useQuery({
		...getLicenseSchemaOptions({ path: { product_id: productId as string } }),
		enabled: !!productId,
		retry: false,
	});
	const templatesQuery = useQuery({
		...listLicenseTemplatesOptions({
			path: { product_id: productId as string },
		}),
		enabled: !!productId,
	});

	const adjust = useMutation({
		...adjustOrganizationLicenseMutation(),
		onSuccess: () => {
			setDraft({});
			setFieldErrors({});
			// The header badge reads the search summary and the Changes tab reads
			// the history this write appends, so neither refetches on its own.
			// Matched by predicate rather than a reconstructed key: each of these
			// queries carries its own arguments, and every shape must invalidate.
			queryClient.invalidateQueries({
				predicate: (query) =>
					invalidatedByAdjustment.has(
						(query.queryKey[0] as { _id?: string } | undefined)?._id ?? "",
					),
			});
			toast.success("License adjusted");
		},
		onError: (error) => setFieldErrors(getApiFieldErrors(error)),
	});

	const license = licenseQuery.data;
	const dirty = Object.keys(draft).length > 0;

	// The draft lives in this component, so switching tab, following the back
	// link, or reloading would drop an edit without saying so. Covers the
	// browser's own reload and close through the same hook.
	const blocker = useBlocker({
		shouldBlockFn: () => dirty,
		enableBeforeUnload: () => dirty,
		withResolver: true,
	});

	if (!license || !productId) return null;

	const fields = schemaQuery.data?.fields ?? [];
	const templateValues = templatesQuery.data?.items?.find(
		(item) => item.id === license.template_id,
	)?.values;

	const values = { ...license.values, ...draft };

	const changed = fields.filter(
		(field) =>
			field.name in draft &&
			!sameValue(draft[field.name], license.values[field.name]),
	);

	const notes = Object.fromEntries(
		fields
			.filter(
				(field) =>
					templateValues !== undefined &&
					field.name in templateValues &&
					!sameValue(license.values[field.name], templateValues[field.name]),
			)
			.map((field) => [
				field.name,
				`Tier grants ${formatFieldValue(field.type, templateValues?.[field.name])}`,
			]),
	);

	const setValue = (name: string, value: unknown) => {
		setDraft((prev) => ({ ...prev, [name]: value }));
		setFieldErrors((prev) => {
			if (!(name in prev)) return prev;
			const { [name]: _removed, ...rest } = prev;
			return rest;
		});
	};

	const save = () => {
		const errors: Record<string, string> = {};
		for (const field of changed) {
			if (!isFieldValueSet(field.type, draft[field.name])) {
				// Merge semantics have no way to express an empty field: the schema
				// declares it, so it stays set. Clearing an input is a mistake here,
				// not an instruction.
				errors[field.name] = "This field must keep a value.";
			}
		}
		setFieldErrors(errors);
		if (Object.keys(errors).length > 0) return;

		adjust.mutate({
			path: { product_id: productId, organization_id: organizationId },
			body: {
				values: Object.fromEntries(
					changed.map((field) => [field.name, draft[field.name]]),
				),
			},
		});
	};

	// "Declares no fields" is an answer, and an unanswered query is not it.
	if (schemaQuery.isLoading) {
		return (
			<div className="flex flex-col gap-2">
				<Skeleton className="h-9 w-full" />
				<Skeleton className="h-9 w-full" />
				<Skeleton className="h-9 w-full" />
			</div>
		);
	}

	if (fields.length === 0) {
		return (
			<p className="text-sm text-muted-foreground">
				This product&rsquo;s license schema declares no fields, so there is
				nothing to adjust.
			</p>
		);
	}

	return (
		<div className="flex flex-col gap-3">
			<p className="text-xs text-muted-foreground">
				Every field this product declares, and what this organization holds for
				it. Change one to adjust this customer alone — the tier is untouched,
				and so is which tier the license says it came from.
			</p>

			<LicenseValueFields
				fields={fields}
				values={values}
				onChange={setValue}
				errors={fieldErrors}
				disabled={adjust.isPending}
				notes={notes}
			/>

			<FormAlert message={getApiErrorMessage(adjust.error)} />

			{changed.length > 0 && (
				<div className="sticky bottom-4 flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-background p-3 shadow-lg">
					<p className="text-sm">
						{changed.length} field{changed.length === 1 ? "" : "s"} changed
						<span className="ml-2 font-mono text-xs text-muted-foreground">
							{changed.map((field) => field.name).join(", ")}
						</span>
					</p>
					<div className="flex items-center gap-2">
						<Button
							variant="ghost"
							size="sm"
							onClick={() => {
								setDraft({});
								setFieldErrors({});
							}}
							disabled={adjust.isPending}
						>
							Discard
						</Button>
						<Button size="sm" onClick={save} disabled={adjust.isPending}>
							{adjust.isPending && <Spinner />}
							Adjust this customer
						</Button>
					</div>
				</div>
			)}

			<AlertDialog
				open={blocker.status === "blocked"}
				onOpenChange={(next) => {
					if (!next) blocker.reset?.();
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>Leave without adjusting?</AlertDialogTitle>
						<AlertDialogDescription>
							{changed.length} field{changed.length === 1 ? "" : "s"} on this
							customer&rsquo;s license {changed.length === 1 ? "has" : "have"}{" "}
							been changed and not saved. Leaving discards the change.
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel onClick={() => blocker.reset?.()}>
							Stay
						</AlertDialogCancel>
						<AlertDialogAction onClick={() => blocker.proceed?.()}>
							Discard and leave
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	);
}
