import { type WorkflowCondition, WorkflowOperator } from "@/client";
import {
	Button,
	IconButton,
} from "@nanostackorg/design-system/components/button";
import { Input } from "@nanostackorg/design-system/components/input";
import {
	NativeSelect,
	NativeSelectOption,
} from "@nanostackorg/design-system/components/native-select";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { Plus, X } from "lucide-react";
import { useId } from "react";
import { VariableMenu } from "./VariableMenu";
import {
	type WorkflowVariable,
	operatorLabels,
	operatorNeedsValue,
	reference,
} from "./workflow-model";

export function ConditionEditor({
	conditions,
	variables,
	onChange,
	emptyLabel,
	addLabel = "Add condition",
}: {
	conditions: WorkflowCondition[];
	variables: WorkflowVariable[];
	onChange: (conditions: WorkflowCondition[]) => void;
	emptyLabel: string;
	addLabel?: string;
}) {
	const listId = useId();
	const update = (index: number, patch: Partial<WorkflowCondition>) =>
		onChange(
			conditions.map((condition, position) =>
				position === index ? { ...condition, ...patch } : condition,
			),
		);
	const remove = (index: number) =>
		onChange(conditions.filter((_, position) => position !== index));
	const add = () =>
		onChange([
			...conditions,
			{
				field: variables[0]?.path ?? "",
				operator: WorkflowOperator.EQUALS,
				value: "",
			},
		]);

	return (
		<Stack space="sm">
			<datalist id={listId}>
				{variables.map((variable) => (
					<option key={variable.path} value={variable.path}>
						{variable.source} · {variable.label}
					</option>
				))}
			</datalist>
			{conditions.length === 0 ? (
				<Text size="sm" tone="muted">
					{emptyLabel}
				</Text>
			) : null}
			{conditions.map((condition, index) => (
				<Box
					// biome-ignore lint/suspicious/noArrayIndexKey: conditions have no identity of their own
					key={index}
					className="grid grid-cols-1 items-center gap-2 sm:grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)_auto]"
				>
					<Input
						size="sm"
						font="mono"
						list={listId}
						aria-label="Value to test"
						placeholder="event.data.organization_id"
						value={condition.field}
						onChange={(event) => update(index, { field: event.target.value })}
					/>
					<NativeSelect
						size="sm"
						aria-label="Comparison"
						value={condition.operator}
						onChange={(event) =>
							update(index, {
								operator: event.target.value as WorkflowOperator,
							})
						}
					>
						{Object.values(WorkflowOperator).map((operator) => (
							<NativeSelectOption key={operator} value={operator}>
								{operatorLabels[operator]}
							</NativeSelectOption>
						))}
					</NativeSelect>
					{operatorNeedsValue(condition.operator) ? (
						<Box className="flex min-w-0 items-center gap-1">
							<Box className="min-w-0 flex-1">
								<Input
									size="sm"
									aria-label="Compared with"
									placeholder={
										condition.operator === WorkflowOperator.IN
											? "a, b, c"
											: "value"
									}
									value={condition.value ?? ""}
									onChange={(event) =>
										update(index, { value: event.target.value })
									}
								/>
							</Box>
							<VariableMenu
								variables={variables}
								onPick={(path) =>
									update(index, {
										value: `${condition.value ?? ""}${reference(path)}`,
									})
								}
							/>
						</Box>
					) : (
						<Box />
					)}
					<IconButton
						variant="ghost"
						size="sm"
						icon={X}
						label="Remove condition"
						onClick={() => remove(index)}
					/>
				</Box>
			))}
			<Box>
				<Button variant="ghost" size="sm" icon={Plus} onClick={add}>
					{addLabel}
				</Button>
			</Box>
		</Stack>
	);
}
