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
import { getApiErrorMessage, getApiFieldErrors } from "@/lib/api-error";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Field,
	FieldError,
	FieldGroup,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Input } from "@nanostackorg/design-system/components/input";
import { Text } from "@nanostackorg/design-system/components/text";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { toast } from "@nanostackorg/design-system/components/toast";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
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
		toast.add({
			type: "success",
			title: template ? "License template updated" : "License template created",
			description: saved.name,
		});
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
		<Stack onSubmit={handleSubmit} space="xl" as="form">
			<FormAlert message={generalError} />
			<FieldGroup>
				<Field invalid={!!fieldErrors.name} disabled={isSubmitting}>
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
				<Field disabled={isSubmitting}>
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
			<Stack aria-labelledby="template-values-heading" space="md" as="section">
				<Stack space="xs">
					<Heading id="template-values-heading" level={2}>
						License values
					</Heading>
					<Text tone="muted">
						{template
							? "Changes propagate to organizations following this template. Their adjusted fields stay unchanged."
							: "Set a value for every field in this product’s license schema."}
					</Text>
				</Stack>
				<LicenseValueFields
					fields={schema.fields}
					values={values}
					onChange={(fieldName, value) =>
						setValues((previous) => ({ ...previous, [fieldName]: value }))
					}
					errors={fieldErrors}
					disabled={isSubmitting}
				/>
			</Stack>
			<Inline space="sm" align="end" wrap={false}>
				<Button
					type="button"
					variant="outline"
					onClick={onCancel}
					disabled={isSubmitting}
				>
					Cancel
				</Button>
				<Button
					variant="solid"
					tone="brand"
					type="submit"
					disabled={isSubmitting}
				>
					{isSubmitting
						? "Saving…"
						: template
							? "Save Template"
							: "Create Template"}
				</Button>
			</Inline>
		</Stack>
	);
}
