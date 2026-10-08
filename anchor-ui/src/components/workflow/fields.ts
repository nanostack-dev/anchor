import {
	WorkflowFieldType,
	WorkflowOperator,
	WorkflowParamType,
} from "@/client";
import type { WorkflowVariable } from "./workflow-model";

export const FIELD_DRAG_TYPE = "application/x-anchor-workflow-field";

export const fieldTypeLabels: Record<WorkflowFieldType, string> = {
	[WorkflowFieldType.TEXT]: "Text",
	[WorkflowFieldType.EMAIL]: "Email",
	[WorkflowFieldType.NUMBER]: "Number",
	[WorkflowFieldType.BOOLEAN]: "Yes or no",
	[WorkflowFieldType.TIMESTAMP]: "Date and time",
	[WorkflowFieldType.URL]: "URL",
	[WorkflowFieldType.JSON]: "JSON",
	[WorkflowFieldType.ORGANIZATION]: "Organization ID",
	[WorkflowFieldType.WORKSPACE]: "Workspace ID",
	[WorkflowFieldType.PRODUCT_USER]: "Product user ID",
	[WorkflowFieldType.INVITATION]: "Invitation ID",
	[WorkflowFieldType.API_KEY]: "API key ID",
	[WorkflowFieldType.ROLE]: "Role ID",
	[WorkflowFieldType.PERMISSION]: "Permission",
	[WorkflowFieldType.LICENSE]: "License ID",
	[WorkflowFieldType.LICENSE_TEMPLATE]: "License template ID",
};

const exactFit: Partial<Record<WorkflowParamType, WorkflowFieldType[]>> = {
	[WorkflowParamType.ORGANIZATION]: [WorkflowFieldType.ORGANIZATION],
	[WorkflowParamType.PRODUCT_USER]: [WorkflowFieldType.PRODUCT_USER],
	[WorkflowParamType.ROLE]: [WorkflowFieldType.ROLE],
	[WorkflowParamType.LICENSE_TEMPLATE]: [WorkflowFieldType.LICENSE_TEMPLATE],
	[WorkflowParamType.EMAIL]: [WorkflowFieldType.EMAIL],
	[WorkflowParamType.URL]: [WorkflowFieldType.URL],
};

/** Whether a field of this type is a sensible value for a parameter. */
export function fits(
	paramType: WorkflowParamType,
	fieldType: WorkflowFieldType,
): boolean {
	const wanted = exactFit[paramType];
	if (wanted) return wanted.includes(fieldType);
	if (paramType === WorkflowParamType.JSON) return true;
	return fieldType !== WorkflowFieldType.JSON;
}

/** What a parameter of this type expects, in words, when it expects one kind. */
export function expectedTypeLabel(
	paramType: WorkflowParamType,
): string | undefined {
	const wanted = exactFit[paramType];
	return wanted ? fieldTypeLabels[wanted[0]] : undefined;
}

const wholeReference = /^\s*\{\{\s*([^{}]+?)\s*\}\}\s*$/;

/** The path of a value that is exactly one `{{ }}` reference. */
export function singleReference(value: string): string | undefined {
	return value.match(wholeReference)?.[1];
}

export function fieldAt(
	fields: WorkflowVariable[],
	path: string,
): WorkflowVariable | undefined {
	return fields.find((field) => field.path === path);
}

/** The type a value most likely has: the field it references, or its shape. */
export function inferFieldType(
	value: string,
	fields: WorkflowVariable[],
): WorkflowFieldType {
	const path = singleReference(value);
	if (path) return fieldAt(fields, path)?.type ?? WorkflowFieldType.TEXT;
	const text = value.trim();
	if (/^-?\d+(\.\d+)?$/.test(text)) return WorkflowFieldType.NUMBER;
	if (/^(true|false)$/i.test(text)) return WorkflowFieldType.BOOLEAN;
	if (/^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(text)) return WorkflowFieldType.EMAIL;
	if (/^https?:\/\//i.test(text)) return WorkflowFieldType.URL;
	return WorkflowFieldType.TEXT;
}

export function matchesQuery(field: WorkflowVariable, query: string): boolean {
	const needle = query.trim().toLowerCase();
	if (!needle) return true;
	return [field.path, field.label, field.source, fieldTypeLabels[field.type]]
		.join(" ")
		.toLowerCase()
		.includes(needle);
}

/**
 * The reference being typed at the caret: a `{{` with no `}}` after it on the
 * way to the caret. `start` is where the `{{` begins.
 */
export function openReference(
	value: string,
	caret: number,
): { start: number; query: string } | undefined {
	const before = value.slice(0, caret);
	const start = before.lastIndexOf("{{");
	if (start < 0) return undefined;
	const query = before.slice(start + 2);
	if (/[{}\n]/.test(query)) return undefined;
	return { start, query: query.trim() };
}

/** Replaces the reference being typed with a complete one. */
export function completeReference(
	value: string,
	caret: number,
	start: number,
	path: string,
): { value: string; caret: number } {
	const after = value.slice(caret).replace(/^[^{}\s]*\}\}/, "");
	const inserted = `{{${path}}}`;
	return {
		value: `${value.slice(0, start)}${inserted}${after}`,
		caret: start + inserted.length,
	};
}

/** Puts a reference at the caret, or makes it the whole value when empty. */
export function insertReference(
	value: string,
	caret: number | null,
	path: string,
): { value: string; caret: number } {
	const inserted = `{{${path}}}`;
	if (!value.trim() || singleReference(value)) {
		return { value: inserted, caret: inserted.length };
	}
	const at = caret ?? value.length;
	return {
		value: `${value.slice(0, at)}${inserted}${value.slice(at)}`,
		caret: at + inserted.length,
	};
}

export interface EventDataRow {
	key: string;
	value: string;
	type: WorkflowFieldType;
}

const fieldTypes = new Set<string>(Object.values(WorkflowFieldType));

/**
 * Reads a custom event's `data` and `data_types` into rows. Answers undefined
 * when the data holds anything rows cannot show: a nested value, a number or a
 * reference outside quotes. The raw JSON editor takes over then.
 */
export function parseEventData(
	data: string,
	types: string,
): EventDataRow[] | undefined {
	if (!data.trim()) return [];
	let object: unknown;
	try {
		object = JSON.parse(data);
	} catch {
		return undefined;
	}
	if (!object || typeof object !== "object" || Array.isArray(object)) {
		return undefined;
	}
	let declared: Record<string, string> = {};
	if (types.trim()) {
		try {
			declared = JSON.parse(types);
		} catch {
			return undefined;
		}
	}
	const rows: EventDataRow[] = [];
	for (const [key, value] of Object.entries(object)) {
		if (typeof value !== "string") return undefined;
		const type = declared[key];
		rows.push({
			key,
			value,
			type:
				type && fieldTypes.has(type)
					? (type as WorkflowFieldType)
					: WorkflowFieldType.TEXT,
		});
	}
	return rows;
}

export function serializeEventData(rows: EventDataRow[]): {
	data: string;
	types: string;
} {
	const named = rows.filter((row) => row.key.trim());
	if (named.length === 0) return { data: "", types: "" };
	return {
		data: JSON.stringify(
			Object.fromEntries(named.map((row) => [row.key.trim(), row.value])),
		),
		types: JSON.stringify(
			Object.fromEntries(named.map((row) => [row.key.trim(), row.type])),
		),
	};
}

export const eventDataKeyPattern = /^[a-z][a-z0-9_]*$/;

const identifierTypes = new Set<WorkflowFieldType>([
	WorkflowFieldType.ORGANIZATION,
	WorkflowFieldType.WORKSPACE,
	WorkflowFieldType.PRODUCT_USER,
	WorkflowFieldType.INVITATION,
	WorkflowFieldType.API_KEY,
	WorkflowFieldType.ROLE,
	WorkflowFieldType.PERMISSION,
	WorkflowFieldType.LICENSE,
	WorkflowFieldType.LICENSE_TEMPLATE,
]);

/** The comparisons that make sense for a field of this type. */
export function operatorsFor(type?: WorkflowFieldType): WorkflowOperator[] {
	if (
		!type ||
		type === WorkflowFieldType.TEXT ||
		type === WorkflowFieldType.EMAIL ||
		type === WorkflowFieldType.URL
	) {
		return Object.values(WorkflowOperator);
	}
	if (type === WorkflowFieldType.JSON) {
		return [
			WorkflowOperator.CONTAINS,
			WorkflowOperator.NOT_CONTAINS,
			WorkflowOperator.EXISTS,
			WorkflowOperator.NOT_EXISTS,
		];
	}
	if (type === WorkflowFieldType.BOOLEAN) {
		return [
			WorkflowOperator.EQUALS,
			WorkflowOperator.NOT_EQUALS,
			WorkflowOperator.EXISTS,
			WorkflowOperator.NOT_EXISTS,
		];
	}
	return [
		WorkflowOperator.EQUALS,
		WorkflowOperator.NOT_EQUALS,
		WorkflowOperator.IN,
		WorkflowOperator.EXISTS,
		WorkflowOperator.NOT_EXISTS,
	];
}

export function isIdentifier(type?: WorkflowFieldType): boolean {
	return type !== undefined && identifierTypes.has(type);
}

/** What to type when comparing a field of this type. */
export function comparedValueHint(
	type: WorkflowFieldType | undefined,
	operator: WorkflowOperator,
): string {
	if (operator === WorkflowOperator.IN) {
		return type === WorkflowFieldType.NUMBER ? "1, 5, 10" : "a, b, c";
	}
	if (isIdentifier(type) && type) {
		return `${fieldTypeLabels[type]} or a field`;
	}
	switch (type) {
		case WorkflowFieldType.EMAIL:
			return operator === WorkflowOperator.ENDS_WITH
				? "@example.com"
				: "someone@example.com";
		case WorkflowFieldType.NUMBER:
			return "25";
		case WorkflowFieldType.TIMESTAMP:
			return "2026-10-08T09:00:00Z";
		case WorkflowFieldType.URL:
			return "https://example.com";
		default:
			return "value";
	}
}

/** Paths a person may type that no listed field names, such as a metadata key. */
export const fieldPathPattern =
	/^(event\.(data\.[a-z0-9_]+|type|id)|steps\.[a-z][a-z0-9_]*\.[a-zA-Z0-9_.]+|workflow\.(id|name))$/;
