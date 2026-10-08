import {
	type WorkflowCondition,
	WorkflowFieldType,
	WorkflowOperator,
	WorkflowParamType,
} from "@/client";
import { cn } from "@/lib/utils";
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
import { Braces, ChevronDown, Plus, X } from "lucide-react";
import { useId, useState } from "react";
import { FieldPicker } from "./FieldPicker";
import { fieldTypeIcons } from "./field-icons";
import {
	FIELD_DRAG_TYPE,
	comparedValueHint,
	fieldAt,
	fieldTypeLabels,
	isIdentifier,
	isInsideJsonField,
	operatorsFor,
} from "./fields";
import type { WorkflowResources } from "./useWorkflowResources";
import {
	type WorkflowVariable,
	operatorLabels,
	operatorNeedsValue,
	reference,
} from "./workflow-model";

const resourceTypes: Partial<Record<WorkflowFieldType, WorkflowParamType>> = {
	[WorkflowFieldType.ROLE]: WorkflowParamType.ROLE,
	[WorkflowFieldType.LICENSE_TEMPLATE]: WorkflowParamType.LICENSE_TEMPLATE,
};

function ConditionRow({
	name,
	condition,
	variables,
	resources,
	onChange,
	onRemove,
}: {
	name: string;
	condition: WorkflowCondition;
	variables: WorkflowVariable[];
	resources: WorkflowResources;
	onChange: (patch: Partial<WorkflowCondition>) => void;
	onRemove: () => void;
}) {
	const listId = useId();
	const warningId = useId();
	const [dropping, setDropping] = useState(false);
	const path = condition.field.trim();
	const field = path ? fieldAt(variables, path) : undefined;
	const type = field?.type;
	const custom = !field && Boolean(path) && isInsideJsonField(variables, path);
	const missing = Boolean(path) && !field && !custom;
	const allowed = operatorsFor(type);
	const operatorFits = allowed.includes(condition.operator);
	const operators = operatorFits ? allowed : [condition.operator, ...allowed];
	const needsValue = operatorNeedsValue(condition.operator);
	const isList = condition.operator === WorkflowOperator.IN;
	const FieldIcon = field ? fieldTypeIcons[field.type] : Braces;
	const resourceType = type ? resourceTypes[type] : undefined;
	const options =
		resourceType && !isList ? (resources[resourceType] ?? []) : [];
	const value = condition.value ?? "";
	const yesOrNo = value.toLowerCase();
	const strayYesOrNo =
		value !== "" && yesOrNo !== "true" && yesOrNo !== "false";

	const chooseField = (next: string) => {
		const nextType = fieldAt(variables, next)?.type;
		const nextAllowed = operatorsFor(nextType);
		onChange({
			field: next,
			operator: nextAllowed.includes(condition.operator)
				? condition.operator
				: nextAllowed[0],
			value: nextType === type || missing ? condition.value : "",
		});
	};

	const fieldDescription = field
		? `${field.label} from ${field.source}, ${fieldTypeLabels[field.type]}`
		: custom
			? `${path}, a custom path`
			: path
				? `${path}, not available here`
				: "pick a field";

	return (
		<Box
			as="fieldset"
			aria-label={name}
			className="@container space-y-2 border-border border-b pb-3 last:border-b-0 last:pb-0"
		>
			<Box className="flex items-center gap-2">
				<Box
					className={cn(
						"min-w-0 flex-1 rounded-3xl transition-[box-shadow] duration-150 ease-out",
						dropping && "ring-2 ring-primary/50",
					)}
					onDragOver={(event) => {
						if (!event.dataTransfer.types.includes(FIELD_DRAG_TYPE)) return;
						event.preventDefault();
						event.dataTransfer.dropEffect = "copy";
						setDropping(true);
					}}
					onDragLeave={() => setDropping(false)}
					onDrop={(event) => {
						setDropping(false);
						const dropped = event.dataTransfer.getData(FIELD_DRAG_TYPE);
						if (!dropped) return;
						event.preventDefault();
						chooseField(dropped);
					}}
				>
					<FieldPicker
						fields={variables}
						allowPath
						selected={path}
						label={`${name}: value to test`}
						onPick={chooseField}
						trigger={
							<button
								type="button"
								aria-label={`${name}: value to test: ${fieldDescription}`}
								aria-describedby={missing ? warningId : undefined}
								title={field?.description ?? path}
								className="flex h-8 w-full min-w-0 items-center gap-2 rounded-3xl border border-transparent bg-input/50 px-3 text-left text-sm outline-none transition-[transform,box-shadow] duration-150 ease-out focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/30 active:scale-[0.99] motion-reduce:active:scale-100"
							>
								<FieldIcon
									className="size-3.5 shrink-0 text-muted-foreground"
									aria-hidden
								/>
								<Box
									as="span"
									className={cn(
										"max-w-[60%] shrink-0 truncate font-mono text-xs",
										!path && "text-muted-foreground",
										missing && "text-warning-on-tint",
									)}
								>
									{field?.label ?? (path || "Pick a field")}
								</Box>
								<Box
									as="span"
									className="ml-auto hidden min-w-0 truncate text-muted-foreground text-xs @xs:inline"
								>
									{field
										? `${field.source} · ${fieldTypeLabels[field.type]}`
										: custom
											? "Custom path"
											: missing
												? "Not available here"
												: ""}
								</Box>
								<ChevronDown
									className="size-3.5 shrink-0 text-muted-foreground"
									aria-hidden
								/>
							</button>
						}
					/>
				</Box>
				<IconButton
					variant="ghost"
					size="sm"
					icon={X}
					label={`Remove ${name.toLowerCase()}`}
					onClick={onRemove}
				/>
			</Box>
			<Box className="grid grid-cols-1 gap-2 @sm:grid-cols-[minmax(0,10rem)_minmax(0,1fr)]">
				<NativeSelect
					size="sm"
					aria-label={`${name}: comparison`}
					aria-describedby={!operatorFits ? warningId : undefined}
					value={condition.operator}
					onChange={(event) =>
						onChange({ operator: event.target.value as WorkflowOperator })
					}
				>
					{operators.map((operator) => (
						<NativeSelectOption key={operator} value={operator}>
							{operator === condition.operator && !operatorFits
								? `${operatorLabels[operator]} (does not fit)`
								: operatorLabels[operator]}
						</NativeSelectOption>
					))}
				</NativeSelect>
				{needsValue ? (
					<Box className="flex min-w-0 items-center gap-1">
						<Box className="min-w-0 flex-1">
							{type === WorkflowFieldType.BOOLEAN ? (
								<NativeSelect
									size="sm"
									aria-label={`${name}: compared with`}
									value={strayYesOrNo ? value : yesOrNo}
									onChange={(event) => onChange({ value: event.target.value })}
								>
									<NativeSelectOption value="">
										Pick yes or no
									</NativeSelectOption>
									{strayYesOrNo ? (
										<NativeSelectOption value={value}>
											{value} (not yes or no)
										</NativeSelectOption>
									) : null}
									<NativeSelectOption value="true">Yes</NativeSelectOption>
									<NativeSelectOption value="false">No</NativeSelectOption>
								</NativeSelect>
							) : (
								<>
									<Input
										size="sm"
										font={
											isIdentifier(type) || type === WorkflowFieldType.NUMBER
												? "mono"
												: "sans"
										}
										inputMode={
											type === WorkflowFieldType.NUMBER && !isList
												? "decimal"
												: undefined
										}
										list={options.length > 0 ? listId : undefined}
										aria-label={`${name}: compared with`}
										placeholder={comparedValueHint(type, condition.operator)}
										value={condition.value ?? ""}
										onChange={(event) =>
											onChange({ value: event.target.value })
										}
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
						</Box>
						{type === WorkflowFieldType.BOOLEAN ? null : (
							<FieldPicker
								label={`Insert a value into ${name.toLowerCase()}`}
								fields={variables.filter((variable) => variable.path !== path)}
								fitType={type}
								onPick={(picked) =>
									onChange({
										value: `${condition.value ?? ""}${reference(picked)}`,
									})
								}
							/>
						)}
					</Box>
				) : null}
			</Box>
			{missing ? (
				<Text id={warningId} size="xs" tone="warning">
					“{path}” is not something this condition can read here. Pick a field.
				</Text>
			) : !operatorFits && type ? (
				<Text id={warningId} size="xs" tone="warning">
					{`${fieldTypeLabels[type]} fields cannot use “${operatorLabels[condition.operator]}”. Pick another comparison.`}
				</Text>
			) : null}
		</Box>
	);
}

/**
 * Conditions on typed fields: the field comes from a picker, the comparisons
 * fit its type, and the value input takes the shape the type expects.
 */
export function ConditionEditor({
	label,
	conditions,
	variables,
	resources = {},
	onChange,
	emptyLabel,
	addLabel = "Add condition",
}: {
	label: string;
	conditions: WorkflowCondition[];
	variables: WorkflowVariable[];
	resources?: WorkflowResources;
	onChange: (conditions: WorkflowCondition[]) => void;
	emptyLabel: string;
	addLabel?: string;
}) {
	const update = (index: number, patch: Partial<WorkflowCondition>) =>
		onChange(
			conditions.map((condition, position) =>
				position === index ? { ...condition, ...patch } : condition,
			),
		);
	const remove = (index: number) =>
		onChange(conditions.filter((_, position) => position !== index));
	const add = () => {
		const first = variables[0];
		onChange([
			...conditions,
			{
				field: first?.path ?? "",
				operator: operatorsFor(first?.type)[0],
				value: "",
			},
		]);
	};

	return (
		<Stack space="sm">
			{conditions.length === 0 ? (
				<Text size="sm" tone="muted">
					{emptyLabel}
				</Text>
			) : null}
			{conditions.length > 0 ? (
				<Box className="flex flex-col gap-3">
					{conditions.map((condition, index) => (
						<ConditionRow
							// biome-ignore lint/suspicious/noArrayIndexKey: conditions have no identity of their own
							key={index}
							name={`${label} ${index + 1}`}
							condition={condition}
							variables={variables}
							resources={resources}
							onChange={(patch) => update(index, patch)}
							onRemove={() => remove(index)}
						/>
					))}
				</Box>
			) : null}
			<Box>
				<Button variant="ghost" size="sm" icon={Plus} onClick={add}>
					{addLabel}
				</Button>
			</Box>
		</Stack>
	);
}
