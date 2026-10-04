import {
	type ApiErrorResponse,
	type EmailTemplateVersionResponse,
	type EmailVariableSchema,
	type EmailVariableSchemaItems,
	type EmailVariableSchemaProperty,
	EmailVariableType,
	IntegrationProviderType,
	type TemplateExample,
} from "@/client";
import {
	getEmailTemplateDraftOptions,
	getEmailTemplateExamplesOptions,
	getEmailTemplateOptions,
	listIntegrationInstancesOptions,
	previewEmailTemplateMutation,
	publishEmailTemplateMutation,
	saveEmailTemplateExamplesMutation,
	sendEmailMutation,
	updateEmailTemplateDraftMutation,
	updateEmailTemplateMutation,
} from "@/client/@tanstack/react-query.gen";
import { StatusBadge } from "@/components/common/StatusBadge";
import { useIsMobile } from "@/hooks/use-mobile";
import { ROUTE_PATHS } from "@/routes/routePaths";
import Editor from "@monaco-editor/react";
import { CopyButton } from "@nanostackorg/design-system/blocks/copy-button";
import {
	Button,
	IconButton,
} from "@nanostackorg/design-system/components/button";
import { Checkbox } from "@nanostackorg/design-system/components/checkbox";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@nanostackorg/design-system/components/dialog";
import { Input } from "@nanostackorg/design-system/components/input";
import { Label } from "@nanostackorg/design-system/components/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@nanostackorg/design-system/components/select";
import { Separator } from "@nanostackorg/design-system/components/separator";
import {
	Tabs,
	TabsContent,
	TabsList,
	TabsTrigger,
} from "@nanostackorg/design-system/components/tabs";
import { Text } from "@nanostackorg/design-system/components/text";
import { TextLink } from "@nanostackorg/design-system/components/text-link";
import { Textarea } from "@nanostackorg/design-system/components/textarea";
import {
	Tooltip,
	TooltipContent,
	TooltipTrigger,
} from "@nanostackorg/design-system/components/tooltip";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Inline } from "@nanostackorg/design-system/layout/inline";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	WarningCircleIcon as AlertCircle,
	CheckIcon as Check,
	CodeIcon as Code2,
	FileTextIcon as FileText,
	LayoutIcon as LayoutTemplate,
	PlusIcon as Plus,
	FloppyDiskIcon as Save,
	TrashIcon as Trash2,
} from "@phosphor-icons/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";

const VARIABLE_TYPES: EmailVariableType[] = [
	EmailVariableType.STRING,
	EmailVariableType.NUMBER,
	EmailVariableType.BOOL,
	EmailVariableType.LIST,
	EmailVariableType.OBJECT,
];

type DetectedSchema = {
	name: string;
	type: EmailVariableType;
	items?: EmailVariableSchemaItems;
};

function syncEditorKeys(keys: string[], targetLength: number): string[] {
	const nextKeys = keys.slice(0, targetLength);

	while (nextKeys.length < targetLength) {
		nextKeys.push(crypto.randomUUID());
	}

	return nextKeys;
}

function getApiErrorMessage(error: unknown, fallback: string): string {
	if (
		typeof error === "object" &&
		error !== null &&
		"errors" in error &&
		Array.isArray((error as ApiErrorResponse).errors)
	) {
		return (error as ApiErrorResponse).errors[0]?.message ?? fallback;
	}

	if (error instanceof Error && error.message) {
		return error.message;
	}

	return fallback;
}

function getApiErrorCode(error: unknown): string | undefined {
	if (
		typeof error === "object" &&
		error !== null &&
		"errors" in error &&
		Array.isArray((error as ApiErrorResponse).errors)
	) {
		return (error as ApiErrorResponse).errors[0]?.code;
	}

	return undefined;
}

function stringifyExampleValues(
	vars: Record<string, unknown>,
): Record<string, string> {
	const out: Record<string, string> = {};

	for (const [k, v] of Object.entries(vars)) {
		if (v !== null && v !== undefined) {
			out[k] = typeof v === "object" ? JSON.stringify(v) : String(v);
		}
	}

	return out;
}

// Extracts all .ident references from a template fragment.
function extractRefs(src: string): string[] {
	const seen = new Set<string>();
	const re = /\{\{-?\s*[^}]*?\.(\w+)/g;
	let match = re.exec(src);

	while (match !== null) {
		seen.add(match[1]);
		match = re.exec(src);
	}

	return Array.from(seen);
}

// Nesting-aware range block extractor.
// Lazy regex fails when range body contains {{ if }}/{{ with }} blocks — it stops
// at the first {{ end }} instead of the range's closing {{ end }}.
// This walks the source character-by-character, tracking block depth.
function extractRangeBlocks(src: string): Array<{
	name: string;
	inner: string;
	start: number;
	end: number;
}> {
	const results: Array<{
		name: string;
		inner: string;
		start: number;
		end: number;
	}> = [];
	const rangeStartRe = /\{\{-?\s*range\s+\.(\w+)[^}]*\}\}/g;
	let match = rangeStartRe.exec(src);

	while (match !== null) {
		const name = match[1];
		const blockStart = match.index;
		const innerStart = match.index + match[0].length;

		let depth = 1;
		let pos = innerStart;
		let innerEnd = -1;
		let blockEnd = -1;

		// regex for block-openers that increment depth
		const openerRe = /\{\{-?\s*(?:range|if|with|block|define|template)\b/g;
		// regex for {{ end }} that decrements depth
		const closerRe = /\{\{-?\s*end\s*-?\}\}/g;

		while (depth > 0 && pos < src.length) {
			openerRe.lastIndex = pos;
			closerRe.lastIndex = pos;
			const nextOpen = openerRe.exec(src);
			const nextClose = closerRe.exec(src);

			if (!nextClose) break;

			if (nextOpen && nextOpen.index < nextClose.index) {
				depth++;
				pos = nextOpen.index + nextOpen[0].length;
			} else {
				depth--;
				if (depth === 0) {
					innerEnd = nextClose.index;
					blockEnd = nextClose.index + nextClose[0].length;
				}
				pos = nextClose.index + nextClose[0].length;
			}
		}

		if (innerEnd !== -1) {
			results.push({
				name,
				inner: src.slice(innerStart, innerEnd),
				start: blockStart,
				end: blockEnd,
			});
			// Skip past the whole range block so nested ranges aren't double-counted.
			rangeStartRe.lastIndex = blockEnd;
		}

		match = rangeStartRe.exec(src);
	}

	return results;
}

// Parses template text into detected variable schemas.
// - {{ range .list }}...{{ end }} → list: LIST · OBJECT {fields}
// - {{ .var }} outside range → var: STRING (unknown type, user refines)
function detectVariableSchemas(
	subject: string,
	html: string,
): DetectedSchema[] {
	const combined = `${subject}\n${html}`;
	const results = new Map<string, DetectedSchema>();

	// First pass: nesting-aware range block extraction.
	const blocks = extractRangeBlocks(combined);
	for (const { name, inner } of blocks) {
		const fields = extractRefs(inner).filter((f) => f !== name);
		const props: EmailVariableSchemaProperty[] = fields.map((f) => ({
			name: f,
			type: EmailVariableType.STRING,
		}));
		results.set(name, {
			name,
			type: EmailVariableType.LIST,
			items: {
				type:
					props.length > 0
						? EmailVariableType.OBJECT
						: EmailVariableType.STRING,
				properties: props.length > 0 ? props : undefined,
			},
		});
	}

	// Strip range blocks from source (reverse order to preserve indices).
	let stripped = combined;
	const sortedBlocks = [...blocks].sort((a, b) => b.start - a.start);
	for (const { start, end } of sortedBlocks) {
		stripped = stripped.slice(0, start) + stripped.slice(end);
	}

	// Second pass: top-level refs not already covered by a range var.
	for (const name of extractRefs(stripped)) {
		if (!results.has(name)) {
			results.set(name, { name, type: EmailVariableType.STRING });
		}
	}

	return Array.from(results.values());
}

const PRIMITIVE_TYPES: EmailVariableType[] = [
	EmailVariableType.STRING,
	EmailVariableType.NUMBER,
	EmailVariableType.BOOL,
];

const LIST_ITEM_TYPES: EmailVariableType[] = [
	...PRIMITIVE_TYPES,
	EmailVariableType.OBJECT,
];

function TypeSelect({
	label,
	value,
	options,
	onChange,
}: {
	label: string;
	value: EmailVariableType;
	options: EmailVariableType[];
	onChange: (v: EmailVariableType) => void;
}) {
	const items = useMemo(
		() => options.map((value) => ({ value, label: value })),
		[options],
	);
	return (
		<Box className="w-28 shrink-0">
			<Select
				items={items}
				value={value}
				onValueChange={(v) => onChange(v as EmailVariableType)}
			>
				<SelectTrigger aria-label={label} size="sm" width="fill">
					<SelectValue />
				</SelectTrigger>
				<SelectContent aria-label={`${label} options`}>
					{items.map((item) => (
						<SelectItem key={item.value} value={item.value}>
							{item.label}
						</SelectItem>
					))}
				</SelectContent>
			</Select>
		</Box>
	);
}

function PropertyListEditor({
	properties,
	onChange,
}: {
	properties: EmailVariableSchemaProperty[];
	onChange: (props: EmailVariableSchemaProperty[]) => void;
}) {
	const propertyKeysRef = useRef<string[]>([]);
	propertyKeysRef.current = syncEditorKeys(
		propertyKeysRef.current,
		properties.length,
	);

	function addProp() {
		propertyKeysRef.current = [...propertyKeysRef.current, crypto.randomUUID()];
		onChange([...properties, { name: "", type: EmailVariableType.STRING }]);
	}
	function removeProp(i: number) {
		propertyKeysRef.current = propertyKeysRef.current.filter(
			(_, idx) => idx !== i,
		);
		onChange(properties.filter((_, idx) => idx !== i));
	}
	function updateProp(i: number, patch: Partial<EmailVariableSchemaProperty>) {
		onChange(properties.map((p, idx) => (idx === i ? { ...p, ...patch } : p)));
	}

	return (
		<Stack space="xs">
			{properties.map((p, i) => (
				<Inline key={propertyKeysRef.current[i]} space="xs" wrap={false}>
					<Input
						aria-label={`Property name ${i + 1}`}
						font="mono"
						size="sm"
						value={p.name}
						onChange={(e) => updateProp(i, { name: e.target.value })}
						placeholder="field_name"
					/>
					<TypeSelect
						label={`Type for property ${p.name || i + 1}`}
						value={p.type}
						options={PRIMITIVE_TYPES}
						onChange={(t) => updateProp(i, { type: t })}
					/>
					<IconButton
						variant="ghost"
						tone="critical"
						size="sm"
						type="button"
						icon={Trash2}
						label={`Remove field ${p.name || i + 1}`}
						onClick={() => removeProp(i)}
					/>
				</Inline>
			))}
			<Button variant="ghost" size="sm" type="button" onClick={addProp}>
				+ field
			</Button>
		</Stack>
	);
}

function VariableEditor({
	variables,
	onChange,
}: {
	variables: EmailVariableSchema[];
	onChange: (vars: EmailVariableSchema[]) => void;
}) {
	const variableKeysRef = useRef<string[]>([]);
	variableKeysRef.current = syncEditorKeys(
		variableKeysRef.current,
		variables.length,
	);

	function add() {
		variableKeysRef.current = [...variableKeysRef.current, crypto.randomUUID()];
		onChange([
			...variables,
			{ name: "", type: EmailVariableType.STRING, required: false },
		]);
	}
	function remove(i: number) {
		variableKeysRef.current = variableKeysRef.current.filter(
			(_, idx) => idx !== i,
		);
		onChange(variables.filter((_, idx) => idx !== i));
	}
	function update(i: number, patch: Partial<EmailVariableSchema>) {
		const updated = variables.map((v, idx) =>
			idx === i ? { ...v, ...patch } : v,
		);
		// When switching away from LIST/OBJECT, clear sub-schema.
		if (patch.type && patch.type !== EmailVariableType.LIST) {
			updated[i] = { ...updated[i], items: undefined };
		}
		if (patch.type && patch.type !== EmailVariableType.OBJECT) {
			updated[i] = { ...updated[i], properties: undefined };
		}
		onChange(updated);
	}

	function updateItems(i: number, patch: Partial<EmailVariableSchemaItems>) {
		const v = variables[i];
		onChange(
			variables.map((vv, idx) =>
				idx === i
					? {
							...vv,
							items: {
								...(v.items ?? { type: EmailVariableType.STRING }),
								...patch,
							},
						}
					: vv,
			),
		);
	}

	return (
		<Stack space="sm">
			{variables.map((v, i) => (
				<Box
					key={variableKeysRef.current[i]}
					className="rounded-lg border border-border bg-card"
				>
					{/* Main row */}
					<Box className="flex items-center gap-2 p-2">
						<Input
							aria-label={`Variable name ${i + 1}`}
							font="mono"
							size="sm"
							placeholder="variable_name"
							value={v.name}
							onChange={(e) => update(i, { name: e.target.value })}
						/>
						<TypeSelect
							label={`Type for variable ${v.name || i + 1}`}
							value={v.type}
							options={VARIABLE_TYPES}
							onChange={(t) => update(i, { type: t })}
						/>
						<Label>
							<Checkbox
								checked={v.required ?? false}
								onCheckedChange={(checked) => update(i, { required: checked })}
							/>
							Req
						</Label>
						<IconButton
							variant="ghost"
							tone="critical"
							size="sm"
							type="button"
							icon={Trash2}
							label={`Remove variable ${v.name || i + 1}`}
							onClick={() => remove(i)}
						/>
					</Box>

					{/* LIST sub-schema */}
					{v.type === EmailVariableType.LIST && (
						<Box className="border-t border-border px-3 py-2.5 flex flex-col gap-2 bg-muted/30 rounded-b-lg">
							<Inline space="sm" wrap={false}>
								<Text as="span" size="xs" tone="muted">
									Each item is
								</Text>
								<TypeSelect
									label={`Item type for variable ${v.name || i + 1}`}
									value={v.items?.type ?? EmailVariableType.STRING}
									options={LIST_ITEM_TYPES}
									onChange={(t) =>
										updateItems(i, {
											type: t,
											properties:
												t === EmailVariableType.OBJECT
													? (v.items?.properties ?? [])
													: undefined,
										})
									}
								/>
							</Inline>
							{v.items?.type === EmailVariableType.OBJECT && (
								<Box className="pl-3 border-l-2 border-border">
									<Box
										as="p"
										className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1.5"
									>
										Object fields
									</Box>
									<PropertyListEditor
										properties={v.items.properties ?? []}
										onChange={(props) => updateItems(i, { properties: props })}
									/>
								</Box>
							)}
						</Box>
					)}

					{/* OBJECT sub-schema */}
					{v.type === EmailVariableType.OBJECT && (
						<Box className="border-t border-border px-3 py-2.5 flex flex-col gap-1.5 bg-muted/30 rounded-b-lg">
							<Box
								as="p"
								className="text-[10px] text-muted-foreground uppercase tracking-wider"
							>
								Properties
							</Box>
							<PropertyListEditor
								properties={v.properties ?? []}
								onChange={(props) => update(i, { properties: props })}
							/>
						</Box>
					)}
				</Box>
			))}
			<Button variant="outline" size="sm" onClick={add}>
				+ Add Variable
			</Button>
			{variables.length > 0 && (
				<Text size="xs" tone="muted">
					Use in template:{" "}
					<code className="font-mono bg-muted px-1 rounded">
						{"{{ .variable_name }}"}
					</code>
					{" · "}list:{" "}
					<code className="font-mono bg-muted px-1 rounded">
						{"{{ range .list }}{{ .field }}{{ end }}"}
					</code>
				</Text>
			)}
		</Stack>
	);
}

// ---------------------------------------------------------------------------
// Schema-aware example variable inputs
// ---------------------------------------------------------------------------

function ListObjectInput({
	properties,
	value,
	onChange,
}: {
	properties: EmailVariableSchemaProperty[];
	value: string;
	onChange: (v: string) => void;
}) {
	const rowKeysRef = useRef<string[]>([]);

	function parseRows(): Record<string, string>[] {
		try {
			const parsed = JSON.parse(value);
			if (Array.isArray(parsed))
				return parsed.map((r) => {
					const row: Record<string, string> = {};
					for (const p of properties)
						row[p.name] = r[p.name] != null ? String(r[p.name]) : "";
					return row;
				});
		} catch {}
		return [];
	}

	const rows = parseRows();
	rowKeysRef.current = syncEditorKeys(rowKeysRef.current, rows.length);

	function commit(newRows: Record<string, string>[]) {
		onChange(JSON.stringify(newRows));
	}

	function addRow() {
		rowKeysRef.current = [...rowKeysRef.current, crypto.randomUUID()];
		const empty: Record<string, string> = {};
		for (const p of properties) empty[p.name] = "";
		commit([...rows, empty]);
	}

	function removeRow(i: number) {
		rowKeysRef.current = rowKeysRef.current.filter((_, idx) => idx !== i);
		commit(rows.filter((_, idx) => idx !== i));
	}

	function updateCell(rowIdx: number, col: string, val: string) {
		const newRows = rows.map((r, i) =>
			i === rowIdx ? { ...r, [col]: val } : r,
		);
		commit(newRows);
	}

	if (properties.length === 0) {
		return (
			<Textarea
				font="mono"
				value={value}
				onChange={(e) => onChange(e.target.value)}
				placeholder='[{"field": "value"}, ...]'
				rows={3}
			/>
		);
	}

	return (
		<Stack space="xs">
			<Box className="rounded-md border border-border overflow-hidden">
				{/* Header */}
				<Box
					className="grid bg-muted/60 border-b border-border"
					style={{
						gridTemplateColumns: `repeat(${properties.length}, minmax(0, 1fr)) 32px`,
					}}
				>
					{properties.map((p) => (
						<Box
							key={p.name}
							className="px-2 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider truncate"
						>
							{p.name}
							<Box
								as="span"
								className="ml-1 text-muted-foreground/50 normal-case tracking-normal"
							>
								{p.type.toLowerCase()}
							</Box>
						</Box>
					))}
					<div />
				</Box>
				{/* Rows */}
				{rows.length === 0 ? (
					<Box className="px-2 py-3 text-xs text-muted-foreground text-center">
						No items — click + Add row
					</Box>
				) : (
					rows.map((row, i) => (
						<Box
							key={rowKeysRef.current[i]}
							className={`grid items-center ${i < rows.length - 1 ? "border-b border-border" : ""}`}
							style={{
								gridTemplateColumns: `repeat(${properties.length}, minmax(0, 1fr)) 32px`,
							}}
						>
							{properties.map((p) => (
								<Input
									key={p.name}
									value={row[p.name] ?? ""}
									onChange={(e) => updateCell(i, p.name, e.target.value)}
									variant="ghost"
									size="sm"
									font="mono"
									placeholder="—"
								/>
							))}
							<IconButton
								variant="ghost"
								tone="critical"
								size="sm"
								type="button"
								icon={Trash2}
								label={`Remove row ${i + 1}`}
								onClick={() => removeRow(i)}
							/>
						</Box>
					))
				)}
			</Box>
			<Button variant="ghost" size="sm" type="button" onClick={addRow}>
				+ Add row
			</Button>
		</Stack>
	);
}

function ObjectInput({
	properties,
	value,
	onChange,
}: {
	properties: EmailVariableSchemaProperty[];
	value: string;
	onChange: (v: string) => void;
}) {
	function parseObj(): Record<string, string> {
		try {
			const parsed = JSON.parse(value);
			if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
				const out: Record<string, string> = {};
				for (const p of properties)
					out[p.name] = parsed[p.name] != null ? String(parsed[p.name]) : "";
				return out;
			}
		} catch {}
		const out: Record<string, string> = {};
		for (const p of properties) out[p.name] = "";
		return out;
	}

	const obj = parseObj();

	function updateField(key: string, val: string) {
		const newObj = { ...obj, [key]: val };
		onChange(JSON.stringify(newObj));
	}

	if (properties.length === 0) {
		return (
			<Textarea
				font="mono"
				value={value}
				onChange={(e) => onChange(e.target.value)}
				placeholder='{"field": "value"}'
				rows={3}
			/>
		);
	}

	return (
		<Box className="rounded-md border border-border overflow-hidden">
			{properties.map((p, i) => (
				<Box
					key={p.name}
					className={`grid items-center ${i < properties.length - 1 ? "border-b border-border" : ""}`}
					style={{ gridTemplateColumns: "120px 1fr" }}
				>
					<Box className="px-2 py-1.5 bg-muted/40 border-r border-border">
						<Text as="span" size="xs" tone="muted" font="mono">
							{p.name}
						</Text>
					</Box>
					<Input
						value={obj[p.name] ?? ""}
						onChange={(e) => updateField(p.name, e.target.value)}
						variant="ghost"
						size="sm"
						font="mono"
						placeholder="—"
					/>
				</Box>
			))}
		</Box>
	);
}

function ExampleVarInput({
	name,
	schema,
	value,
	onChange,
}: {
	name: string;
	schema: EmailVariableSchema | undefined;
	value: string;
	onChange: (v: string) => void;
}) {
	const type = schema?.type;
	const isListOfObject =
		type === EmailVariableType.LIST &&
		schema?.items?.type === EmailVariableType.OBJECT;
	const isListPrimitive =
		type === EmailVariableType.LIST &&
		schema?.items?.type !== EmailVariableType.OBJECT;
	const isObjectWithProps =
		type === EmailVariableType.OBJECT && (schema?.properties?.length ?? 0) > 0;
	const isComplexNoSchema =
		(type === EmailVariableType.LIST || type === EmailVariableType.OBJECT) &&
		!isListOfObject &&
		!isObjectWithProps;

	function typeLabel() {
		if (!type) return null;
		if (type === EmailVariableType.LIST && schema?.items) {
			const itemType = schema.items.type;
			return `LIST · ${itemType}`;
		}
		return type;
	}

	return (
		<Stack space="xs">
			<Inline space="xs" wrap={false}>
				<Text as="span" size="xs" font="mono">
					{name}
				</Text>
				{typeLabel() && (
					<Box
						as="span"
						className="text-[10px] text-muted-foreground/70 bg-muted px-1.5 rounded"
					>
						{typeLabel()}
					</Box>
				)}
			</Inline>
			{isListOfObject ? (
				<ListObjectInput
					properties={schema?.items?.properties ?? []}
					value={value}
					onChange={onChange}
				/>
			) : isObjectWithProps ? (
				<ObjectInput
					properties={schema?.properties ?? []}
					value={value}
					onChange={onChange}
				/>
			) : isListPrimitive || isComplexNoSchema ? (
				<Textarea
					font="mono"
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder={
						type === EmailVariableType.LIST
							? '["value1", "value2"]'
							: '{"key": "value"}'
					}
					rows={2}
				/>
			) : (
				<Input
					font="mono"
					size="sm"
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder={`value for .${name}`}
				/>
			)}
		</Stack>
	);
}

function ExampleManager({
	productId,
	templateId,
	detectedVarNames,
	variables,
	activeVarValues,
	onActiveChange,
}: {
	productId: string;
	templateId: string;
	detectedVarNames: string[];
	variables: EmailVariableSchema[];
	activeVarValues: Record<string, string>;
	onActiveChange: (vals: Record<string, string>) => void;
}) {
	const queryClient = useQueryClient();
	const queryOptions = {
		path: { product_id: productId, email_template_id: templateId },
	};

	const { data } = useQuery(getEmailTemplateExamplesOptions(queryOptions));

	const [examples, setExamples] = useState<TemplateExample[]>([]);
	const [activeId, setActiveId] = useState<string | null>(null);
	const [saveStatus, setSaveStatus] = useState<
		"idle" | "saving" | "saved" | "error"
	>("idle");
	const [rawMode, setRawMode] = useState(false);
	const [rawText, setRawText] = useState("");
	const [rawError, setRawError] = useState<string | null>(null);
	const initialized = useRef(false);
	const schemaNames = useMemo(() => variables.map((v) => v.name), [variables]);

	useEffect(() => {
		if (data && !initialized.current) {
			setExamples(data.examples ?? []);
			if ((data.examples ?? []).length > 0) {
				const first = data.examples[0];
				setActiveId(first.id);
				onActiveChange(stringifyExampleValues(first.variables ?? {}));
			}
			initialized.current = true;
		}
	}, [data, onActiveChange]);

	const { mutate: saveExamples } = useMutation({
		...saveEmailTemplateExamplesMutation(),
		onMutate: () => setSaveStatus("saving"),
		onSuccess: () => {
			setSaveStatus("saved");
			queryClient.invalidateQueries({ queryKey: ["getEmailTemplateExamples"] });
			setTimeout(() => setSaveStatus("idle"), 2000);
		},
		onError: () => setSaveStatus("error"),
	});

	function handleSelectExample(id: string) {
		setActiveId(id);
		setRawMode(false);
		setRawError(null);
		const ex = examples.find((e) => e.id === id);
		onActiveChange(stringifyExampleValues(ex?.variables ?? {}));
	}

	function handleNewExample() {
		const id = `ex-${Date.now().toString(36)}`;
		const prefilled: Record<string, unknown> = {};
		for (const name of detectedVarNames) prefilled[name] = "";
		const newEx: TemplateExample = {
			id,
			name: "New Example",
			variables: prefilled,
		};
		const updated = [...examples, newEx];
		setExamples(updated);
		setActiveId(id);
		setRawMode(false);
		setRawError(null);
		onActiveChange(Object.fromEntries(detectedVarNames.map((n) => [n, ""])));
	}

	function handleDeleteExample(id: string) {
		const updated = examples.filter((e) => e.id !== id);
		setExamples(updated);
		if (activeId === id) {
			const next = updated[0] ?? null;
			setActiveId(next?.id ?? null);
			setRawMode(false);
			setRawError(null);
			onActiveChange(stringifyExampleValues(next?.variables ?? {}));
		}
	}

	function handleNameChange(id: string, name: string) {
		setExamples((prev) => prev.map((e) => (e.id === id ? { ...e, name } : e)));
	}

	function handleVarChange(varName: string, value: string) {
		setExamples((prev) =>
			prev.map((e) => {
				if (e.id !== activeId) return e;
				return { ...e, variables: { ...e.variables, [varName]: value } };
			}),
		);
		onActiveChange({ ...activeVarValues, [varName]: value });
	}

	function handleRawChange(text: string) {
		setRawText(text);
		try {
			const parsed = JSON.parse(text);
			if (
				typeof parsed !== "object" ||
				Array.isArray(parsed) ||
				parsed === null
			) {
				setRawError("Must be a JSON object { ... }");
				return;
			}
			setRawError(null);
			setExamples((prev) =>
				prev.map((e) => (e.id === activeId ? { ...e, variables: parsed } : e)),
			);
			onActiveChange(stringifyExampleValues(parsed));
		} catch (err: unknown) {
			setRawError(err instanceof Error ? err.message : "Invalid JSON");
		}
	}

	const activeExample = examples.find((e) => e.id === activeId) ?? null;
	// Union: detected from template + any extra keys already saved in the example.
	const savedKeys = Object.keys(activeExample?.variables ?? {});
	const varNames = Array.from(new Set([...detectedVarNames, ...savedKeys]));

	function toggleRawMode(on: boolean) {
		if (on && activeExample) {
			setRawText(JSON.stringify(activeExample.variables ?? {}, null, 2));
			setRawError(null);
		}
		setRawMode(on);
	}

	function handleSave() {
		saveExamples({
			path: queryOptions.path,
			body: { examples },
		});
	}

	useEffect(() => {
		if (!initialized.current) return;
		let activePatched: Record<string, unknown> | null = null;
		setExamples((prev) => {
			let didPatch = false;
			const nextExamples = prev.map((ex) => {
				const missing = schemaNames.filter((n) => !(n in (ex.variables ?? {})));
				if (missing.length === 0) return ex;
				didPatch = true;
				const patched = { ...ex.variables };
				for (const n of missing) patched[n] = "";
				if (ex.id === activeId) activePatched = patched;
				return { ...ex, variables: patched };
			});

			return didPatch ? nextExamples : prev;
		});
		if (activePatched) onActiveChange(stringifyExampleValues(activePatched));
	}, [activeId, onActiveChange, schemaNames]);

	return (
		<Stack space="lg">
			{/* Header */}
			<Spread space="md">
				<Inline space="sm" wrap={false}>
					<LayoutTemplate className="size-4 text-muted-foreground" />
					<Text tone="muted">
						Named variable sets for preview and test sends
					</Text>
				</Inline>
				<Button
					icon={Plus}
					variant="outline"
					size="sm"
					onClick={handleNewExample}
				>
					New Example
				</Button>
			</Spread>

			{examples.length === 0 ? (
				<Box className="border rounded-lg p-8 text-center">
					<FileText className="size-8 text-muted-foreground mx-auto mb-3" />
					<Box as="p" className="text-sm text-muted-foreground mb-1">
						No examples yet
					</Box>
					<Text size="xs" tone="muted">
						Create an example to pre-fill variables for preview and test sends
					</Text>
				</Box>
			) : (
				<Box className="flex flex-col gap-3 sm:flex-row">
					{/* Example list sidebar */}
					<Box className="w-full sm:w-44 sm:shrink-0">
						<Box className="text-xs font-medium text-muted-foreground mb-1.5 px-1">
							Examples ({examples.length})
						</Box>
						<Stack space="xxs">
							{examples.map((ex) => (
								<Button
									variant={activeId === ex.id ? "soft" : "ghost"}
									tone={activeId === ex.id ? "brand" : "neutral"}
									aria-pressed={activeId === ex.id}
									size="sm"
									type="button"
									key={ex.id}
									onClick={() => handleSelectExample(ex.id)}
								>
									<FileText className="size-3.5 shrink-0 opacity-70" />
									<Text as="span" truncate>
										{ex.name || "Unnamed"}
									</Text>
								</Button>
							))}
						</Stack>
					</Box>

					{/* Active example editor */}
					{activeExample && (
						<Box className="flex-1 bg-muted/30 rounded-lg border p-4 flex flex-col gap-3">
							{/* Editor toolbar */}
							<Inline space="sm" wrap={false}>
								<Input
									size="sm"
									value={activeExample.name}
									onChange={(e) =>
										handleNameChange(activeExample.id, e.target.value)
									}
									placeholder="Example name"
								/>
								<Button
									icon={Code2}
									variant="ghost"
									size="sm"
									onClick={() => toggleRawMode(!rawMode)}
								>
									{rawMode ? "Form" : "Raw"}
								</Button>
								<IconButton
									tone="critical"
									icon={Trash2}
									label="Delete example"
									variant="ghost"
									size="sm"
									onClick={() => handleDeleteExample(activeExample.id)}
								/>
							</Inline>

							{/* Editor content */}
							{rawMode ? (
								<Stack space="xs">
									<Textarea
										font="mono"
										value={rawText}
										onChange={(e) => handleRawChange(e.target.value)}
										spellCheck={false}
										placeholder='{"userName": "Alice", "orderTotal": 99.99}'
									/>
									{rawError && (
										<Box className="flex items-center gap-1.5 text-xs text-destructive font-mono">
											<AlertCircle className="size-3" />
											{rawError}
										</Box>
									)}
								</Stack>
							) : varNames.length === 0 ? (
								<Box className="py-6 text-center">
									<Text size="xs" tone="muted">
										No variables detected yet. Add{" "}
										<code className="font-mono bg-muted px-1 rounded">
											{"{{ .varName }}"}
										</code>{" "}
										to your template.
									</Text>
								</Box>
							) : (
								<Stack space="md">
									{varNames.map((name) => {
										const schema = variables.find((v) => v.name === name);
										return (
											<ExampleVarInput
												key={name}
												name={name}
												schema={schema}
												value={activeVarValues[name] ?? ""}
												onChange={(v) => handleVarChange(name, v)}
											/>
										);
									})}
								</Stack>
							)}
						</Box>
					)}
				</Box>
			)}

			{/* Footer actions */}
			<Inline space="sm" wrap={false}>
				<Button
					variant="solid"
					tone="brand"
					icon={Save}
					size="sm"
					onClick={handleSave}
					disabled={saveStatus === "saving"}
				>
					{saveStatus === "saving" ? "Saving…" : "Save Examples"}
				</Button>
				{saveStatus === "saved" && (
					<Box
						as="span"
						className="text-xs text-success flex items-center gap-1"
					>
						<Check className="size-3" />
						Saved
					</Box>
				)}
				{saveStatus === "error" && (
					<Box
						as="span"
						className="text-xs text-destructive flex items-center gap-1"
					>
						<AlertCircle className="size-3" />
						Save failed
					</Box>
				)}
			</Inline>
		</Stack>
	);
}

function TestSendDialog({
	productId,
	templateId,
	variables,
	activeVarValues,
	hasEmailIntegration,
}: {
	productId: string;
	templateId: string;
	variables: EmailVariableSchema[];
	activeVarValues: Record<string, string>;
	hasEmailIntegration: boolean;
}) {
	const [open, setOpen] = useState(false);
	const [toAddress, setToAddress] = useState("");
	const [varValues, setVarValues] = useState<Record<string, string>>({});
	const [result, setResult] = useState<{ status: string; id: string } | null>(
		null,
	);
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		setVarValues(activeVarValues);
	}, [activeVarValues]);

	const { mutate, isPending } = useMutation({
		...sendEmailMutation(),
		onSuccess: (data) => {
			setResult({ status: data.status, id: data.id });
			setError(null);
		},
		onError: (err) => {
			setError(getApiErrorMessage(err, "Send failed"));
			setResult(null);
		},
	});

	function handleSend(e: React.FormEvent) {
		e.preventDefault();
		setError(null);
		setResult(null);

		// For any field left blank, fall back to the loaded example values
		// (the first example is pre-loaded when one exists) before validating,
		// so a required variable only errors when neither the form nor an
		// example provides a value.
		const effective: Record<string, string> = {};
		for (const v of variables) {
			const entered = varValues[v.name] ?? "";
			effective[v.name] =
				entered.trim() !== "" ? entered : (activeVarValues[v.name] ?? "");
		}

		const missing = variables
			.filter((v) => v.required && (effective[v.name] ?? "").trim() === "")
			.map((v) => v.name);
		if (missing.length > 0) {
			setError(`Fill required variable(s): ${missing.join(", ")}`);
			return;
		}

		setVarValues(effective);
		mutate({
			path: { product_id: productId },
			body: {
				template_id: templateId,
				to_address: toAddress,
				variables: buildVarsPayload(effective, variables),
				use_draft: true,
			},
		});
	}

	return (
		<Dialog
			open={open}
			onOpenChange={(o) => {
				setOpen(o);
				if (!o) {
					setResult(null);
					setError(null);
				}
			}}
		>
			<Tooltip>
				<TooltipTrigger
					render={<span tabIndex={!hasEmailIntegration ? 0 : undefined} />}
				>
					<Button
						variant="outline"
						size="sm"
						disabled={!hasEmailIntegration}
						onClick={() => hasEmailIntegration && setOpen(true)}
					>
						Send Test
					</Button>
				</TooltipTrigger>
				{!hasEmailIntegration && (
					<TooltipContent side="bottom">
						No active SMTP integration. Go to Integrations → SMTP Email to
						configure one.
					</TooltipContent>
				)}
			</Tooltip>
			<DialogContent size="lg">
				<DialogHeader>
					<DialogTitle>Send Test Email</DialogTitle>
				</DialogHeader>
				<Stack onSubmit={handleSend} space="lg" as="form">
					<Stack space="xs">
						<Label htmlFor="to">Recipient</Label>
						<Input
							id="to"
							type="email"
							required
							value={toAddress}
							onChange={(e) => setToAddress(e.target.value)}
							placeholder="you@example.com"
						/>
					</Stack>
					{variables.length > 0 && (
						<Stack space="sm">
							<Label>Variables</Label>
							{variables.map((v) => (
								<Inline key={v.name} space="sm" wrap={false}>
									<Box
										as="span"
										className="font-mono text-xs text-muted-foreground w-24 shrink-0"
									>
										{v.name}
									</Box>
									<Input
										placeholder={v.required ? "required" : "optional"}
										value={varValues[v.name] ?? ""}
										onChange={(e) =>
											setVarValues((prev) => ({
												...prev,
												[v.name]: e.target.value,
											}))
										}
									/>
								</Inline>
							))}
						</Stack>
					)}
					{error && <Text tone="critical">{error}</Text>}
					{result && (
						<Text tone="success">
							Sent — status: <strong>{result.status}</strong> (ID: {result.id})
						</Text>
					)}
					<Inline space="md" align="end" wrap={false}>
						<Button
							variant="solid"
							tone="brand"
							type="submit"
							disabled={isPending}
						>
							{isPending ? "Sending…" : "Send"}
						</Button>
					</Inline>
				</Stack>
			</DialogContent>
		</Dialog>
	);
}

// Parse string values to typed payloads.
// LIST/OBJECT schema types are always parsed. Values that look like JSON
// arrays/objects are also parsed regardless of schema — handles cases where
// the schema type hasn't been set yet (e.g. steps before "Push to Variables").
function buildVarsPayload(
	values: Record<string, string>,
	schemas: EmailVariableSchema[],
): Record<string, unknown> {
	const vars: Record<string, unknown> = {};
	for (const [k, v] of Object.entries(values)) {
		if (v === "") continue;
		const schema = schemas.find((s) => s.name === k);
		const isComplex =
			schema?.type === EmailVariableType.LIST ||
			schema?.type === EmailVariableType.OBJECT;
		const trimmed = v.trimStart();
		const looksLikeJson = trimmed.startsWith("[") || trimmed.startsWith("{");
		if (isComplex || looksLikeJson) {
			try {
				vars[k] = JSON.parse(v);
			} catch {
				vars[k] = v;
			}
		} else {
			vars[k] = v;
		}
	}
	return vars;
}

function PreviewPane({
	productId,
	templateId,
	variables,
	draftVersion,
	activeVarValues,
}: {
	productId: string;
	templateId: string;
	variables: EmailVariableSchema[];
	draftVersion: EmailTemplateVersionResponse | null;
	activeVarValues: Record<string, string>;
}) {
	const [preview, setPreview] = useState<{
		subject: string;
		body_html: string;
		warnings: string[];
	} | null>(null);
	const [previewError, setPreviewError] = useState<string | null>(null);
	const debounceTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
	const previewRequest = useMemo(
		() => ({
			body: {
				variables: buildVarsPayload(activeVarValues, variables),
				use_published: false as const,
			},
			draftSnapshotKey: draftVersion?.updated_at ?? null,
		}),
		[activeVarValues, draftVersion?.updated_at, variables],
	);

	const { mutate: runPreview, isPending: isPreviewing } = useMutation({
		...previewEmailTemplateMutation(),
		onSuccess: (data) => {
			setPreview({
				subject: data.subject,
				body_html: data.body_html,
				warnings: data.warnings ?? [],
			});
			setPreviewError(null);
		},
		onError: (err) => {
			const msg = getApiErrorMessage(err, "Preview failed");
			const code = getApiErrorCode(err);
			setPreviewError(code ? `[${code}] ${msg}` : msg);
			setPreview(null);
		},
	});

	const runPreviewRef = useRef(runPreview);
	runPreviewRef.current = runPreview;

	// Auto-refresh: triggers when draft is saved (draftVersion changes) or active var values change.
	useEffect(() => {
		if (debounceTimer.current) clearTimeout(debounceTimer.current);
		debounceTimer.current = setTimeout(() => {
			runPreviewRef.current({
				path: { product_id: productId, email_template_id: templateId },
				body: previewRequest.body,
			});
		}, 500);
		return () => {
			if (debounceTimer.current) clearTimeout(debounceTimer.current);
		};
	}, [previewRequest, productId, templateId]);

	function handlePreview() {
		if (debounceTimer.current) clearTimeout(debounceTimer.current);
		runPreview({
			path: { product_id: productId, email_template_id: templateId },
			body: previewRequest.body,
		});
	}

	const previewWarningKeys = (() => {
		const warningKeyCounts = new Map<string, number>();

		return (
			preview?.warnings.map((warning) => {
				const nextCount = warningKeyCounts.get(warning) ?? 0;
				warningKeyCounts.set(warning, nextCount + 1);
				return `${warning}-${nextCount}`;
			}) ?? []
		);
	})();

	return (
		<Box className="flex flex-col h-full gap-4">
			<Spread space="md">
				<Button
					onClick={handlePreview}
					disabled={isPreviewing}
					size="sm"
					variant="outline"
				>
					{isPreviewing ? "Rendering…" : "Refresh Preview"}
				</Button>
				{isPreviewing && (
					<Box as="span" className="text-xs text-muted-foreground">
						Rendering…
					</Box>
				)}
			</Spread>
			{previewError && (
				<Box className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2">
					<Box
						as="p"
						className="text-xs text-destructive font-mono whitespace-pre-wrap"
					>
						{previewError}
					</Box>
				</Box>
			)}
			{preview ? (
				<Box className="flex flex-col gap-2 flex-1 min-h-0">
					<Box className="text-sm">
						<Text as="span" weight="medium">
							Subject:{" "}
						</Text>
						<Text as="span" tone="muted">
							{preview.subject}
						</Text>
					</Box>
					{preview.warnings.length > 0 && (
						<Box className="rounded-md border border-warning/40 bg-warning/10 px-3 py-2 flex flex-col gap-1">
							<Box as="p" className="text-xs font-medium text-warning">
								{preview.warnings.length} render warning
								{preview.warnings.length > 1 ? "s" : ""}
							</Box>
							<Stack space="xxs" as="ul">
								{preview.warnings.map((w, i) => (
									<Box
										as="li"
										key={previewWarningKeys[i] ?? `${w}-${i}`}
										className="text-xs text-warning font-mono"
									>
										· {w}
									</Box>
								))}
							</Stack>
						</Box>
					)}
					<Box className="flex-1 min-h-0 border rounded-md overflow-hidden">
						<iframe
							title="Email Preview"
							srcDoc={preview.body_html}
							className="w-full h-full"
							sandbox="allow-same-origin"
						/>
					</Box>
				</Box>
			) : draftVersion ? (
				<Box className="flex-1 min-h-0 border rounded-md overflow-hidden">
					<iframe
						title="Email Preview"
						srcDoc={draftVersion.body_html}
						className="w-full h-full"
						sandbox="allow-same-origin"
					/>
				</Box>
			) : (
				<Box className="flex-1 flex items-center justify-center text-sm text-muted-foreground border rounded-md">
					Click "Refresh Preview" to render
				</Box>
			)}
		</Box>
	);
}

export function EmailTemplateBuilder({
	productId,
	templateId,
}: {
	productId: string;
	templateId: string;
}) {
	const queryClient = useQueryClient();
	const isMobile = useIsMobile();

	const { data: template, isLoading: templateLoading } = useQuery(
		getEmailTemplateOptions({
			path: { product_id: productId, email_template_id: templateId },
		}),
	);

	const { data: draft, isLoading: draftLoading } = useQuery(
		getEmailTemplateDraftOptions({
			path: { product_id: productId, email_template_id: templateId },
		}),
	);

	const { data: integrations } = useQuery(
		listIntegrationInstancesOptions({ path: { product_id: productId } }),
	);

	const hasEmailIntegration = (integrations?.items ?? []).some(
		(i) => i.provider_type === IntegrationProviderType.SMTP && i.is_enabled,
	);

	const [name, setName] = useState("");
	const [subject, setSubject] = useState("");
	const [bodyHtml, setBodyHtml] = useState("");
	const [variables, setVariables] = useState<EmailVariableSchema[]>([]);
	const [activeVarValues, setActiveVarValues] = useState<
		Record<string, string>
	>({});
	const [saveState, setSaveState] = useState<
		"idle" | "saving" | "saved" | "error"
	>("idle");
	const [publishError, setPublishError] = useState<string | null>(null);

	const initialized = useRef(false);
	const metaInitialized = useRef(false);

	useEffect(() => {
		if (template && !metaInitialized.current) {
			setName(template.name ?? "");
			metaInitialized.current = true;
		}
	}, [template]);

	useEffect(() => {
		if (draft && !initialized.current) {
			setSubject(draft.subject ?? "");
			setBodyHtml(draft.body_html ?? "");
			setVariables(draft.variables ?? []);
			initialized.current = true;
		}
	}, [draft]);

	const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
	const nameTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

	const { mutate: saveMeta } = useMutation({
		...updateEmailTemplateMutation(),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey: ["getEmailTemplate"] });
			queryClient.invalidateQueries({ queryKey: ["listEmailTemplates"] });
		},
	});

	function handleNameChange(val: string) {
		setName(val);
		if (nameTimer.current) clearTimeout(nameTimer.current);
		nameTimer.current = setTimeout(() => {
			saveMeta({
				path: { product_id: productId, email_template_id: templateId },
				body: { name: val },
			});
		}, 800);
	}

	const { mutate: saveDraft } = useMutation({
		...updateEmailTemplateDraftMutation(),
		onMutate: () => setSaveState("saving"),
		onSuccess: () => {
			setSaveState("saved");
			queryClient.invalidateQueries({ queryKey: ["getEmailTemplateDraft"] });
			setTimeout(() => setSaveState("idle"), 2000);
		},
		onError: () => setSaveState("error"),
	});

	const scheduleSave = useCallback(
		(s: string, h: string, v: EmailVariableSchema[]) => {
			if (saveTimer.current) clearTimeout(saveTimer.current);
			saveTimer.current = setTimeout(() => {
				saveDraft({
					path: { product_id: productId, email_template_id: templateId },
					body: { subject: s, body_html: h, variables: v },
				});
			}, 800);
		},
		[productId, templateId, saveDraft],
	);

	function handleSubjectChange(val: string) {
		setSubject(val);
		scheduleSave(val, bodyHtml, variables);
	}
	function handleBodyHtmlChange(val: string) {
		setBodyHtml(val);
		scheduleSave(subject, val, variables);
	}
	function handleVariablesChange(v: EmailVariableSchema[]) {
		setVariables(v);
		scheduleSave(subject, bodyHtml, v);
	}

	const { mutate: publish, isPending: isPublishing } = useMutation({
		...publishEmailTemplateMutation(),
		onSuccess: () => {
			setPublishError(null);
			queryClient.invalidateQueries({ queryKey: ["getEmailTemplate"] });
			queryClient.invalidateQueries({ queryKey: ["listEmailTemplates"] });
		},
		onError: (err) => {
			setPublishError(getApiErrorMessage(err, "Publish failed"));
		},
	});

	const detectedSchemas = detectVariableSchemas(subject, bodyHtml);
	const detectedVarNames = detectedSchemas.map((s) => s.name);

	if (templateLoading || draftLoading) {
		return (
			<Box className="p-8 text-muted-foreground text-sm">Loading template…</Box>
		);
	}

	if (!template) {
		return (
			<Box className="p-8 text-destructive text-sm">Template not found.</Box>
		);
	}

	return (
		<Box className="flex flex-col h-full gap-0">
			{/* Header */}
			<Box className="flex flex-col gap-3 px-4 py-3 border-b shrink-0 sm:flex-row sm:items-center sm:justify-between sm:px-6">
				<Box className="flex flex-wrap items-center gap-x-3 gap-y-1 min-w-0">
					<TextLink href={ROUTE_PATHS.EMAIL_TEMPLATES}>← Templates</TextLink>
					<Separator orientation="vertical" length="short" />
					<Input
						value={name}
						onChange={(e) => handleNameChange(e.target.value)}
						variant="ghost"
						size="sm"
						aria-label="Template name"
						placeholder="Template name"
					/>
					<Text as="span" size="xs" tone="muted" font="mono" truncate>
						/{template.slug}
					</Text>
					{template.published_version_id ? (
						<StatusBadge tone="info">Published</StatusBadge>
					) : (
						<StatusBadge tone="warning">Draft only</StatusBadge>
					)}
				</Box>
				<Box className="flex items-center gap-2 sm:justify-end">
					<Text as="span" size="xs" tone="muted">
						{saveState === "saving" && "Saving…"}
						{saveState === "saved" && "Saved"}
						{saveState === "error" && "Save failed"}
					</Text>
					<TestSendDialog
						productId={productId}
						templateId={templateId}
						variables={variables}
						activeVarValues={activeVarValues}
						hasEmailIntegration={hasEmailIntegration}
					/>
					<Button
						variant="solid"
						tone="brand"
						size="sm"
						onClick={() =>
							publish({
								path: { product_id: productId, email_template_id: templateId },
							})
						}
						disabled={isPublishing}
					>
						{isPublishing ? "Publishing…" : "Publish"}
					</Button>
				</Box>
			</Box>
			{publishError && (
				<Box className="px-6 py-2 text-sm text-destructive bg-destructive/10 border-b">
					{publishError}
				</Box>
			)}

			{/* Body — split editor / preview on desktop, tabbed on mobile */}
			<Box className="flex flex-1 min-h-0">
				{/* Left: editor */}
				<Box
					className={`grid h-full min-h-0 grid-cols-1 p-4 ${
						isMobile ? "w-full" : "w-1/2 border-r"
					}`}
				>
					<Tabs defaultValue="content">
						<TabsList>
							<TabsTrigger value="content">Content</TabsTrigger>
							<TabsTrigger value="variables">
								Variables ({variables.length})
							</TabsTrigger>
							<TabsTrigger value="examples">Examples</TabsTrigger>
							{isMobile && <TabsTrigger value="preview">Preview</TabsTrigger>}
						</TabsList>

						<TabsContent value="content">
							<Box className="flex h-full min-h-0 flex-col gap-4">
								<Box className="flex flex-col gap-1 shrink-0">
									<Label htmlFor="subject">Subject</Label>
									<Input
										font="mono"
										id="subject"
										value={subject}
										onChange={(e) => handleSubjectChange(e.target.value)}
										placeholder="Hello {{ .name }}"
									/>
								</Box>
								<Box className="flex flex-col flex-1 min-h-0 gap-1">
									<Box className="flex items-center justify-between shrink-0">
										<Label>HTML Body</Label>
										<CopyButton
											value={bodyHtml}
											label="Copy HTML"
											copiedLabel="Copied"
											type="button"
											variant="ghost"
											size="sm"
											disabled={!bodyHtml}
										/>
									</Box>
									<Box
										className="flex-1 min-h-0 border rounded-2xl overflow-hidden"
										as="section"
										aria-label="HTML editor"
									>
										<Editor
											language="html"
											value={bodyHtml}
											onChange={(val) => handleBodyHtmlChange(val ?? "")}
											theme="vs-light"
											height="100%"
											options={{
												minimap: { enabled: false },
												fontSize: 13,
												lineNumbers: "on",
												wordWrap: "on",
												scrollBeyondLastLine: false,
												tabSize: 2,
												automaticLayout: true,
											}}
										/>
									</Box>
								</Box>
							</Box>
						</TabsContent>

						<TabsContent value="variables">
							<Box className="h-full min-h-0 overflow-y-auto">
								<Stack space="lg">
									<Text tone="muted">
										Define the variables your template expects. These appear in
										the preview and test-send panels.
									</Text>
									{(() => {
										const schemaNames = new Set(variables.map((v) => v.name));
										const unpushed = detectedSchemas.filter(
											(s) => !schemaNames.has(s.name),
										);
										if (unpushed.length === 0) return null;

										function pushOne(s: DetectedSchema) {
											handleVariablesChange([
												...variables,
												{
													name: s.name,
													type: s.type,
													required: false,
													items: s.items,
												},
											]);
										}

										function pushAll() {
											handleVariablesChange([
												...variables,
												...unpushed.map((s) => ({
													name: s.name,
													type: s.type,
													required: false,
													items: s.items,
												})),
											]);
										}

										return (
											<Box className="border rounded-md p-3 flex flex-col gap-2 bg-muted/40">
												<Spread space="md">
													<Box
														as="p"
														className="text-xs font-medium text-muted-foreground uppercase tracking-wide"
													>
														Detected in template
													</Box>
													<Button variant="outline" size="sm" onClick={pushAll}>
														Push all ({unpushed.length})
													</Button>
												</Spread>
												<Stack space="xs">
													{unpushed.map((s) => {
														const isListObj =
															s.type === EmailVariableType.LIST &&
															s.items?.type === EmailVariableType.OBJECT;
														const fields = isListObj
															? (s.items?.properties ?? [])
															: [];
														return (
															<Button
																width="fill"
																variant="ghost"
																size="sm"
																type="button"
																key={s.name}
																onClick={() => pushOne(s)}
															>
																<Box
																	as="span"
																	className="font-mono text-xs text-foreground group-hover:text-foreground mt-0.5"
																>
																	+ {s.name}
																</Box>
																<Box className="flex flex-wrap items-center gap-1 mt-0.5">
																	<Box
																		as="span"
																		className={`text-[10px] px-1.5 py-0 rounded font-medium ${
																			s.type === EmailVariableType.LIST
																				? "bg-accent text-accent-foreground border border-border"
																				: s.type === EmailVariableType.OBJECT
																					? "bg-secondary text-secondary-foreground border border-border"
																					: "bg-muted text-muted-foreground border border-border"
																		}`}
																	>
																		{s.type === EmailVariableType.LIST &&
																		s.items
																			? `LIST · ${s.items.type}`
																			: s.type}
																	</Box>
																	{fields.map((f) => (
																		<Box
																			as="span"
																			key={f.name}
																			className="text-[10px] font-mono text-muted-foreground/70 bg-muted px-1 rounded"
																		>
																			{f.name}
																		</Box>
																	))}
																</Box>
															</Button>
														);
													})}
												</Stack>
											</Box>
										);
									})()}
									<VariableEditor
										variables={variables}
										onChange={handleVariablesChange}
									/>
								</Stack>
							</Box>
						</TabsContent>

						<TabsContent value="examples">
							<Box className="h-full min-h-0 overflow-y-auto">
								<ExampleManager
									productId={productId}
									templateId={templateId}
									detectedVarNames={detectedVarNames}
									variables={variables}
									activeVarValues={activeVarValues}
									onActiveChange={setActiveVarValues}
								/>
							</Box>
						</TabsContent>

						{/* Mobile-only preview tab (desktop renders the split panel below) */}
						{isMobile && (
							<TabsContent value="preview">
								<Box className="flex h-full min-h-0 flex-col gap-4">
									<PreviewPane
										productId={productId}
										templateId={templateId}
										variables={variables}
										draftVersion={draft ?? null}
										activeVarValues={activeVarValues}
									/>
								</Box>
							</TabsContent>
						)}
					</Tabs>
				</Box>

				{/* Right: preview (desktop split view) */}
				{!isMobile && (
					<Box className="flex flex-col w-1/2 p-4 min-h-0">
						<Box
							as="p"
							className="text-xs text-muted-foreground uppercase tracking-wide font-medium mb-3"
						>
							Live Preview
						</Box>
						<PreviewPane
							productId={productId}
							templateId={templateId}
							variables={variables}
							draftVersion={draft ?? null}
							activeVarValues={activeVarValues}
						/>
					</Box>
				)}
			</Box>
		</Box>
	);
}
