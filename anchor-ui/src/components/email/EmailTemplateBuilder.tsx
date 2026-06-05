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
import { Button } from "@/components/ui/button";
import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Separator } from "@/components/ui/separator";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Textarea } from "@/components/ui/textarea";
import {
	Tooltip,
	TooltipContent,
	TooltipProvider,
	TooltipTrigger,
} from "@/components/ui/tooltip";
import { ROUTE_PATHS } from "@/routes/routePaths";
import Editor from "@monaco-editor/react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
	AlertCircle,
	Check,
	Code2,
	FileText,
	LayoutTemplate,
	Plus,
	Save,
	Trash2,
} from "lucide-react";
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

function TypeSelect({
	value,
	options,
	onChange,
	className,
}: {
	value: EmailVariableType;
	options: EmailVariableType[];
	onChange: (v: EmailVariableType) => void;
	className?: string;
}) {
	return (
		<Select
			value={value}
			onValueChange={(v) => onChange(v as EmailVariableType)}
		>
			<SelectTrigger className={`text-xs h-7 ${className ?? ""}`}>
				<SelectValue />
			</SelectTrigger>
			<SelectContent>
				{options.map((t) => (
					<SelectItem key={t} value={t} className="text-xs">
						{t}
					</SelectItem>
				))}
			</SelectContent>
		</Select>
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
		<div className="flex flex-col gap-1.5">
			{properties.map((p, i) => (
				<div
					key={propertyKeysRef.current[i]}
					className="flex items-center gap-1.5"
				>
					<Input
						value={p.name}
						onChange={(e) => updateProp(i, { name: e.target.value })}
						placeholder="field_name"
						className="font-mono text-xs h-7 flex-1"
					/>
					<TypeSelect
						value={p.type}
						options={PRIMITIVE_TYPES}
						onChange={(t) => updateProp(i, { type: t })}
						className="w-24"
					/>
					<button
						type="button"
						onClick={() => removeProp(i)}
						className="text-muted-foreground hover:text-destructive text-xs px-1"
					>
						✕
					</button>
				</div>
			))}
			<button
				type="button"
				onClick={addProp}
				className="text-xs text-muted-foreground hover:text-foreground border border-dashed border-muted-foreground/40 hover:border-foreground/40 rounded px-2 py-0.5 transition-colors"
			>
				+ field
			</button>
		</div>
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
		<div className="flex flex-col gap-2">
			{variables.map((v, i) => (
				<div
					key={variableKeysRef.current[i]}
					className="rounded-lg border border-border bg-card"
				>
					{/* Main row */}
					<div className="flex items-center gap-2 p-2">
						<Input
							placeholder="variable_name"
							value={v.name}
							onChange={(e) => update(i, { name: e.target.value })}
							className="font-mono text-sm h-7 flex-1"
						/>
						<TypeSelect
							value={v.type}
							options={VARIABLE_TYPES}
							onChange={(t) => update(i, { type: t })}
							className="w-28"
						/>
						<label className="flex items-center gap-1 text-xs text-muted-foreground cursor-pointer whitespace-nowrap">
							<input
								type="checkbox"
								checked={v.required ?? false}
								onChange={(e) => update(i, { required: e.target.checked })}
								className="rounded"
							/>
							Req
						</label>
						<button
							type="button"
							onClick={() => remove(i)}
							className="text-muted-foreground hover:text-destructive text-xs px-1"
						>
							✕
						</button>
					</div>

					{/* LIST sub-schema */}
					{v.type === EmailVariableType.LIST && (
						<div className="border-t border-border px-3 py-2.5 flex flex-col gap-2 bg-muted/30 rounded-b-lg">
							<div className="flex items-center gap-2">
								<span className="text-xs text-muted-foreground">
									Each item is
								</span>
								<TypeSelect
									value={v.items?.type ?? EmailVariableType.STRING}
									options={[...PRIMITIVE_TYPES, EmailVariableType.OBJECT]}
									onChange={(t) =>
										updateItems(i, {
											type: t,
											properties:
												t === EmailVariableType.OBJECT
													? (v.items?.properties ?? [])
													: undefined,
										})
									}
									className="w-24"
								/>
							</div>
							{v.items?.type === EmailVariableType.OBJECT && (
								<div className="pl-3 border-l-2 border-border">
									<p className="text-[10px] text-muted-foreground uppercase tracking-wider mb-1.5">
										Object fields
									</p>
									<PropertyListEditor
										properties={v.items.properties ?? []}
										onChange={(props) => updateItems(i, { properties: props })}
									/>
								</div>
							)}
						</div>
					)}

					{/* OBJECT sub-schema */}
					{v.type === EmailVariableType.OBJECT && (
						<div className="border-t border-border px-3 py-2.5 flex flex-col gap-1.5 bg-muted/30 rounded-b-lg">
							<p className="text-[10px] text-muted-foreground uppercase tracking-wider">
								Properties
							</p>
							<PropertyListEditor
								properties={v.properties ?? []}
								onChange={(props) => update(i, { properties: props })}
							/>
						</div>
					)}
				</div>
			))}
			<Button variant="outline" size="sm" onClick={add}>
				+ Add Variable
			</Button>
			{variables.length > 0 && (
				<p className="text-xs text-muted-foreground">
					Use in template:{" "}
					<code className="font-mono bg-muted px-1 rounded">
						{"{{ .variable_name }}"}
					</code>
					{" · "}list:{" "}
					<code className="font-mono bg-muted px-1 rounded">
						{"{{ range .list }}{{ .field }}{{ end }}"}
					</code>
				</p>
			)}
		</div>
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
				value={value}
				onChange={(e) => onChange(e.target.value)}
				placeholder='[{"field": "value"}, ...]'
				className="text-xs font-mono resize-y min-h-[60px]"
				rows={3}
			/>
		);
	}

	return (
		<div className="flex flex-col gap-1.5">
			<div className="rounded-md border border-border overflow-hidden">
				{/* Header */}
				<div
					className="grid bg-muted/60 border-b border-border"
					style={{
						gridTemplateColumns: `repeat(${properties.length}, 1fr) 28px`,
					}}
				>
					{properties.map((p) => (
						<div
							key={p.name}
							className="px-2 py-1 text-[10px] font-medium text-muted-foreground uppercase tracking-wider truncate"
						>
							{p.name}
							<span className="ml-1 text-muted-foreground/50 normal-case tracking-normal">
								{p.type.toLowerCase()}
							</span>
						</div>
					))}
					<div />
				</div>
				{/* Rows */}
				{rows.length === 0 ? (
					<div className="px-2 py-3 text-xs text-muted-foreground text-center">
						No items — click + Add row
					</div>
				) : (
					rows.map((row, i) => (
						<div
							key={rowKeysRef.current[i]}
							className={`grid items-center ${i < rows.length - 1 ? "border-b border-border" : ""}`}
							style={{
								gridTemplateColumns: `repeat(${properties.length}, 1fr) 28px`,
							}}
						>
							{properties.map((p) => (
								<input
									key={p.name}
									value={row[p.name] ?? ""}
									onChange={(e) => updateCell(i, p.name, e.target.value)}
									className="px-2 py-1.5 text-xs bg-transparent border-r border-border last:border-r-0 focus:outline-none focus:bg-primary/5 font-mono"
									placeholder="—"
								/>
							))}
							<button
								type="button"
								onClick={() => removeRow(i)}
								className="flex items-center justify-center h-full text-muted-foreground/50 hover:text-destructive text-xs"
							>
								✕
							</button>
						</div>
					))
				)}
			</div>
			<button
				type="button"
				onClick={addRow}
				className="text-xs text-muted-foreground hover:text-foreground border border-dashed border-muted-foreground/40 hover:border-foreground/40 rounded px-2 py-0.5 transition-colors"
			>
				+ Add row
			</button>
		</div>
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
				value={value}
				onChange={(e) => onChange(e.target.value)}
				placeholder='{"field": "value"}'
				className="text-xs font-mono resize-y min-h-[60px]"
				rows={3}
			/>
		);
	}

	return (
		<div className="rounded-md border border-border overflow-hidden">
			{properties.map((p, i) => (
				<div
					key={p.name}
					className={`grid items-center ${i < properties.length - 1 ? "border-b border-border" : ""}`}
					style={{ gridTemplateColumns: "120px 1fr" }}
				>
					<div className="px-2 py-1.5 bg-muted/40 border-r border-border">
						<span className="text-xs font-mono text-muted-foreground">
							{p.name}
						</span>
					</div>
					<input
						value={obj[p.name] ?? ""}
						onChange={(e) => updateField(p.name, e.target.value)}
						className="px-2 py-1.5 text-xs bg-transparent focus:outline-none focus:bg-primary/5 font-mono"
						placeholder="—"
					/>
				</div>
			))}
		</div>
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
		<div className="flex flex-col gap-1">
			<div className="flex items-center gap-1.5">
				<span className="font-mono text-xs text-foreground">{name}</span>
				{typeLabel() && (
					<span className="text-[10px] text-muted-foreground/70 bg-muted px-1.5 rounded">
						{typeLabel()}
					</span>
				)}
			</div>
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
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder={
						type === EmailVariableType.LIST
							? '["value1", "value2"]'
							: '{"key": "value"}'
					}
					className="text-xs font-mono resize-y min-h-[48px]"
					rows={2}
				/>
			) : (
				<Input
					value={value}
					onChange={(e) => onChange(e.target.value)}
					placeholder={`value for .${name}`}
					className="text-xs h-7 font-mono"
				/>
			)}
		</div>
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
		<div className="flex flex-col gap-4">
			{/* Header */}
			<div className="flex items-center justify-between">
				<div className="flex items-center gap-2">
					<LayoutTemplate className="size-4 text-muted-foreground" />
					<p className="text-sm text-muted-foreground">
						Named variable sets for preview and test sends
					</p>
				</div>
				<Button variant="outline" size="sm" onClick={handleNewExample}>
					<Plus className="size-3.5 mr-1" />
					New Example
				</Button>
			</div>

			{examples.length === 0 ? (
				<div className="border rounded-lg p-8 text-center">
					<FileText className="size-8 text-muted-foreground mx-auto mb-3" />
					<p className="text-sm text-muted-foreground mb-1">No examples yet</p>
					<p className="text-xs text-muted-foreground">
						Create an example to pre-fill variables for preview and test sends
					</p>
				</div>
			) : (
				<div className="flex gap-3">
					{/* Example list sidebar */}
					<div className="w-44 shrink-0">
						<div className="text-xs font-medium text-muted-foreground mb-1.5 px-1">
							Examples ({examples.length})
						</div>
						<div className="flex flex-col gap-0.5">
							{examples.map((ex) => (
								<button
									type="button"
									key={ex.id}
									onClick={() => handleSelectExample(ex.id)}
									className={`w-full flex items-center gap-2 text-left text-sm px-2.5 py-1.5 rounded-md transition-colors ${
										activeId === ex.id
											? "bg-primary/10 text-primary font-medium"
											: "hover:bg-muted text-muted-foreground"
									}`}
								>
									<FileText className="size-3.5 shrink-0 opacity-70" />
									<span className="truncate">{ex.name || "Unnamed"}</span>
								</button>
							))}
						</div>
					</div>

					{/* Active example editor */}
					{activeExample && (
						<div className="flex-1 bg-muted/30 rounded-lg border p-4 flex flex-col gap-3">
							{/* Editor toolbar */}
							<div className="flex items-center gap-2">
								<Input
									value={activeExample.name}
									onChange={(e) =>
										handleNameChange(activeExample.id, e.target.value)
									}
									placeholder="Example name"
									className="text-sm h-8 flex-1 bg-background"
								/>
								<Button
									variant="ghost"
									size="sm"
									className="h-8 px-2 text-muted-foreground hover:text-foreground"
									onClick={() => toggleRawMode(!rawMode)}
								>
									<Code2 className="size-3.5 mr-1" />
									{rawMode ? "Form" : "Raw"}
								</Button>
								<Button
									variant="ghost"
									size="sm"
									className="h-8 px-2 text-destructive hover:text-destructive hover:bg-destructive/10"
									onClick={() => handleDeleteExample(activeExample.id)}
								>
									<Trash2 className="size-3.5" />
								</Button>
							</div>

							{/* Editor content */}
							{rawMode ? (
								<div className="flex flex-col gap-1">
									<Textarea
										value={rawText}
										onChange={(e) => handleRawChange(e.target.value)}
										className="text-xs font-mono resize-y min-h-[200px] bg-background"
										spellCheck={false}
										placeholder='{"userName": "Alice", "orderTotal": 99.99}'
									/>
									{rawError && (
										<div className="flex items-center gap-1.5 text-xs text-destructive font-mono">
											<AlertCircle className="size-3" />
											{rawError}
										</div>
									)}
								</div>
							) : varNames.length === 0 ? (
								<div className="py-6 text-center">
									<p className="text-xs text-muted-foreground">
										No variables detected yet. Add{" "}
										<code className="font-mono bg-muted px-1 rounded">
											{"{{ .varName }}"}
										</code>{" "}
										to your template.
									</p>
								</div>
							) : (
								<div className="flex flex-col gap-3">
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
								</div>
							)}
						</div>
					)}
				</div>
			)}

			{/* Footer actions */}
			<div className="flex items-center gap-2">
				<Button
					size="sm"
					onClick={handleSave}
					disabled={saveStatus === "saving"}
				>
					<Save className="size-3.5 mr-1" />
					{saveStatus === "saving" ? "Saving…" : "Save Examples"}
				</Button>
				{saveStatus === "saved" && (
					<span className="text-xs text-success flex items-center gap-1">
						<Check className="size-3" />
						Saved
					</span>
				)}
				{saveStatus === "error" && (
					<span className="text-xs text-destructive flex items-center gap-1">
						<AlertCircle className="size-3" />
						Save failed
					</span>
				)}
			</div>
		</div>
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
		mutate({
			path: { product_id: productId },
			body: {
				template_id: templateId,
				to_address: toAddress,
				variables: buildVarsPayload(varValues, variables),
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
			<TooltipProvider>
				<Tooltip>
					<TooltipTrigger asChild>
						<span tabIndex={!hasEmailIntegration ? 0 : undefined}>
							<Button
								variant="outline"
								size="sm"
								disabled={!hasEmailIntegration}
								onClick={() => hasEmailIntegration && setOpen(true)}
							>
								Send Test
							</Button>
						</span>
					</TooltipTrigger>
					{!hasEmailIntegration && (
						<TooltipContent side="bottom">
							No active SMTP integration. Go to Integrations → SMTP Email to
							configure one.
						</TooltipContent>
					)}
				</Tooltip>
			</TooltipProvider>
			<DialogContent className="max-w-md">
				<DialogHeader>
					<DialogTitle>Send Test Email</DialogTitle>
				</DialogHeader>
				<form onSubmit={handleSend} className="flex flex-col gap-4">
					<div className="flex flex-col gap-1">
						<Label htmlFor="to">Recipient</Label>
						<Input
							id="to"
							type="email"
							required
							value={toAddress}
							onChange={(e) => setToAddress(e.target.value)}
							placeholder="you@example.com"
						/>
					</div>
					{variables.length > 0 && (
						<div className="flex flex-col gap-2">
							<Label>Variables</Label>
							{variables.map((v) => (
								<div key={v.name} className="flex items-center gap-2">
									<span className="font-mono text-xs text-muted-foreground w-24 shrink-0">
										{v.name}
									</span>
									<Input
										placeholder={v.required ? "required" : "optional"}
										value={varValues[v.name] ?? ""}
										onChange={(e) =>
											setVarValues((prev) => ({
												...prev,
												[v.name]: e.target.value,
											}))
										}
										className="text-sm"
									/>
								</div>
							))}
						</div>
					)}
					{error && <p className="text-sm text-destructive">{error}</p>}
					{result && (
						<p className="text-sm text-success">
							Sent — status: <strong>{result.status}</strong> (ID: {result.id})
						</p>
					)}
					<div className="flex justify-end">
						<Button type="submit" disabled={isPending}>
							{isPending ? "Sending…" : "Send"}
						</Button>
					</div>
				</form>
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
		<div className="flex flex-col h-full gap-4">
			<div className="flex items-center justify-between">
				<Button
					onClick={handlePreview}
					disabled={isPreviewing}
					size="sm"
					variant="outline"
				>
					{isPreviewing ? "Rendering…" : "Refresh Preview"}
				</Button>
				{isPreviewing && (
					<span className="text-xs text-muted-foreground animate-pulse">
						Rendering…
					</span>
				)}
			</div>
			{previewError && (
				<div className="rounded-md border border-destructive/40 bg-destructive/5 px-3 py-2">
					<p className="text-xs text-destructive font-mono whitespace-pre-wrap">
						{previewError}
					</p>
				</div>
			)}
			{preview ? (
				<div className="flex flex-col gap-2 flex-1 min-h-0">
					<div className="text-sm">
						<span className="font-medium">Subject: </span>
						<span className="text-muted-foreground">{preview.subject}</span>
					</div>
					{preview.warnings.length > 0 && (
						<div className="rounded-md border border-warning/40 bg-warning/10 px-3 py-2 flex flex-col gap-1">
							<p className="text-xs font-medium text-warning">
								{preview.warnings.length} render warning
								{preview.warnings.length > 1 ? "s" : ""}
							</p>
							<ul className="flex flex-col gap-0.5">
								{preview.warnings.map((w, i) => (
									<li
										key={previewWarningKeys[i] ?? `${w}-${i}`}
										className="text-xs text-warning font-mono"
									>
										· {w}
									</li>
								))}
							</ul>
						</div>
					)}
					<div className="flex-1 min-h-0 border rounded-md overflow-hidden">
						<iframe
							title="Email Preview"
							srcDoc={preview.body_html}
							className="w-full h-full"
							sandbox="allow-same-origin"
						/>
					</div>
				</div>
			) : draftVersion ? (
				<div className="flex-1 min-h-0 border rounded-md overflow-hidden">
					<iframe
						title="Email Preview"
						srcDoc={draftVersion.body_html}
						className="w-full h-full"
						sandbox="allow-same-origin"
					/>
				</div>
			) : (
				<div className="flex-1 flex items-center justify-center text-sm text-muted-foreground border rounded-md">
					Click "Refresh Preview" to render
				</div>
			)}
		</div>
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
			<div className="p-8 text-muted-foreground text-sm">Loading template…</div>
		);
	}

	if (!template) {
		return (
			<div className="p-8 text-destructive text-sm">Template not found.</div>
		);
	}

	return (
		<div className="flex flex-col h-full gap-0">
			{/* Header */}
			<div className="flex items-center justify-between px-6 py-3 border-b shrink-0">
				<div className="flex items-center gap-3">
					<Link
						to={ROUTE_PATHS.EMAIL_TEMPLATES}
						className="text-sm text-muted-foreground hover:text-foreground"
					>
						← Templates
					</Link>
					<Separator orientation="vertical" className="h-4" />
					<input
						value={name}
						onChange={(e) => handleNameChange(e.target.value)}
						className="font-semibold text-sm bg-transparent border-0 border-b border-transparent hover:border-border focus:border-border outline-none px-0 py-0.5 w-48"
						placeholder="Template name"
					/>
					<span className="font-mono text-xs text-muted-foreground">
						/{template.slug}
					</span>
					{template.published_version_id ? (
						<StatusBadge tone="info">Published</StatusBadge>
					) : (
						<StatusBadge tone="warning">Draft only</StatusBadge>
					)}
				</div>
				<div className="flex items-center gap-2">
					<span className="text-xs text-muted-foreground">
						{saveState === "saving" && "Saving…"}
						{saveState === "saved" && "Saved"}
						{saveState === "error" && "Save failed"}
					</span>
					<TestSendDialog
						productId={productId}
						templateId={templateId}
						variables={variables}
						activeVarValues={activeVarValues}
						hasEmailIntegration={hasEmailIntegration}
					/>
					<Button
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
				</div>
			</div>
			{publishError && (
				<div className="px-6 py-2 text-sm text-destructive bg-destructive/10 border-b">
					{publishError}
				</div>
			)}

			{/* Body — split editor / preview */}
			<div className="flex flex-1 min-h-0">
				{/* Left: editor */}
				<div className="flex flex-col w-1/2 border-r min-h-0">
					<Tabs defaultValue="content" className="flex flex-col flex-1 min-h-0">
						<TabsList className="mx-4 mt-4 w-fit shrink-0">
							<TabsTrigger value="content">Content</TabsTrigger>
							<TabsTrigger value="variables">
								Variables ({variables.length})
							</TabsTrigger>
							<TabsTrigger value="examples">Examples</TabsTrigger>
						</TabsList>

						<TabsContent
							value="content"
							className="flex flex-col flex-1 min-h-0 px-4 pb-4 gap-3"
						>
							<div className="flex flex-col gap-1 shrink-0">
								<Label htmlFor="subject">Subject</Label>
								<Input
									id="subject"
									value={subject}
									onChange={(e) => handleSubjectChange(e.target.value)}
									placeholder="Hello {{ .name }}"
									className="font-mono"
								/>
							</div>
							<div className="flex flex-col flex-1 min-h-0 gap-1">
								<Label className="shrink-0">HTML Body</Label>
								<div className="flex-1 min-h-0 border rounded-md overflow-hidden">
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
								</div>
							</div>
						</TabsContent>

						<TabsContent
							value="variables"
							className="flex-1 min-h-0 overflow-y-auto px-4 pb-4"
						>
							<div className="flex flex-col gap-4">
								<p className="text-sm text-muted-foreground">
									Define the variables your template expects. These appear in
									the preview and test-send panels.
								</p>
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
										<div className="border rounded-md p-3 flex flex-col gap-2 bg-muted/40">
											<div className="flex items-center justify-between">
												<p className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
													Detected in template
												</p>
												<Button
													variant="outline"
													size="sm"
													className="h-6 text-xs"
													onClick={pushAll}
												>
													Push all ({unpushed.length})
												</Button>
											</div>
											<div className="flex flex-col gap-1.5">
												{unpushed.map((s) => {
													const isListObj =
														s.type === EmailVariableType.LIST &&
														s.items?.type === EmailVariableType.OBJECT;
													const fields = isListObj
														? (s.items?.properties ?? [])
														: [];
													return (
														<button
															type="button"
															key={s.name}
															onClick={() => pushOne(s)}
															className="w-full text-left flex items-start gap-2 px-2.5 py-1.5 rounded border border-dashed border-muted-foreground/40 hover:border-foreground/60 hover:bg-background transition-colors group"
														>
															<span className="font-mono text-xs text-foreground group-hover:text-foreground mt-0.5">
																+ {s.name}
															</span>
															<div className="flex flex-wrap items-center gap-1 mt-0.5">
																<span
																	className={`text-[10px] px-1.5 py-0 rounded font-medium ${
																		s.type === EmailVariableType.LIST
																			? "bg-accent-soft text-accent-foreground border border-border"
																			: s.type === EmailVariableType.OBJECT
																				? "bg-secondary text-secondary-foreground border border-border"
																				: "bg-muted text-muted-foreground border border-border"
																	}`}
																>
																	{s.type === EmailVariableType.LIST && s.items
																		? `LIST · ${s.items.type}`
																		: s.type}
																</span>
																{fields.map((f) => (
																	<span
																		key={f.name}
																		className="text-[10px] font-mono text-muted-foreground/70 bg-muted px-1 rounded"
																	>
																		{f.name}
																	</span>
																))}
															</div>
														</button>
													);
												})}
											</div>
										</div>
									);
								})()}
								<VariableEditor
									variables={variables}
									onChange={handleVariablesChange}
								/>
							</div>
						</TabsContent>

						<TabsContent
							value="examples"
							className="flex-1 min-h-0 overflow-y-auto px-4 pb-4"
						>
							<ExampleManager
								productId={productId}
								templateId={templateId}
								detectedVarNames={detectedVarNames}
								variables={variables}
								activeVarValues={activeVarValues}
								onActiveChange={setActiveVarValues}
							/>
						</TabsContent>
					</Tabs>
				</div>

				{/* Right: preview */}
				<div className="flex flex-col w-1/2 p-4 min-h-0">
					<p className="text-xs text-muted-foreground uppercase tracking-wide font-medium mb-3">
						Live Preview
					</p>
					<PreviewPane
						productId={productId}
						templateId={templateId}
						variables={variables}
						draftVersion={draft ?? null}
						activeVarValues={activeVarValues}
					/>
				</div>
			</div>
		</div>
	);
}
