import type { LicenseSchemaResponse } from "@/client";
import {
	createLicenseSchemaMutation,
	getLicenseSchemaQueryKey,
	updateLicenseSchemaMutation,
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
import { getApiErrorMessage, getApiFieldErrors } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { toast } from "sonner";

import { LicenseSchemaEditor } from "./LicenseSchemaEditor";
import {
	type FieldRow,
	fieldRowsFromSchema,
	fieldRowsToDeclarations,
	newFieldRow,
	validateFieldRows,
} from "./license-schema-draft";

interface LicenseSchemaFormDialogProps {
	productId: string;
	trigger: ReactElement;
	mode?: "create" | "edit";
	existingSchema?: LicenseSchemaResponse;
	onSaved?: () => void;
}

/**
 * Declares or edits a product's license schema: the fields a license may
 * carry, each field's type, and its validation rules. There is no separate
 * "required" flag — every declared field is mandatory on every template and
 * license, so the schema itself is the only place that decision is made.
 *
 * The dialog owns the request and its errors; `LicenseSchemaEditor` owns how
 * the draft is authored.
 */
export function LicenseSchemaFormDialog({
	productId,
	trigger,
	mode = "create",
	existingSchema,
	onSaved,
}: LicenseSchemaFormDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const isEditMode = mode === "edit" && !!existingSchema;

	const [description, setDescription] = useState("");
	const [fields, setFields] = useState<FieldRow[]>([]);
	const [rowErrors, setRowErrors] = useState<Record<string, string>>({});
	const [listError, setListError] = useState<string | undefined>();
	const [generalError, setGeneralError] = useState<string | undefined>();
	// Text mode can hold source the parser cannot read. Submitting then would
	// send whatever last parsed, which is not what the operator is looking at.
	const [sourceInvalid, setSourceInvalid] = useState(false);

	const resetForm = () => {
		if (isEditMode && existingSchema) {
			setDescription(existingSchema.description ?? "");
			setFields(fieldRowsFromSchema(existingSchema));
		} else {
			setDescription("");
			setFields([newFieldRow()]);
		}
		setRowErrors({});
		setListError(undefined);
		setGeneralError(undefined);
		setSourceInvalid(false);
	};

	const handleOpenChange = (next: boolean) => {
		setOpen(next);
		if (next) resetForm();
	};

	const invalidate = () => {
		queryClient.invalidateQueries({
			queryKey: getLicenseSchemaQueryKey({ path: { product_id: productId } }),
		});
	};

	const handleError = (error: unknown, fallback: string) => {
		const apiFieldErrors = getApiFieldErrors(error);
		if (Object.keys(apiFieldErrors).length > 0) {
			// API errors key by license field name; map them onto the row that
			// currently carries that name so the offending row is highlighted.
			setRowErrors(
				Object.fromEntries(
					fields
						.filter((f) => apiFieldErrors[f.name.trim()])
						.map((f) => [f.uiKey, apiFieldErrors[f.name.trim()] as string]),
				),
			);
		}
		setGeneralError(getApiErrorMessage(error) ?? fallback);
	};

	const createMutation = useMutation({
		...createLicenseSchemaMutation(),
		onSuccess: () => {
			toast.success("License schema created");
			setOpen(false);
			invalidate();
			onSaved?.();
		},
		onError: (error) => handleError(error, "Failed to create license schema."),
	});

	const updateMutation = useMutation({
		...updateLicenseSchemaMutation(),
		onSuccess: () => {
			toast.success("License schema updated");
			setOpen(false);
			invalidate();
			onSaved?.();
		},
		onError: (error) => handleError(error, "Failed to update license schema."),
	});

	const isSubmitting = createMutation.isPending || updateMutation.isPending;

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		setGeneralError(undefined);

		if (fields.length === 0) {
			setListError("Add at least one field to declare a schema.");
			return;
		}
		setListError(undefined);

		const errors = validateFieldRows(fields);
		setRowErrors(errors);
		if (Object.keys(errors).length > 0) return;

		const body = {
			description: description.trim(),
			fields: fieldRowsToDeclarations(fields),
		};

		if (isEditMode && existingSchema) {
			updateMutation.mutate({ path: { product_id: productId }, body });
		} else {
			createMutation.mutate({ path: { product_id: productId }, body });
		}
	};

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger render={trigger} />
			<DialogContent className="flex max-h-[90vh] flex-col p-0 sm:max-w-[680px]">
				<form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
					<DialogHeader className="px-6 pt-6">
						<DialogTitle>
							{isEditMode ? "Edit License Schema" : "Create License Schema"}
						</DialogTitle>
						<DialogDescription>
							Every field declared here becomes mandatory on every template and
							organization license for this product.
						</DialogDescription>
					</DialogHeader>

					<div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
						<FormAlert message={generalError} />

						<LicenseSchemaEditor
							description={description}
							onDescriptionChange={setDescription}
							fields={fields}
							onFieldsChange={setFields}
							errors={rowErrors}
							onSourceInvalidChange={setSourceInvalid}
							disabled={isSubmitting}
						/>

						{listError && (
							<p className="text-sm text-destructive">{listError}</p>
						)}
					</div>

					<DialogFooter className="border-t border-border px-6 py-4">
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
							disabled={isSubmitting}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={isSubmitting || sourceInvalid}>
							{isSubmitting
								? isEditMode
									? "Saving…"
									: "Creating…"
								: isEditMode
									? "Save Schema"
									: "Create Schema"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
