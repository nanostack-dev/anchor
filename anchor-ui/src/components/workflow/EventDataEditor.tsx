import {
	type WorkflowActionParamResponse,
	WorkflowFieldType,
	WorkflowParamType,
} from "@/client";
import {
	Button,
	IconButton,
} from "@nanostackorg/design-system/components/button";
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
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Plus, X } from "lucide-react";
import { useEffect, useId, useRef, useState } from "react";
import { ParamInput } from "./ParamInput";
import {
	type EventDataRow,
	eventDataKeyPattern,
	fieldTypeLabels,
	inferFieldType,
	parseEventData,
	serializeEventData,
	singleReference,
} from "./fields";
import type { WorkflowVariable } from "./workflow-model";

type EditedRow = EventDataRow & { id: number };

function identify(
	list: EventDataRow[],
	counter: { current: number },
): EditedRow[] {
	return list.map((row) => ({ ...row, id: counter.current++ }));
}

function RowEditor({
	row,
	index,
	duplicate,
	typeProblem,
	focusName,
	variables,
	onChange,
	onRemove,
}: {
	row: EventDataRow;
	index: number;
	duplicate: boolean;
	typeProblem?: string;
	focusName: boolean;
	variables: WorkflowVariable[];
	onChange: (row: EventDataRow) => void;
	onRemove: () => void;
}) {
	const nameId = useId();
	const typeId = useId();
	const nameInput = useRef<HTMLInputElement | null>(null);
	const name = row.key.trim();
	const nameProblem = duplicate
		? `Another field is already named “${name}”.`
		: name && !eventDataKeyPattern.test(name)
			? "Use lowercase letters, digits and underscores, starting with a letter."
			: undefined;
	const valueParam: WorkflowActionParamResponse = {
		name: `value_${index}`,
		label: name ? `Value of ${name}` : `Value of field ${index + 1}`,
		type: WorkflowParamType.TEXT,
		required: false,
		literal: false,
	};

	useEffect(() => {
		if (focusName) nameInput.current?.focus();
	}, [focusName]);

	return (
		<Box
			as="fieldset"
			aria-label={`Event field ${index + 1}`}
			className="@container space-y-2 rounded-lg border border-border bg-card p-3"
		>
			<Box className="flex items-center justify-between gap-2">
				<Text as="span" size="xs" tone="muted">
					Field {index + 1}
				</Text>
				<IconButton
					variant="ghost"
					size="sm"
					icon={X}
					label={`Remove event field ${name || index + 1}`}
					onClick={onRemove}
				/>
			</Box>
			<Box className="grid grid-cols-1 gap-2 @sm:grid-cols-[minmax(0,1fr)_minmax(0,10rem)]">
				<Field invalid={Boolean(nameProblem)}>
					<FieldLabel htmlFor={nameId} size="sm">
						Name
					</FieldLabel>
					<Input
						ref={nameInput}
						id={nameId}
						size="sm"
						font="mono"
						placeholder="plan"
						aria-invalid={nameProblem ? true : undefined}
						value={row.key}
						onChange={(event) => onChange({ ...row, key: event.target.value })}
					/>
					{nameProblem ? <FieldError>{nameProblem}</FieldError> : null}
				</Field>
				<Field invalid={Boolean(typeProblem)}>
					<FieldLabel htmlFor={typeId} size="sm">
						Type
					</FieldLabel>
					<NativeSelect
						id={typeId}
						size="sm"
						aria-invalid={typeProblem ? true : undefined}
						value={row.type}
						onChange={(event) =>
							onChange({
								...row,
								type: event.target.value as WorkflowFieldType,
							})
						}
					>
						{Object.values(WorkflowFieldType).map((type) => (
							<NativeSelectOption key={type} value={type}>
								{fieldTypeLabels[type]}
							</NativeSelectOption>
						))}
					</NativeSelect>
				</Field>
			</Box>
			{typeProblem ? <FieldError>{typeProblem}</FieldError> : null}
			<ParamInput
				param={valueParam}
				value={row.value}
				variables={variables}
				resources={{}}
				onChange={(value) =>
					onChange({
						...row,
						value,
						type: singleReference(value)
							? inferFieldType(value, variables)
							: row.type,
					})
				}
			/>
		</Box>
	);
}

/**
 * The data a custom event carries, as named and typed fields. Data the rows
 * cannot show (nested values, numbers) stays editable as JSON.
 */
export function EventDataEditor({
	dataParam,
	typesParam,
	data,
	types,
	variables,
	errors,
	fieldErrors = {},
	onChange,
}: {
	dataParam: WorkflowActionParamResponse;
	typesParam: WorkflowActionParamResponse;
	data: string;
	types: string;
	variables: WorkflowVariable[];
	errors: Record<string, string>;
	fieldErrors?: Record<string, string>;
	onChange: (next: { data: string; types: string }) => void;
}) {
	const parsed = parseEventData(data, types);
	const [asJSON, setAsJSON] = useState(parsed === undefined);
	const nextId = useRef(0);
	const [rows, setRows] = useState<EditedRow[]>(() =>
		identify(parsed ?? [], nextId),
	);
	const [focusIndex, setFocusIndex] = useState<number>();
	const sent = useRef(serializeEventData(rows));

	useEffect(() => {
		const current = sent.current;
		if (current.data === data && current.types === types) return;
		const next = parseEventData(data, types);
		if (next === undefined) {
			setAsJSON(true);
			return;
		}
		setRows(identify(next, nextId));
		sent.current = serializeEventData(next);
	}, [data, types]);

	const update = (next: EditedRow[]) => {
		setRows(next);
		sent.current = serializeEventData(next);
		onChange(sent.current);
	};
	const names = rows.map((row) => row.key.trim());
	const pinnedOnARow = Object.keys(fieldErrors).some((key) =>
		names.includes(key),
	);
	const problem =
		errors[dataParam.name] ??
		(pinnedOnARow && !asJSON ? undefined : errors[typesParam.name]);

	return (
		<Box
			as="section"
			aria-label="Event data"
			className="space-y-3 border-t border-border pt-4"
		>
			<Box className="flex flex-wrap items-center justify-between gap-2">
				<Box>
					<Text as="p" size="sm" weight="semibold">
						Event data
					</Text>
					<Box as="p" className="text-xs text-muted-foreground">
						Workflows started by this event read each field as
						event.data.&lt;name&gt;, with the type you give it.
					</Box>
				</Box>
				<Button
					variant="ghost"
					size="xs"
					disabled={asJSON && parseEventData(data, types) === undefined}
					onClick={() => {
						if (asJSON) {
							const next = parseEventData(data, types);
							if (next === undefined) return;
							setRows(identify(next, nextId));
							sent.current = serializeEventData(next);
						}
						setAsJSON(!asJSON);
					}}
				>
					{asJSON ? "Edit as fields" : "Edit as JSON"}
				</Button>
			</Box>
			{problem ? <FieldError>{problem}</FieldError> : null}
			{asJSON ? (
				<>
					{parseEventData(data, types) === undefined ? (
						<FieldDescription>
							Fields hold text values only. Nested objects and numbers stay
							editable here as JSON.
						</FieldDescription>
					) : null}
					<ParamInput
						param={dataParam}
						value={data}
						variables={variables}
						resources={{}}
						onChange={(value) => onChange({ data: value, types })}
					/>
					<ParamInput
						param={typesParam}
						value={types}
						variables={variables}
						resources={{}}
						onChange={(value) => onChange({ data, types: value })}
					/>
				</>
			) : (
				<>
					{rows.length === 0 ? (
						<Box as="p" className="text-xs text-muted-foreground">
							The event carries no data yet.
						</Box>
					) : null}
					{rows.map((row, index) => (
						<RowEditor
							key={row.id}
							row={row}
							index={index}
							duplicate={
								row.key.trim() !== "" && names.indexOf(row.key.trim()) !== index
							}
							typeProblem={fieldErrors[row.key.trim()]}
							focusName={index === focusIndex}
							variables={variables}
							onChange={(next) =>
								update(
									rows.map((current) =>
										current.id === row.id ? { ...next, id: row.id } : current,
									),
								)
							}
							onRemove={() =>
								update(rows.filter((current) => current.id !== row.id))
							}
						/>
					))}
					<Box>
						<Button
							variant="outline"
							size="sm"
							icon={Plus}
							onClick={() => {
								setFocusIndex(rows.length);
								update([
									...rows,
									{
										id: nextId.current++,
										key: "",
										value: "",
										type: WorkflowFieldType.TEXT,
									},
								]);
							}}
						>
							Add field
						</Button>
					</Box>
				</>
			)}
		</Box>
	);
}
