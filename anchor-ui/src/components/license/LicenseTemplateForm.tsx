import {
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	type LicenseTemplateValues,
	zLicenseTemplateCreateRequest,
} from "@/client";
import {
	createLicenseTemplateMutation,
	getLicenseTemplateQueryKey,
	updateLicenseTemplateMutation,
} from "@/client/@tanstack/react-query.gen";
import { FormAlert } from "@/components/common/FormAlert";
import { Button } from "@/components/ui/button";
import {
	Field,
	FieldError,
	FieldGroup,
	FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { getApiErrorMessage, getApiFieldErrors } from "@/lib/api-error";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { toast } from "sonner";
import { LicenseValueFields } from "./LicenseValueFields";
import { isFieldValueSet } from "./license-field-format";

interface LicenseTemplateFormProps {
	productId: string;
	schema: LicenseSchemaResponse;
	template?: LicenseTemplateResponse;
	onCancel: () => void;
	onSaved: (template: LicenseTemplateResponse) => void;
}

export function LicenseTemplateForm({
	productId,
	schema,
	template,
	onCancel,
	onSaved,
}: LicenseTemplateFormProps) {
	const queryClient = useQueryClient();
	const [name, setName] = useState(template?.name ?? "");
	const [description, setDescription] = useState(template?.description ?? "");
	const [values, setValues] = useState<LicenseTemplateValues>(() => ({
		...template?.values,
	}));
	const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
	const [generalError, setGeneralError] = useState<string>();
	const handleSuccess = (saved: LicenseTemplateResponse) => {
		queryClient.setQueryData(
			getLicenseTemplateQueryKey({
				path: { product_id: productId, license_template_id: saved.id },
			}),
			saved,
		);
		void queryClient.invalidateQueries({
			predicate: (query) =>
				(query.queryKey[0] as { _id?: string } | undefined)?._id ===
				"listLicenseTemplates",
		});
		toast.success(
			template ? "License template updated" : "License template created",
			{ description: saved.name },
		);
		onSaved(saved);
	};
	const handleError = (error: unknown) => {
		setFieldErrors(getApiFieldErrors(error));
		setGeneralError(
			getApiErrorMessage(error) ?? "Failed to save license template.",
		);
	};
	const createMutation = useMutation({
		...createLicenseTemplateMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});
	const updateMutation = useMutation({
		...updateLicenseTemplateMutation(),
		onSuccess: handleSuccess,
		onError: handleError,
	});
	const isSubmitting = createMutation.isPending || updateMutation.isPending;

	const handleSubmit = (event: React.FormEvent) => {
		event.preventDefault();
		if (isSubmitting || template?.status === "ARCHIVED") return;
		setGeneralError(undefined);
		const validation = zLicenseTemplateCreateRequest
			.superRefine((body, context) => {
				if (!body.name.trim())
					context.addIssue({
						code: "custom",
						path: ["name"],
						message: "Name is required.",
					});
				for (const field of schema.fields) {
					if (!isFieldValueSet(field.type, values[field.name]))
						context.addIssue({
							code: "custom",
							path: ["values", field.name],
							message: "This field is required.",
						});
				}
			})
			.safeParse({
				name: name.trim(),
				description: description.trim(),
				values: Object.fromEntries(
					schema.fields.map((field) => [field.name, values[field.name]]),
				),
			});
		if (!validation.success) {
			setFieldErrors(
				Object.fromEntries(
					validation.error.issues.map((issue) => [
						String(issue.path.at(-1)),
						issue.message,
					]),
				),
			);
			return;
		}
		setFieldErrors({});
		if (template) {
			updateMutation.mutate({
				path: { product_id: productId, license_template_id: template.id },
				body: validation.data,
			});
		} else {
			createMutation.mutate({
				path: { product_id: productId },
				body: validation.data,
			});
		}
	};

	return (
		<form onSubmit={handleSubmit} className="flex flex-col gap-6">
			<FormAlert message={generalError} />
			<FieldGroup>
				<Field data-invalid={!!fieldErrors.name} data-disabled={isSubmitting}>
					<FieldLabel htmlFor="template-name">Name</FieldLabel>
					<Input
						id="template-name"
						value={name}
						onChange={(event) => setName(event.target.value)}
						placeholder="Pro"
						maxLength={120}
						disabled={isSubmitting}
						aria-invalid={!!fieldErrors.name}
						aria-describedby={
							fieldErrors.name ? "template-name-error" : undefined
						}
					/>
					{fieldErrors.name && (
						<FieldError id="template-name-error">{fieldErrors.name}</FieldError>
					)}
				</Field>
				<Field data-disabled={isSubmitting}>
					<FieldLabel htmlFor="template-description">Description</FieldLabel>
					<Textarea
						id="template-description"
						value={description}
						onChange={(event) => setDescription(event.target.value)}
						placeholder="Optional"
						rows={2}
						disabled={isSubmitting}
					/>
				</Field>
			</FieldGroup>
			<section
				aria-labelledby="template-values-heading"
				className="flex flex-col gap-3"
			>
				<div className="flex flex-col gap-1">
					<h2 id="template-values-heading" className="text-base font-semibold">
						License values
					</h2>
					<p className="text-sm text-muted-foreground">
						{template
							? "Changes propagate to organizations following this template. Their adjusted fields stay unchanged."
							: "Set a value for every field in this product’s license schema."}
					</p>
				</div>
				<LicenseValueFields
					fields={schema.fields}
					values={values}
					onChange={(fieldName, value) =>
						setValues((previous) => ({ ...previous, [fieldName]: value }))
					}
					errors={fieldErrors}
					disabled={isSubmitting}
				/>
			</section>
			<div className="flex justify-end gap-2">
				<Button
					type="button"
					variant="outline"
					onClick={onCancel}
					disabled={isSubmitting}
				>
					Cancel
				</Button>
				<Button type="submit" disabled={isSubmitting}>
					{isSubmitting
						? "Saving…"
						: template
							? "Save Template"
							: "Create Template"}
				</Button>
			</div>
		</form>
	);
}
