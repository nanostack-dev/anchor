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

export function changeTypeLabel(type: LicenseChangeType): string {
	switch (type) {
		case LicenseChangeType.INSTANTIATED:
			return "Instantiated";
		case LicenseChangeType.ADJUSTED:
			return "Adjusted";
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
