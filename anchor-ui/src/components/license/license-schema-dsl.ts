import { type LicenseFieldRules, LicenseFieldType } from "@/client";

import { type FieldRow, newFieldRow } from "./license-schema-draft";

/**
 * A one-line-per-field text form of a license schema.
 *
 *   name: type [constraints] [# description]
 *
 *   max_flows:   limit 0..100                    # Concurrent flows allowed
 *   seats:       number 1..
 *   sso:         boolean
 *   tier:        enum free | pro | enterprise
 *   webhook_url: string /^https:\/\// len 1..2048
 *
 * The grammar carries exactly the rules the backend evaluator applies per type
 * (`internal/license/rules`), so a parsed draft can never hold a rule the
 * server would reject as inapplicable. Blank lines and full-line `#` comments
 * are ignored.
 */

export interface DslError {
	/** 1-based, so it matches what the operator counts in the textarea. */
	line: number;
	message: string;
}

export interface DslParseResult {
	rows: FieldRow[];
	errors: DslError[];
}

const TYPE_KEYWORDS: Record<string, LicenseFieldType> = {
	limit: LicenseFieldType.LIMIT,
	number: LicenseFieldType.NUMBER,
	boolean: LicenseFieldType.BOOLEAN,
	enum: LicenseFieldType.ENUM,
	string: LicenseFieldType.STRING,
};

const TYPE_KEYWORD_BY_TYPE: Record<LicenseFieldType, string> = {
	[LicenseFieldType.LIMIT]: "limit",
	[LicenseFieldType.NUMBER]: "number",
	[LicenseFieldType.BOOLEAN]: "boolean",
	[LicenseFieldType.ENUM]: "enum",
	[LicenseFieldType.STRING]: "string",
};

export const DSL_TYPE_KEYWORDS = Object.keys(TYPE_KEYWORDS);

/**
 * Splits a line into its body and its trailing `# description`. A `#` inside a
 * `/regex/` belongs to the pattern, so the scan tracks whether it is currently
 * inside one rather than searching for the last `#`.
 */
function splitComment(line: string): { body: string; description?: string } {
	let inPattern = false;
	for (let i = 0; i < line.length; i++) {
		const char = line[i];
		if (char === "\\") {
			i++;
			continue;
		}
		if (char === "/") {
			inPattern = !inPattern;
			continue;
		}
		if (char === "#" && !inPattern) {
			return {
				body: line.slice(0, i).trim(),
				description: line.slice(i + 1).trim() || undefined,
			};
		}
	}
	return { body: line.trim() };
}

/** `0..100`, `0..`, `..100`. Returns null when the token is not a range. */
function parseRange(
	token: string,
): { min?: number; max?: number } | null | "invalid" {
	if (!token.includes("..")) return null;
	const [rawMin, rawMax] = token.split("..");
	if (rawMax === undefined) return "invalid";

	const min = rawMin.trim() === "" ? undefined : Number(rawMin);
	const max = rawMax.trim() === "" ? undefined : Number(rawMax);
	if (Number.isNaN(min) || Number.isNaN(max)) return "invalid";
	if (min === undefined && max === undefined) return "invalid";
	return { min, max };
}

/** Splits on whitespace but keeps a `/regex/` token whole. */
function tokenize(body: string): string[] {
	const tokens: string[] = [];
	let current = "";
	let inPattern = false;

	for (let i = 0; i < body.length; i++) {
		const char = body[i];
		if (char === "\\" && inPattern) {
			current += char + (body[i + 1] ?? "");
			i++;
			continue;
		}
		if (char === "/") {
			inPattern = !inPattern;
			current += char;
			continue;
		}
		if (/\s/.test(char) && !inPattern) {
			if (current) tokens.push(current);
			current = "";
			continue;
		}
		current += char;
	}
	if (current) tokens.push(current);
	return tokens;
}

function parseConstraints(
	type: LicenseFieldType,
	tokens: string[],
	line: number,
	errors: DslError[],
): LicenseFieldRules {
	const rules: LicenseFieldRules = {};

	if (type === LicenseFieldType.BOOLEAN) {
		if (tokens.length > 0) {
			errors.push({ line, message: "A boolean field takes no rules." });
		}
		return rules;
	}

	if (type === LicenseFieldType.ENUM) {
		const values = tokens
			.join(" ")
			.split("|")
			.map((value) => value.trim())
			.filter(Boolean);
		if (values.length === 0) {
			errors.push({
				line,
				message: "An enum needs allowed values, separated by `|`.",
			});
		}
		rules.values = values;
		return rules;
	}

	if (type === LicenseFieldType.LIMIT || type === LicenseFieldType.NUMBER) {
		for (const token of tokens) {
			const range = parseRange(token);
			if (range === null) {
				errors.push({
					line,
					message: `Expected a range like \`0..100\`, got \`${token}\`.`,
				});
				continue;
			}
			if (range === "invalid") {
				errors.push({ line, message: `\`${token}\` is not a valid range.` });
				continue;
			}
			rules.min = range.min;
			rules.max = range.max;
		}
		if (
			rules.min !== undefined &&
			rules.max !== undefined &&
			rules.min > rules.max
		) {
			errors.push({ line, message: "Min must not exceed max." });
		}
		return rules;
	}

	// String: an optional `/pattern/` and an optional `len <range>`.
	for (let i = 0; i < tokens.length; i++) {
		const token = tokens[i];

		if (token.startsWith("/")) {
			if (!token.endsWith("/") || token.length < 2) {
				errors.push({ line, message: "A pattern must close with `/`." });
				continue;
			}
			// `\/` only exists to keep the delimiter unambiguous in source; the
			// stored pattern carries a plain slash.
			rules.pattern = token.slice(1, -1).replace(/\\\//g, "/");
			continue;
		}

		if (token === "len") {
			const rangeToken = tokens[i + 1];
			i++;
			const range = rangeToken ? parseRange(rangeToken) : null;
			if (range === null || range === "invalid") {
				errors.push({
					line,
					message: "`len` needs a range like `1..64`.",
				});
				continue;
			}
			rules.min_length = range.min;
			rules.max_length = range.max;
			continue;
		}

		errors.push({
			line,
			message: `Expected \`/pattern/\` or \`len 1..64\`, got \`${token}\`.`,
		});
	}

	if (
		rules.min_length !== undefined &&
		rules.max_length !== undefined &&
		rules.min_length > rules.max_length
	) {
		errors.push({ line, message: "Min length must not exceed max length." });
	}

	return rules;
}

export function parseSchemaDsl(source: string): DslParseResult {
	const rows: FieldRow[] = [];
	const errors: DslError[] = [];
	const namesSeen = new Map<string, number>();

	source.split("\n").forEach((rawLine, index) => {
		const line = index + 1;
		const trimmed = rawLine.trim();
		if (!trimmed || trimmed.startsWith("#")) return;

		const { body, description } = splitComment(rawLine);
		const separator = body.indexOf(":");
		if (separator === -1) {
			errors.push({
				line,
				message: "Expected `name: type`, with a colon after the field name.",
			});
			return;
		}

		const name = body.slice(0, separator).trim();
		if (!name) {
			errors.push({ line, message: "Name is required." });
			return;
		}
		const firstSeen = namesSeen.get(name);
		if (firstSeen !== undefined) {
			errors.push({
				line,
				message: `\`${name}\` is already declared on line ${firstSeen}.`,
			});
			return;
		}
		namesSeen.set(name, line);

		const tokens = tokenize(body.slice(separator + 1));
		const typeKeyword = tokens.shift()?.toLowerCase();
		if (!typeKeyword) {
			errors.push({
				line,
				message: `\`${name}\` has no type. One of: ${DSL_TYPE_KEYWORDS.join(", ")}.`,
			});
			return;
		}
		const type = TYPE_KEYWORDS[typeKeyword];
		if (!type) {
			errors.push({
				line,
				message: `\`${typeKeyword}\` is not a field type. One of: ${DSL_TYPE_KEYWORDS.join(", ")}.`,
			});
			return;
		}

		rows.push(
			newFieldRow({
				name,
				type,
				description: description ?? "",
				rules: parseConstraints(type, tokens, line, errors),
			}),
		);
	});

	return { rows, errors };
}

function constraintsToDsl(row: FieldRow): string {
	const { rules, type } = row;

	if (type === LicenseFieldType.BOOLEAN) return "";
	if (type === LicenseFieldType.ENUM) return (rules.values ?? []).join(" | ");

	if (type === LicenseFieldType.LIMIT || type === LicenseFieldType.NUMBER) {
		if (rules.min === undefined && rules.max === undefined) return "";
		return `${rules.min ?? ""}..${rules.max ?? ""}`;
	}

	const parts: string[] = [];
	if (rules.pattern) parts.push(`/${rules.pattern.replace(/\//g, "\\/")}/`);
	if (rules.min_length !== undefined || rules.max_length !== undefined) {
		parts.push(`len ${rules.min_length ?? ""}..${rules.max_length ?? ""}`);
	}
	return parts.join(" ");
}

/**
 * Renders a draft back to source. Names are padded to a common width so the
 * type column lines up, which is what makes a schema readable as a block.
 *
 * A row with no name yet has no source form — a fresh draft holds one, and
 * writing it out as `: string` would put a parse error in front of an operator
 * who has typed nothing.
 */
export function serializeSchemaDsl(allRows: FieldRow[]): string {
	const rows = allRows.filter((row) => row.name.trim() !== "");
	const nameWidth = rows.reduce(
		(widest, row) => Math.max(widest, row.name.trim().length),
		0,
	);

	return rows
		.map((row) => {
			const name = `${row.name.trim()}:`.padEnd(nameWidth + 2);
			const constraints = constraintsToDsl(row);
			const declaration =
				`${name}${TYPE_KEYWORD_BY_TYPE[row.type]} ${constraints}`.trimEnd();
			const description = row.description?.trim();
			return description ? `${declaration}  # ${description}` : declaration;
		})
		.join("\n");
}
