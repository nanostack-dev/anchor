import { type WorkflowActionParamResponse, WorkflowParamType } from "@/client";
import { cn } from "@/lib/utils";
import { IconButton } from "@nanostackorg/design-system/components/button";
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
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import { Box } from "@nanostackorg/design-system/layout/box";
import { X } from "lucide-react";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { FieldPicker } from "./FieldPicker";
import { fieldTypeIcons } from "./field-icons";
import { useFieldTargets } from "./field-target";
import {
	FIELD_DRAG_TYPE,
	completeReference,
	expectedTypeLabel,
	fieldAt,
	fieldTypeLabels,
	fits,
	insertReference,
	matchesQuery,
	openReference,
	singleReference,
} from "./fields";
import type { WorkflowResources } from "./useWorkflowResources";
import type { WorkflowVariable } from "./workflow-model";

const placeholders: Partial<Record<WorkflowParamType, string>> = {
	[WorkflowParamType.ORGANIZATION]: "org_… or drop a field",
	[WorkflowParamType.PRODUCT_USER]: "pusr_… or drop a field",
	[WorkflowParamType.ROLE]: "Pick a role",
	[WorkflowParamType.LICENSE_TEMPLATE]: "Pick a license template",
	[WorkflowParamType.EMAIL_TEMPLATE]: "Pick an email template slug",
	[WorkflowParamType.EMAIL]: "someone@example.com or drop a field",
	[WorkflowParamType.JSON]: '{"key": "value"}',
	[WorkflowParamType.URL]: "https://api.example.com/hooks/anchor",
	[WorkflowParamType.CUSTOM_EVENT]: "onboarding.started",
};

const MAX_SUGGESTIONS = 8;

type TextControl = HTMLInputElement | HTMLTextAreaElement;

function hasField(event: React.DragEvent) {
	return event.dataTransfer.types.includes(FIELD_DRAG_TYPE);
}

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
	const suggestionsId = useId();
	const control = useRef<TextControl | null>(null);
	const pill = useRef<HTMLButtonElement | null>(null);
	const hadFocus = useRef(false);
	const caret = useRef<number | null>(null);
	const latest = useRef({ value, onChange });
	latest.current = { value, onChange };
	const targets = useFieldTargets();
	const [editing, setEditing] = useState(false);
	const [dropping, setDropping] = useState(false);
	const [typed, setTyped] = useState<{ start: number; query: string }>();
	const [active, setActive] = useState(0);

	const options = resources[param.type] ?? [];
	const label = param.required ? `${param.label} *` : param.label;
	const invalid = Boolean(error);
	const takesFields = !param.literal && !param.options?.length;
	const json = param.type === WorkflowParamType.JSON;
	const path = singleReference(value);
	const referenced = path ? fieldAt(variables, path) : undefined;
	const showPill = takesFields && !json && !editing && Boolean(path);
	const expected = expectedTypeLabel(param.type);
	const mismatch =
		referenced && !fits(param.type, referenced.type) ? referenced : undefined;
	const suggestions = typed
		? variables
				.filter((field) => matchesQuery(field, typed.query))
				.slice(0, MAX_SUGGESTIONS)
		: [];

	const insert = useCallback((fieldPath: string) => {
		const { value: current, onChange: change } = latest.current;
		const next = insertReference(current, caret.current, fieldPath);
		change(next.value);
		caret.current = next.caret;
	}, []);

	useEffect(() => {
		if (editing) control.current?.focus();
	}, [editing]);

	useEffect(() => {
		const lost =
			!document.activeElement || document.activeElement === document.body;
		if (showPill && hadFocus.current && lost) pill.current?.focus();
	}, [showPill]);

	const release = useRef(targets?.release);
	release.current = targets?.release;
	useEffect(() => () => release.current?.(insert), [insert]);

	const remember = (element: TextControl) => {
		caret.current = element.selectionStart;
	};

	const changeText = (element: TextControl) => {
		remember(element);
		onChange(element.value);
		const open = openReference(element.value, element.selectionStart ?? 0);
		setTyped(open);
		setActive(0);
	};

	const choose = (field: WorkflowVariable) => {
		if (!typed) return;
		const element = control.current;
		const at = element?.selectionStart ?? value.length;
		const next = completeReference(value, at, typed.start, field.path);
		onChange(next.value);
		caret.current = next.caret;
		setTyped(undefined);
		requestAnimationFrame(() =>
			control.current?.setSelectionRange(next.caret, next.caret),
		);
	};

	const keyDown = (event: React.KeyboardEvent<TextControl>) => {
		if (!typed || suggestions.length === 0) return;
		if (event.key === "ArrowDown") {
			event.preventDefault();
			setActive((current) => (current + 1) % suggestions.length);
		} else if (event.key === "ArrowUp") {
			event.preventDefault();
			setActive(
				(current) => (current - 1 + suggestions.length) % suggestions.length,
			);
		} else if (event.key === "Enter" || event.key === "Tab") {
			event.preventDefault();
			choose(suggestions[active]);
		} else if (event.key === "Escape") {
			setTyped(undefined);
		}
	};

	const textProps = {
		id: inputId,
		ref: (element: TextControl | null) => {
			control.current = element;
		},
		"aria-invalid": invalid || undefined,
		placeholder: placeholders[param.type],
		value,
		onFocus: (event: React.FocusEvent<TextControl>) => {
			remember(event.currentTarget);
			if (takesFields) targets?.focus({ label: param.label, insert });
		},
		onSelect: (event: React.SyntheticEvent<TextControl>) =>
			remember(event.currentTarget),
		onBlur: () => {
			setEditing(false);
			setTyped(undefined);
		},
		onKeyDown: takesFields ? keyDown : undefined,
		...(typed && suggestions.length > 0
			? {
					role: "combobox",
					"aria-expanded": true,
					"aria-controls": suggestionsId,
					"aria-autocomplete": "list" as const,
					"aria-activedescendant": `${suggestionsId}-${active}`,
				}
			: {}),
	};

	const dropZone = takesFields
		? {
				onDragOver: (event: React.DragEvent) => {
					if (!hasField(event)) return;
					event.preventDefault();
					event.dataTransfer.dropEffect = "copy";
					setDropping(true);
				},
				onDragLeave: () => setDropping(false),
				onDrop: (event: React.DragEvent) => {
					setDropping(false);
					const dropped = event.dataTransfer.getData(FIELD_DRAG_TYPE);
					if (!dropped) return;
					const freeText = value.trim() && !path;
					if (freeText && event.target === control.current) return;
					event.preventDefault();
					insert(dropped);
				},
			}
		: {};

	const PillIcon = referenced ? fieldTypeIcons[referenced.type] : undefined;
	const pillName = referenced?.label ?? path ?? "";
	const pillSource = referenced?.source ?? "not available here";

	const body = (() => {
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
		if (showPill) {
			return (
				<Box className="flex h-8 w-full min-w-0 items-center gap-1 rounded-3xl border border-transparent bg-input/50 px-1.5">
					<button
						ref={pill}
						type="button"
						id={inputId}
						aria-label={`${label}: ${pillName} from ${pillSource}. Edit`}
						title={referenced?.description ?? path}
						onClick={() => setEditing(true)}
						className="inline-flex min-w-0 max-w-full items-center gap-1.5 rounded-full bg-primary/10 px-2 py-0.5 text-xs font-medium text-foreground outline-none transition-transform duration-150 ease-out focus-visible:ring-2 focus-visible:ring-ring/60 active:scale-[0.97] motion-reduce:active:scale-100"
					>
						{PillIcon ? (
							<PillIcon className="size-3 shrink-0 text-primary" aria-hidden />
						) : null}
						<Box as="span" className="max-w-56 shrink-0 truncate font-mono">
							{pillName}
						</Box>
						<Box
							as="span"
							className="hidden min-w-0 truncate text-muted-foreground sm:inline"
						>
							· {pillSource}
						</Box>
					</button>
					<Box className="ml-auto">
						<IconButton
							variant="ghost"
							size="xs"
							icon={X}
							label={`Clear ${param.label}`}
							onClick={() => {
								onChange("");
								setEditing(true);
							}}
						/>
					</Box>
				</Box>
			);
		}
		if (json) {
			return (
				<Textarea
					{...textProps}
					font="mono"
					rows={3}
					onChange={(event) => changeText(event.currentTarget)}
				/>
			);
		}
		return (
			<>
				<Input
					{...textProps}
					size="sm"
					font={param.type === WorkflowParamType.TEXT ? "sans" : "mono"}
					list={options.length > 0 && !typed ? listId : undefined}
					onChange={(event) => changeText(event.currentTarget)}
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
				{takesFields ? (
					<FieldPicker
						fields={variables}
						paramType={param.type}
						label={`Insert a value into ${param.label}`}
						onPick={insert}
					/>
				) : null}
			</Box>
			<Box
				{...dropZone}
				onFocus={() => {
					hadFocus.current = true;
				}}
				onBlur={(event: React.FocusEvent<HTMLDivElement>) => {
					if (!event.currentTarget.contains(event.relatedTarget as Node | null))
						hadFocus.current = false;
				}}
				className={cn(
					"relative rounded-3xl transition-[box-shadow] duration-150 ease-out",
					dropping && "ring-2 ring-primary/50",
				)}
			>
				{body}
				{typed && suggestions.length > 0 ? (
					<Box className="absolute inset-x-0 top-full z-30 mt-1 rounded-xl border border-border bg-popover text-popover-foreground shadow-lg">
						<Box
							// biome-ignore lint/a11y/useSemanticElements: the list of an ARIA combobox, which a native select cannot be
							role="listbox"
							id={suggestionsId}
							aria-label={`Fields for ${param.label}`}
							className="p-1"
						>
							{suggestions.map((field, index) => {
								const Icon = fieldTypeIcons[field.type];
								return (
									<Box
										key={field.path}
										// biome-ignore lint/a11y/useSemanticElements: an option of the ARIA combobox list above
										id={`${suggestionsId}-${index}`}
										role="option"
										aria-selected={index === active}
										onMouseDown={(event) => {
											event.preventDefault();
											choose(field);
										}}
										onMouseEnter={() => setActive(index)}
										className={cn(
											"flex cursor-pointer items-center gap-2 rounded-lg px-2 py-1.5 text-xs",
											index === active && "bg-accent text-accent-foreground",
										)}
									>
										<Icon
											className="size-3.5 shrink-0 text-muted-foreground"
											aria-hidden
										/>
										<Box
											as="span"
											className="min-w-0 flex-1 truncate font-mono"
										>
											{field.label}
										</Box>
										<Box as="span" className="shrink-0 text-muted-foreground">
											{field.source} · {fieldTypeLabels[field.type]}
										</Box>
									</Box>
								);
							})}
						</Box>
					</Box>
				) : null}
			</Box>
			{mismatch ? (
				<Text size="xs" tone="warning">
					{param.label} expects {expected ?? "text"}; “{mismatch.label}” is{" "}
					{fieldTypeLabels[mismatch.type]}.
				</Text>
			) : null}
			{param.description ? (
				<FieldDescription>{param.description}</FieldDescription>
			) : null}
			{error ? <FieldError>{error}</FieldError> : null}
		</Field>
	);
}
