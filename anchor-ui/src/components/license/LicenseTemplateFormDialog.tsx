import type {
	LicenseSchemaResponse,
	LicenseTemplateResponse,
	LicenseTemplateValues,
} from "@/client";
import {
	createLicenseTemplateMutation,
	updateLicenseTemplateMutation,
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
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage, getApiFieldErrors } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ReactElement, useState } from "react";
import { toast } from "sonner";
import { LicenseValueFields } from "./LicenseValueFields";
import { isFieldValueSet } from "./license-field-format";

interface LicenseTemplateFormDialogProps {
	productId: string;
	schema: LicenseSchemaResponse;
	trigger: ReactElement;
	mode?: "create" | "edit";
	existingTemplate?: LicenseTemplateResponse;
	onSaved?: () => void;
}

/**
 * Creates or edits a license template — a named, reusable set of values
 * satisfying the product's license schema. Every declared field must be set;
 * this mirrors the schema in `service.ValidateValues` client-side for
 * immediate feedback, and still surfaces the server's own field-scoped errors
 * on submit (a compiled regex, a duplicate name racing another write, and so
 * on cannot be caught here).
 */
export function LicenseTemplateFormDialog({
	productId,
	schema,
	trigger,
	mode = "create",
	existingTemplate,
	onSaved,
}: LicenseTemplateFormDialogProps) {
	const queryClient = useQueryClient();
	const [open, setOpen] = useState(false);
	const isEditMode = mode === "edit" && !!existingTemplate;

	const [name, setName] = useState("");
	const [description, setDescription] = useState("");
	const [values, setValues] = useState<LicenseTemplateValues>({});
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
	const [generalError, setGeneralError] = useState<string | undefined>();

	const resetForm = () => {
		if (isEditMode && existingTemplate) {
			setName(existingTemplate.name);
			setDescription(existingTemplate.description ?? "");
			setValues({ ...existingTemplate.values });
		} else {
			setName("");
			setDescription("");
			setValues({});
		}
		setFieldErrors({});
		setGeneralError(undefined);
	};

	const handleOpenChange = (next: boolean) => {
		setOpen(next);
		if (next) resetForm();
	};

	// Predicate rather than a reconstructed key: the datatable's own list query
	// may or may not carry a `status` filter, and both shapes must invalidate.
	const invalidate = () => {
		queryClient.invalidateQueries({
			predicate: (query) =>
				(query.queryKey[0] as { _id?: string } | undefined)?._id ===
				"listLicenseTemplates",
		});
	};

	const handleError = (error: unknown, fallback: string) => {
		setFieldErrors(getApiFieldErrors(error));
		setGeneralError(getApiErrorMessage(error) ?? fallback);
	};

	const createMutation = useMutation({
		...createLicenseTemplateMutation(),
		onSuccess: () => {
			toast.success("License template created", { description: name });
			setOpen(false);
			invalidate();
			onSaved?.();
		},
		onError: (error) =>
			handleError(error, "Failed to create license template."),
	});

	const updateMutation = useMutation({
		...updateLicenseTemplateMutation(),
		onSuccess: () => {
			toast.success("License template updated", { description: name });
			setOpen(false);
			invalidate();
			onSaved?.();
		},
		onError: (error) =>
			handleError(error, "Failed to update license template."),
	});

	const isSubmitting = createMutation.isPending || updateMutation.isPending;

	const setValue = (fieldName: string, value: unknown) => {
		setValues((prev) => ({ ...prev, [fieldName]: value }));
	};

	function validate(): boolean {
		const errors: Record<string, string> = {};
		if (!name.trim()) {
			errors.name = "Name is required.";
		}
		for (const field of schema.fields) {
			if (!isFieldValueSet(field.type, values[field.name])) {
				errors[field.name] = "This field is required.";
			}
		}
		setFieldErrors(errors);
		return Object.keys(errors).length === 0;
	}

	const handleSubmit = (e: React.FormEvent) => {
		e.preventDefault();
		setGeneralError(undefined);
		if (!validate()) return;

		const submittedValues: LicenseTemplateValues = Object.fromEntries(
			schema.fields.map((field) => [field.name, values[field.name]]),
		);

		if (isEditMode && existingTemplate) {
			updateMutation.mutate({
				path: {
					product_id: productId,
					license_template_id: existingTemplate.id,
				},
				body: {
					name: name.trim(),
					description: description.trim(),
					values: submittedValues,
				},
			});
		} else {
			createMutation.mutate({
				path: { product_id: productId },
				body: {
					name: name.trim(),
					description: description.trim(),
					values: submittedValues,
				},
			});
		}
	};

	return (
		<Dialog open={open} onOpenChange={handleOpenChange}>
			<DialogTrigger render={trigger} />
			<DialogContent className="flex max-h-[90vh] flex-col p-0 sm:max-w-[600px]">
				<form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
					<DialogHeader className="px-6 pt-6">
						<DialogTitle>
							{isEditMode ? "Edit License Template" : "Create License Template"}
						</DialogTitle>
						<DialogDescription>
							A named set of values satisfying this product&rsquo;s license
							schema, ready to instantiate onto an organization.
						</DialogDescription>
					</DialogHeader>

					<div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-4">
						<FormAlert message={generalError} />

						<div className="space-y-1.5">
							<Label htmlFor="template-name">Name</Label>
							<Input
								id="template-name"
								value={name}
								onChange={(e) => setName(e.target.value)}
								placeholder="Pro"
								maxLength={120}
								disabled={isSubmitting}
							/>
							{fieldErrors.name && (
								<p className="text-sm text-destructive">{fieldErrors.name}</p>
							)}
						</div>

						<div className="space-y-1.5">
							<Label htmlFor="template-description">Description</Label>
							<Textarea
								id="template-description"
								value={description}
								onChange={(e) => setDescription(e.target.value)}
								placeholder="Optional"
								rows={2}
								disabled={isSubmitting}
							/>
						</div>

						<div className="space-y-1.5">
							<Label>Values</Label>
							<LicenseValueFields
								fields={schema.fields}
								values={values}
								onChange={setValue}
								errors={fieldErrors}
								disabled={isSubmitting}
							/>
						</div>
					</div>

					<DialogFooter className="mx-0 mb-0 border-t border-border px-6 py-4">
						<Button
							type="button"
							variant="outline"
							onClick={() => setOpen(false)}
							disabled={isSubmitting}
						>
							Cancel
						</Button>
						<Button type="submit" disabled={isSubmitting}>
							{isSubmitting
								? isEditMode
									? "Saving…"
									: "Creating…"
								: isEditMode
									? "Save Template"
									: "Create Template"}
						</Button>
					</DialogFooter>
				</form>
			</DialogContent>
		</Dialog>
	);
}
