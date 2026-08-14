import { LicenseUsageStatus } from "@/client";
import type { StatusTone } from "@/components/common/StatusBadge";

const statusLabels: Record<LicenseUsageStatus, string> = {
	[LicenseUsageStatus.WITHIN_LIMIT]: "Within limit",
	[LicenseUsageStatus.AT_LIMIT]: "At limit",
	[LicenseUsageStatus.EXCEEDED]: "Exceeded",
	[LicenseUsageStatus.STALE]: "Never reported",
};

const statusTones: Record<LicenseUsageStatus, StatusTone> = {
	[LicenseUsageStatus.WITHIN_LIMIT]: "success",
	[LicenseUsageStatus.AT_LIMIT]: "warning",
	[LicenseUsageStatus.EXCEEDED]: "destructive",
	[LicenseUsageStatus.STALE]: "neutral",
};

const statusExplanations: Record<LicenseUsageStatus, string> = {
	[LicenseUsageStatus.WITHIN_LIMIT]:
		"The latest reported usage is below this limit.",
	[LicenseUsageStatus.AT_LIMIT]:
		"The latest reported usage has reached this limit exactly.",
	[LicenseUsageStatus.EXCEEDED]:
		"The latest reported usage is above this limit. Anchor records this and never blocks on it.",
	[LicenseUsageStatus.STALE]:
		"Nothing has been reported against this limit, so there is no current number to trust.",
};

export function usageStatusLabel(status: LicenseUsageStatus) {
	return statusLabels[status] ?? status;
}

export function usageStatusTone(status: LicenseUsageStatus) {
	return statusTones[status] ?? "neutral";
}

export function usageStatusExplanation(status: LicenseUsageStatus) {
	return statusExplanations[status] ?? "";
}

/**
 * Pinned to en-US rather than the viewer's locale: en-GB abbreviates a million
 * as a lowercase "m", which reads as milli- next to a raw count, and a
 * locale-dependent rendering also makes any assertion on these strings pass or
 * fail by machine.
 */
const compactNumber = new Intl.NumberFormat("en-US", {
	notation: "compact",
	maximumFractionDigits: 1,
});

const preciseNumber = new Intl.NumberFormat("en-US", {
	maximumFractionDigits: 2,
});

export function formatUsageNumber(value: number) {
	return Math.abs(value) >= 10_000
		? compactNumber.format(value)
		: preciseNumber.format(value);
}

export function formatExactNumber(value: number) {
	return preciseNumber.format(value);
}

/**
 * Clamped so a limit of zero, or usage far past the limit, still yields a bar
 * width between 0 and 100 rather than a division by zero or an overflow.
 */
export function usageBarPercent(usage: number, limit: number) {
	if (!Number.isFinite(limit) || limit <= 0) {
		return usage > 0 ? 100 : 0;
	}
	return Math.min(100, Math.max(0, (usage / limit) * 100));
}
