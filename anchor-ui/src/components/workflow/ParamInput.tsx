import { type WorkflowActionParamResponse, WorkflowParamType } from "@/client";
import {
	Field,
	FieldDescription,
	FieldLabel,
} from "@nanostackorg/design-system/components/field";
import { Input } from "@nanostackorg/design-system/components/input";
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
};

export function ParamInput({
	param,
	value,
	variables,
	resources,
	onChange,
}: {
	param: WorkflowActionParamResponse;
	value: string;
	variables: WorkflowVariable[];
	resources: WorkflowResources;
	onChange: (value: string) => void;
}) {
	const inputId = useId();
	const listId = useId();
	const options = resources[param.type] ?? [];
	const insert = (path: string) => onChange(`${value}${reference(path)}`);
	const label = param.required ? `${param.label} *` : param.label;

	return (
		<Field>
			<Box className="flex items-center justify-between gap-2">
				<FieldLabel htmlFor={inputId} size="sm">
					{label}
				</FieldLabel>
				<VariableMenu
					variables={variables}
					onPick={insert}
					label={`Insert a value into ${param.label}`}
				/>
			</Box>
			{param.type === WorkflowParamType.JSON ? (
				<Textarea
					id={inputId}
					font="mono"
					rows={3}
					placeholder={placeholders[param.type]}
					value={value}
					onChange={(event) => onChange(event.target.value)}
				/>
			) : (
				<>
					<Input
						id={inputId}
						size="sm"
						font={param.type === WorkflowParamType.TEXT ? "sans" : "mono"}
						list={options.length > 0 ? listId : undefined}
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
			)}
			{param.description ? (
				<FieldDescription>{param.description}</FieldDescription>
			) : null}
		</Field>
	);
}
