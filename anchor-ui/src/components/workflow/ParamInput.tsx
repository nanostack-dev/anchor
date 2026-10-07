import { type WorkflowActionParamResponse, WorkflowParamType } from "@/client";
import {
	Field,
	FieldDescription,
	FieldError,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
import {
	NativeSelect,
	NativeSelectOption,
} from "@nanostackorg/design-system/components/native-select";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { Box } from "@nanostackorg/design-system/layout/box";
import { useId } from "react";
import { VariableMenu } from "./VariableMenu";
import type { WorkflowResources } from "./useWorkflowResources";
import { type WorkflowVariable, reference } from "./workflow-model";

const placeholders: Partial<Record<WorkflowParamType, string>> = {
	[WorkflowParamType.ORGANIZATION]: "org_… or {{event.data.organization_id}}",
	[WorkflowParamType.PRODUCT_USER]: "pusr_… or {{event.data.product_user_id}}",
	[WorkflowParamType.ROLE]: "Pick a role",
	[WorkflowParamType.LICENSE_TEMPLATE]: "Pick a license template",
	[WorkflowParamType.EMAIL_TEMPLATE]: "Pick an email template slug",
	[WorkflowParamType.EMAIL]: "someone@example.com",
	[WorkflowParamType.JSON]: '{"key": "value"}',
	[WorkflowParamType.URL]: "https://api.example.com/hooks/anchor",
	[WorkflowParamType.CUSTOM_EVENT]: "onboarding.started",
};

export function ParamInput({
	param,
	value,
	variables,
	resources,
	error,
	onChange,
}: {
	param: WorkflowActionParamResponse;
	value: string;
	variables: WorkflowVariable[];
	resources: WorkflowResources;
	error?: string;
	onChange: (value: string) => void;
}) {
	const inputId = useId();
	const listId = useId();
	const options = resources[param.type] ?? [];
	const insert = (path: string) => onChange(`${value}${reference(path)}`);
	const label = param.required ? `${param.label} *` : param.label;
	const invalid = Boolean(error);

	const control = (() => {
		if (param.options?.length) {
			return (
				<NativeSelect
					id={inputId}
					size="sm"
					aria-invalid={invalid || undefined}
					value={value}
					onChange={(event) => onChange(event.target.value)}
				>
					<NativeSelectOption value="">Default</NativeSelectOption>
					{param.options.map((option) => (
						<NativeSelectOption key={option} value={option}>
							{option}
						</NativeSelectOption>
					))}
				</NativeSelect>
			);
		}
		if (param.type === WorkflowParamType.JSON) {
			return (
				<Textarea
					id={inputId}
					font="mono"
					rows={3}
					aria-invalid={invalid || undefined}
					placeholder={placeholders[param.type]}
					value={value}
					onChange={(event) => onChange(event.target.value)}
				/>
			);
		}
		return (
			<>
				<Input
					id={inputId}
					size="sm"
					font={param.type === WorkflowParamType.TEXT ? "sans" : "mono"}
					list={options.length > 0 ? listId : undefined}
					aria-invalid={invalid || undefined}
					placeholder={placeholders[param.type]}
					value={value}
					onChange={(event) => onChange(event.target.value)}
				/>
				{options.length > 0 ? (
					<datalist id={listId}>
						{options.map((option) => (
							<option key={option.value} value={option.value}>
								{option.label}
							</option>
						))}
					</datalist>
				) : null}
			</>
		);
	})();

	return (
		<Field invalid={invalid}>
			<Box className="flex items-center justify-between gap-2">
				<FieldLabel htmlFor={inputId} size="sm">
					{label}
				</FieldLabel>
				{param.literal || param.options?.length ? null : (
					<VariableMenu
						variables={variables}
						onPick={insert}
						label={`Insert a value into ${param.label}`}
					/>
				)}
			</Box>
			{control}
			{param.description ? (
				<FieldDescription>{param.description}</FieldDescription>
			) : null}
			{error ? <FieldError>{error}</FieldError> : null}
		</Field>
	);
}
