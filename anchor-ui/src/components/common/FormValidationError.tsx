import { FieldError } from "@nanostackorg/design-system/components/field";
import type { StandardSchemaV1 } from "@standard-schema/spec";
import type { AnyFieldApi } from "@tanstack/react-form";

export interface FormErrorProps {
	field: AnyFieldApi;
}

export function FormValidationError({ field }: FormErrorProps) {
	if (!field.state.meta.isTouched || !field.state.meta.errors?.length)
		return null;
	const issues = field.state.meta.errors as StandardSchemaV1.Issue[];
	return (
		<FieldError
			errors={issues.map((issue) => ({
				message: issue.message || "Unknown error",
			}))}
		/>
	);
}
