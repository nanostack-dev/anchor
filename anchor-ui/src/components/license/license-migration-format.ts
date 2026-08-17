import {
	type LicenseFieldResponse,
	LicenseMigrationOutcome,
	LicenseMigrationSkipReason,
	type LicenseTemplateValues,
	type OrganizationLicenseSummaryResponse,
} from "@/client";
import type { StatusTone } from "@/components/common/StatusBadge";

/**
 * How one license field differs between two sets of values.
 *
 * `carried` marks a field the organization holds its own value for, which a
 * migration keeps unless differences are discarded. It is computed against the
 * template the organization holds today, never against the target — a value
 * only counts as its own if it has come apart from its own tier.
 */
export interface ValueChange {
	field: string;
	type: LicenseFieldResponse["type"] | undefined;
	from: unknown;
	to: unknown;
	carried?: boolean;
}

function sameValue(a: unknown, b: unknown): boolean {
	if (typeof a === "number" || typeof b === "number") {
		return Number(a) === Number(b);
	}
	return a === b;
}

function fieldType(
	fields: LicenseFieldResponse[],
	name: string,
): LicenseFieldResponse["type"] | undefined {
	return fields.find((field) => field.name === name)?.type;
}

/**
 * Every license field on which two templates disagree, ordered by name.
 *
 * This is what an operator reads before a migration: Anchor has no preview, and
 * the honest question before moving a cohort between tiers is how the two tiers
 * differ. Fields present in only one of the two are included, because the
 * target's declaration is what the migrated license ends up carrying.
 */
export function templateValueChanges(
	fromValues: LicenseTemplateValues | undefined,
	toValues: LicenseTemplateValues | undefined,
	fields: LicenseFieldResponse[],
): ValueChange[] {
	const from = fromValues ?? {};
	const to = toValues ?? {};
	const names = [...new Set([...Object.keys(from), ...Object.keys(to)])].sort();

	return names
		.filter((name) => !sameValue(from[name], to[name]))
		.map((name) => ({
			field: name,
			type: fieldType(fields, name),
			from: from[name],
			to: to[name],
		}));
}

/**
 * The license fields this organization holds its own value for: those that
 * differ from the template it is on today and that the target template also
 * declares. Only these survive a carry-forward migration — a value the target
 * does not name belongs to a field the schema no longer declares.
 */
export function carriedForwardChanges(
	summary: OrganizationLicenseSummaryResponse,
	currentTemplateValues: LicenseTemplateValues | undefined,
	targetValues: LicenseTemplateValues | undefined,
	fields: LicenseFieldResponse[],
): ValueChange[] {
	const held = summary.license?.values ?? {};
	const current = currentTemplateValues ?? {};
	const target = targetValues ?? {};

	return Object.keys(target)
		.filter((name) => name in held && !sameValue(held[name], current[name]))
		.sort()
		.map((name) => ({
			field: name,
			type: fieldType(fields, name),
			from: target[name],
			to: held[name],
			carried: true,
		}));
}

/** Whether this organization's license has come apart from its own tier. */
export function differsFromItsTemplate(
	summary: OrganizationLicenseSummaryResponse,
	templateValues: LicenseTemplateValues | undefined,
): boolean | undefined {
	if (!summary.license) return false;
	// Undefined is not an empty template. Answering "differs" while the
	// templates are still loading marks every customer adjusted, and answering
	// it for a template that resolved to nothing marks one adjusted forever.
	if (templateValues === undefined) return undefined;
	const held = summary.license.values ?? {};
	const names = new Set([...Object.keys(held), ...Object.keys(templateValues)]);
	return [...names].some(
		(name) => !sameValue(held[name], templateValues[name]),
	);
}

export const OUTCOME_LABELS: Record<LicenseMigrationOutcome, string> = {
	[LicenseMigrationOutcome.MIGRATED]: "Moved",
	[LicenseMigrationOutcome.UNCHANGED]: "Already there",
	[LicenseMigrationOutcome.SKIPPED]: "Skipped",
	[LicenseMigrationOutcome.FAILED]: "Failed",
};

export const OUTCOME_TONES: Record<LicenseMigrationOutcome, StatusTone> = {
	[LicenseMigrationOutcome.MIGRATED]: "success",
	[LicenseMigrationOutcome.UNCHANGED]: "neutral",
	[LicenseMigrationOutcome.SKIPPED]: "warning",
	[LicenseMigrationOutcome.FAILED]: "destructive",
};

export const SKIP_REASON_LABELS: Record<LicenseMigrationSkipReason, string> = {
	[LicenseMigrationSkipReason.NOT_LICENSED]:
		"Holds no license — instantiate one first",
};
