import {
	EntitlementType,
	type EntitlementValue,
	type LicenseResponse,
	LicenseStatus,
	type PlanResponse,
} from "@/client";
import { StatusBadge, type StatusTone } from "@/components/common/StatusBadge";
import { Badge } from "@/components/ui/badge";
import {
	Sheet,
	SheetContent,
	SheetDescription,
	SheetHeader,
	SheetTitle,
	SheetTrigger,
} from "@/components/ui/sheet";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import dayjs from "dayjs";
import type { ReactElement } from "react";

export const licenseStatusTone: Record<LicenseStatus, StatusTone> = {
	[LicenseStatus.ACTIVE]: "success",
	[LicenseStatus.SUSPENDED]: "warning",
	[LicenseStatus.REVOKED]: "destructive",
};

const formatEntitlementValue = (
	entitlement: EntitlementValue | undefined,
): string => {
	if (!entitlement) return "—";
	if (entitlement.type === EntitlementType.BOOLEAN) {
		return entitlement.value === true ? "Enabled" : "Disabled";
	}
	return String(entitlement.value);
};

const formatDate = (value: string | null | undefined): string =>
	value ? dayjs(value).format("D MMMM YYYY H:mm") : "—";

interface ResolvedEntitlement {
	key: string;
	planValue?: EntitlementValue;
	overrideValue?: EntitlementValue;
	effective: EntitlementValue;
}

/**
 * Client-side view of the resolution chain the token service applies:
 * plan defaults first, then per-organization overrides replace matching
 * keys.
 */
function resolveEntitlements(
	license: LicenseResponse,
	plan: PlanResponse | undefined,
): ResolvedEntitlement[] {
	const keys = new Set<string>([
		...Object.keys(plan?.entitlements ?? {}),
		...Object.keys(license.entitlement_overrides),
	]);
	return [...keys].sort().map((key) => {
		const planValue = plan?.entitlements[key];
		const overrideValue = license.entitlement_overrides[key];
		const effective = overrideValue ?? planValue;
		return {
			key,
			planValue,
			overrideValue,
			// One of the two is always defined because the key came from a map.
			effective: effective as EntitlementValue,
		};
	});
}

interface LicenseDetailSheetProps {
	license: LicenseResponse;
	plan?: PlanResponse;
	organizationName?: string;
	trigger: ReactElement;
}

export function LicenseDetailSheet({
	license,
	plan,
	organizationName,
	trigger,
}: LicenseDetailSheetProps) {
	const resolved = resolveEntitlements(license, plan);
	const overrideCount = Object.keys(license.entitlement_overrides).length;

	const summaryRows: Array<{ label: string; value: ReactElement | string }> = [
		{
			label: "Organization",
			value: organizationName ?? license.organization_id,
		},
		{
			label: "Plan",
			value: plan ? `${plan.name} (${plan.key})` : license.plan_id,
		},
		{
			label: "Status",
			value: (
				<StatusBadge tone={licenseStatusTone[license.status]}>
					{license.status}
				</StatusBadge>
			),
		},
		{ label: "Expires at", value: formatDate(license.expires_at) },
		{ label: "Grace until", value: formatDate(license.grace_until) },
		{
			label: "Token TTL",
			value: `${license.token_ttl_seconds.toLocaleString()} seconds`,
		},
		{ label: "Created", value: formatDate(license.created_at) },
		{ label: "Updated", value: formatDate(license.updated_at) },
	];

	return (
		<Sheet>
			<SheetTrigger render={trigger} />
			<SheetContent side="right" className="w-full overflow-y-auto sm:max-w-xl">
				<SheetHeader>
					<SheetTitle>License Details</SheetTitle>
					<SheetDescription>
						Resolved entitlements are the plan defaults with the {overrideCount}{" "}
						per-organization override
						{overrideCount === 1 ? "" : "s"} applied — exactly what issued
						license tokens carry.
					</SheetDescription>
				</SheetHeader>
				<div className="flex flex-col gap-6 px-4 pb-6">
					<dl className="grid grid-cols-[auto_1fr] items-center gap-x-6 gap-y-2 text-sm">
						{summaryRows.map((row) => (
							<div key={row.label} className="contents">
								<dt className="font-medium text-muted-foreground">
									{row.label}
								</dt>
								<dd className="min-w-0 truncate text-right">{row.value}</dd>
							</div>
						))}
					</dl>
					<div className="flex flex-col gap-2">
						<h3 className="text-sm font-medium">Resolved entitlements</h3>
						{resolved.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								Neither the plan nor this license defines any entitlements.
							</p>
						) : (
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead>Key</TableHead>
										<TableHead>Plan</TableHead>
										<TableHead>Override</TableHead>
										<TableHead>Effective</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{resolved.map((entitlement) => (
										<TableRow key={entitlement.key}>
											<TableCell className="font-mono text-xs">
												{entitlement.key}
											</TableCell>
											<TableCell className="text-sm text-muted-foreground">
												{formatEntitlementValue(entitlement.planValue)}
											</TableCell>
											<TableCell className="text-sm text-muted-foreground">
												{formatEntitlementValue(entitlement.overrideValue)}
											</TableCell>
											<TableCell>
												<div className="flex items-center gap-2 text-sm">
													{formatEntitlementValue(entitlement.effective)}
													{entitlement.overrideValue && (
														<Badge variant="outline">Overridden</Badge>
													)}
												</div>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						)}
					</div>
				</div>
			</SheetContent>
		</Sheet>
	);
}
