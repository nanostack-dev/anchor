import { type LicenseFieldRules, LicenseFieldType } from "@/client";

/** Operator-facing label for a license field type. */
export const FIELD_TYPE_LABELS: Record<LicenseFieldType, string> = {
	[LicenseFieldType.LIMIT]: "Limit",
	[LicenseFieldType.NUMBER]: "Number",
	[LicenseFieldType.BOOLEAN]: "Boolean",
	[LicenseFieldType.ENUM]: "Enum",
	[LicenseFieldType.STRING]: "String",
};

export const FIELD_TYPES: LicenseFieldType[] = [
	LicenseFieldType.LIMIT,
	LicenseFieldType.NUMBER,
	LicenseFieldType.BOOLEAN,
	LicenseFieldType.ENUM,
	LicenseFieldType.STRING,
];

/**
 * Drops rule keys that don't apply to `type`, so a leftover rule from a
 * previous type selection never reaches the API. Mirrors
 * `internal/license/rules.rejectInapplicable` on the backend.
 */
export function sanitizeRules(
	type: LicenseFieldType,
	rules: LicenseFieldRules,
): LicenseFieldRules {
	switch (type) {
		case LicenseFieldType.LIMIT:
		case LicenseFieldType.NUMBER:
			return { min: rules.min, max: rules.max };
		case LicenseFieldType.STRING:
			return {
				pattern: rules.pattern,
				min_length: rules.min_length,
				max_length: rules.max_length,
			};
		case LicenseFieldType.ENUM:
			return { values: rules.values };
		default:
			return {};
	}
}

/** One-line human summary of a field's rules, for a read-only table row. */
export function summarizeRules(
	type: LicenseFieldType,
	rules: LicenseFieldRules,
): string {
	const parts: string[] = [];
	if (rules.min !== undefined) parts.push(`min ${rules.min}`);
	if (rules.max !== undefined) parts.push(`max ${rules.max}`);
	if (rules.min_length !== undefined)
		parts.push(`min length ${rules.min_length}`);
	if (rules.max_length !== undefined)
		parts.push(`max length ${rules.max_length}`);
	if (rules.pattern) parts.push(`pattern ${rules.pattern}`);
	if (rules.values?.length) parts.push(`values: ${rules.values.join(", ")}`);

	if (parts.length === 0) {
		return type === LicenseFieldType.BOOLEAN
			? "No rules apply"
			: "No constraints";
	}
	return parts.join(" · ");
}

/**
 * Whether a field carries a value at all. Every field the schema declares must
 * stay set: a template is rejected without one, and an adjustment cannot
 * remove one.
 */
export function isFieldValueSet(
	type: LicenseFieldType,
	value: unknown,
): boolean {
	if (type === LicenseFieldType.BOOLEAN) return typeof value === "boolean";
	if (type === LicenseFieldType.NUMBER || type === LicenseFieldType.LIMIT) {
		return typeof value === "number" && Number.isFinite(value);
	}
	return typeof value === "string" && value.trim().length > 0;
}

/** Renders a template/license value for read-only display. */
export function formatFieldValue(
	type: LicenseFieldType,
	value: unknown,
): string {
	if (value === null || value === undefined) return "—";
	if (type === LicenseFieldType.BOOLEAN) return value ? "Yes" : "No";
	return String(value);
}
