import {
	LicenseChangeType,
	type OrganizationLicenseChangeResponse,
} from "@/client";

export function formatHistoryValue(value: unknown): string {
	if (value === null || value === undefined) return "—";
	if (typeof value === "boolean") return value ? "Yes" : "No";
	if (typeof value === "number") return String(value);
	if (typeof value === "string") return value;
	return JSON.stringify(value);
}

export function asValueSet(value: unknown): Record<string, unknown> | null {
	if (value === null || value === undefined || typeof value !== "object") {
		return null;
	}
	if (Array.isArray(value)) return null;
	return value as Record<string, unknown>;
}

/**
 * A `SET` entry covers both a tier move and a first license granted through
 * the migrate route, so the label needs `previous_template_id` alongside
 * `type` to tell which happened — `type` alone no longer says.
 */
export function changeTypeLabel(
	entry: Pick<
		OrganizationLicenseChangeResponse,
		"type" | "previous_template_id"
	>,
): string {
	switch (entry.type) {
		case LicenseChangeType.INSTANTIATED:
			return "Instantiated";
		case LicenseChangeType.ADJUSTED:
			return "Adjusted";
		case LicenseChangeType.SET:
			return entry.previous_template_id
				? "Moved to another tier"
				: "Licensed for the first time";
	}
}

/** Consecutive entries that share a timestamp are one adjustment. */
export function groupHistoryByMoment(
	items: OrganizationLicenseChangeResponse[],
): OrganizationLicenseChangeResponse[][] {
	const groups: OrganizationLicenseChangeResponse[][] = [];
	for (const item of items) {
		const last = groups.at(-1);
		if (last && last[0].changed_at === item.changed_at) {
			last.push(item);
			continue;
		}
		groups.push([item]);
	}
	return groups;
}
