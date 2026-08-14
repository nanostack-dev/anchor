import {
	type LicenseFieldDeclaration,
	type LicenseFieldRules,
	LicenseFieldType,
	type LicenseSchemaResponse,
} from "@/client";

import { sanitizeRules } from "./license-field-format";

/**
 * One field being authored. `rules` is optional on the wire (an unconstrained
 * field omits it) but always present here, so every row has something to bind
 * inputs to and update in place. `uiKey` is local identity: a field has no
 * server id until the schema is saved, and the name is editable, so neither can
 * key a React list.
 */
export type FieldRow = Omit<LicenseFieldDeclaration, "rules"> & {
	uiKey: string;
	rules: LicenseFieldRules;
};

export function newFieldRow(
	overrides: Partial<Omit<FieldRow, "uiKey">> = {},
): FieldRow {
	return {
		uiKey: crypto.randomUUID(),
		name: "",
		type: LicenseFieldType.STRING,
		description: "",
		rules: {},
		...overrides,
	};
}

export function fieldRowsFromSchema(schema: LicenseSchemaResponse): FieldRow[] {
	return schema.fields.map((field) =>
		newFieldRow({
			name: field.name,
			type: field.type,
			description: field.description ?? "",
			rules: field.rules ?? {},
		}),
	);
}

export function fieldRowsToDeclarations(
	rows: FieldRow[],
): LicenseFieldDeclaration[] {
	return rows.map((row) => ({
		name: row.name.trim(),
		type: row.type,
		description: row.description?.trim() || undefined,
		rules: sanitizeRules(row.type, row.rules),
	}));
}

/**
 * Client-side pass over a draft, keyed by `uiKey`. It catches the mistakes an
 * operator makes while typing; the API stays the source of truth for anything a
 * rule evaluator has to compile (a regex) or that depends on server state (a
 * duplicate name racing another write).
 */
export function validateFieldRows(rows: FieldRow[]): Record<string, string> {
	const errors: Record<string, string> = {};
	const namesSeen = new Map<string, string[]>();

	for (const row of rows) {
		const name = row.name.trim();
		if (!name) {
			errors[row.uiKey] = "Name is required.";
			continue;
		}
		namesSeen.set(name, [...(namesSeen.get(name) ?? []), row.uiKey]);

		if (
			row.type === LicenseFieldType.ENUM &&
			(row.rules.values?.length ?? 0) === 0
		) {
			errors[row.uiKey] = "Add at least one allowed value.";
			continue;
		}
		if (
			row.rules.min !== undefined &&
			row.rules.max !== undefined &&
			row.rules.min > row.rules.max
		) {
			errors[row.uiKey] = "Min must not exceed max.";
			continue;
		}
		if (
			row.rules.min_length !== undefined &&
			row.rules.max_length !== undefined &&
			row.rules.min_length > row.rules.max_length
		) {
			errors[row.uiKey] = "Min length must not exceed max length.";
		}
	}

	for (const uiKeys of namesSeen.values()) {
		if (uiKeys.length > 1) {
			for (const uiKey of uiKeys) {
				errors[uiKey] = "This field name is used more than once.";
			}
		}
	}

	return errors;
}
