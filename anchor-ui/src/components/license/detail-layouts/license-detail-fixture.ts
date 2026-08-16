import {
	LicenseChangeType,
	type LicenseFieldResponse,
	LicenseFieldType,
	type LicenseFieldUsageResponse,
	type LicenseTemplateValues,
	LicenseUsageStatus,
	type OrganizationLicenseChangeResponse,
} from "@/client";

/**
 * A product with twenty declared limits, which is the case the single inline
 * chart was never designed for. Every layout candidate is judged against this
 * one fixture so the comparison is about arrangement and nothing else.
 */

const minutesAgo = (minutes: number) =>
	new Date(Date.now() - minutes * 60_000).toISOString();

const LIMIT_NAMES = [
	"max_flows",
	"max_members",
	"max_schedules",
	"max_webhooks",
	"max_collections",
	"max_environments",
	"max_api_keys",
	"max_workspaces",
	"monthly_runs",
	"monthly_webhook_deliveries",
	"monthly_ai_tokens",
	"concurrent_runs",
	"max_flow_versions",
	"max_retained_executions",
	"max_secrets",
	"max_custom_domains",
	"max_saved_queries",
	"max_alert_rules",
	"max_seats_per_workspace",
	"max_scheduled_run_fanout",
];

function statusFor(index: number): LicenseUsageStatus {
	if (index % 7 === 3) return LicenseUsageStatus.EXCEEDED;
	if (index % 5 === 2) return LicenseUsageStatus.AT_LIMIT;
	if (index % 4 === 1) return LicenseUsageStatus.STALE;
	return LicenseUsageStatus.WITHIN_LIMIT;
}

export const MANY_LIMITS: Record<string, LicenseFieldUsageResponse> =
	Object.fromEntries(
		LIMIT_NAMES.map((name, index) => {
			const limit = [10, 25, 100, 500, 2000, 250000][index % 6];
			const status = statusFor(index);
			if (status === LicenseUsageStatus.STALE) {
				return [name, { limit, status } satisfies LicenseFieldUsageResponse];
			}
			const usage =
				status === LicenseUsageStatus.EXCEEDED
					? Math.round(limit * 1.3)
					: status === LicenseUsageStatus.AT_LIMIT
						? limit
						: Math.round(limit * (0.2 + (index % 5) * 0.13));
			return [
				name,
				{
					limit,
					usage,
					status,
					last_reported_at: minutesAgo(3 + index * 11),
				} satisfies LicenseFieldUsageResponse,
			];
		}),
	);

export const FEW_LIMITS: Record<string, LicenseFieldUsageResponse> =
	Object.fromEntries(
		LIMIT_NAMES.slice(0, 3).map((name) => [name, MANY_LIMITS[name]]),
	);

export const SERIES_POINTS = Array.from({ length: 14 }, (_, day) => ({
	bucket: new Date(Date.now() - (13 - day) * 86_400_000).toISOString(),
	value: Math.round(180 + Math.sin(day / 2) * 90 + day * 12),
}));

export const SCHEMA_FIELDS: LicenseFieldResponse[] = [
	...LIMIT_NAMES.map((name, index) => ({
		id: `lfld_${index}`,
		name,
		type: LicenseFieldType.LIMIT,
		rules: {},
		created_at: minutesAgo(60_000),
		updated_at: minutesAgo(60_000),
	})),
	{
		id: "lfld_sso",
		name: "sso",
		type: LicenseFieldType.BOOLEAN,
		rules: {},
		created_at: minutesAgo(60_000),
		updated_at: minutesAgo(60_000),
	},
	{
		id: "lfld_support",
		name: "support_tier",
		type: LicenseFieldType.ENUM,
		rules: {},
		created_at: minutesAgo(60_000),
		updated_at: minutesAgo(60_000),
	},
	{
		id: "lfld_region",
		name: "region",
		type: LicenseFieldType.STRING,
		rules: {},
		created_at: minutesAgo(60_000),
		updated_at: minutesAgo(60_000),
	},
];

export const VALUES: LicenseTemplateValues = {
	...Object.fromEntries(
		LIMIT_NAMES.map((name) => [name, MANY_LIMITS[name].limit]),
	),
	sso: true,
	support_tier: "priority",
	region: "eu-west",
};

const changeBase = {
	product_id: "prd_sandbox",
	organization_id: "org_acme",
	license_id: "lic_acme",
};

export const HISTORY: OrganizationLicenseChangeResponse[] = [
	{
		...changeBase,
		id: "lchg_migrated",
		type: LicenseChangeType.MIGRATED,
		template_id: "ltpl_enterprise",
		previous_template_id: "ltpl_pro",
		old_value: { max_flows: 500, max_members: 25 },
		new_value: { max_flows: 1200, max_members: 40, max_schedules: 2000 },
		changed_at: minutesAgo(20),
	},
	{
		...changeBase,
		id: "lchg_adjust_flows",
		type: LicenseChangeType.ADJUSTED,
		field: "max_flows",
		old_value: 500,
		new_value: 1200,
		changed_at: minutesAgo(4300),
	},
	{
		...changeBase,
		id: "lchg_adjust_members",
		type: LicenseChangeType.ADJUSTED,
		field: "max_members",
		old_value: 25,
		new_value: 40,
		changed_at: minutesAgo(4300),
	},
	{
		...changeBase,
		id: "lchg_instantiated",
		type: LicenseChangeType.INSTANTIATED,
		template_id: "ltpl_pro",
		new_value: { max_flows: 500, max_members: 25, sso: true },
		changed_at: minutesAgo(52_000),
	},
];

export const TEMPLATE_NAMES: Record<string, string> = {
	ltpl_pro: "Pro",
	ltpl_enterprise: "Enterprise",
};

export const templateName = (id: string) => TEMPLATE_NAMES[id] ?? id;
