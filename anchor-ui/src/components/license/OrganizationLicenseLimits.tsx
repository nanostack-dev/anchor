import { type LicenseFieldUsageResponse, LicenseUsageStatus } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { cn } from "@/lib/utils";
import dayjs from "dayjs";
import relativeTime from "dayjs/plugin/relativeTime";
import {
	formatExactNumber,
	formatUsageNumber,
	usageBarPercent,
	usageStatusLabel,
	usageStatusTone,
} from "./license-usage-status";

dayjs.extend(relativeTime);

export interface OrganizationLicenseLimitsProps {
	usage: Record<string, LicenseFieldUsageResponse>;
	selectedField: string | null;
	onSelectField: (field: string) => void;
}

const barToneClasses: Record<LicenseUsageStatus, string> = {
	[LicenseUsageStatus.WITHIN_LIMIT]: "bg-success",
	[LicenseUsageStatus.AT_LIMIT]: "bg-warning",
	[LicenseUsageStatus.EXCEEDED]: "bg-destructive",
	[LicenseUsageStatus.STALE]: "bg-muted-foreground/30",
};

export function OrganizationLicenseLimits({
	usage,
	selectedField,
	onSelectField,
}: OrganizationLicenseLimitsProps) {
	const limits = Object.entries(usage).sort(([a], [b]) => a.localeCompare(b));

	if (limits.length === 0) {
		return (
			<p className="text-sm text-muted-foreground">
				This product&rsquo;s license schema declares no limit fields, so there
				is no usage to measure.
			</p>
		);
	}

	return (
		<ul className="divide-y divide-border rounded-lg border border-border">
			{limits.map(([field, fieldUsage]) => {
				const isSelected = field === selectedField;
				const hasUsage = typeof fieldUsage.usage === "number";
				const barPercent = hasUsage
					? usageBarPercent(fieldUsage.usage as number, fieldUsage.limit)
					: 0;

				return (
					<li key={field}>
						<button
							type="button"
							onClick={() => onSelectField(field)}
							aria-pressed={isSelected}
							className={cn(
								"flex w-full flex-col gap-2 p-3 text-left transition-colors",
								"hover:bg-accent/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-inset",
								isSelected && "bg-accent",
							)}
						>
							<div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
								<span className="font-mono text-sm">{field}</span>
								<StatusBadge tone={usageStatusTone(fieldUsage.status)}>
									{usageStatusLabel(fieldUsage.status)}
								</StatusBadge>
							</div>

							<div className="flex items-baseline gap-1.5 tabular-nums">
								{hasUsage ? (
									<>
										<span
											className="text-lg font-semibold"
											title={formatExactNumber(fieldUsage.usage as number)}
										>
											{formatUsageNumber(fieldUsage.usage as number)}
										</span>
										<span className="text-sm text-muted-foreground">
											of {formatUsageNumber(fieldUsage.limit)}
										</span>
									</>
								) : (
									<>
										<span className="text-lg font-semibold text-muted-foreground">
											—
										</span>
										<span className="text-sm text-muted-foreground">
											limit {formatUsageNumber(fieldUsage.limit)}
										</span>
									</>
								)}
							</div>

							<div
								className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
								role="img"
								aria-label={
									hasUsage
										? `${formatExactNumber(fieldUsage.usage as number)} of ${formatExactNumber(fieldUsage.limit)} used`
										: `Nothing reported against a limit of ${formatExactNumber(fieldUsage.limit)}`
								}
							>
								<div
									className={cn(
										"h-full rounded-full transition-[width] duration-200",
										barToneClasses[fieldUsage.status],
									)}
									style={{ width: `${barPercent}%` }}
								/>
							</div>

							{fieldUsage.last_reported_at && (
								<span
									className="text-xs text-muted-foreground"
									title={dayjs(fieldUsage.last_reported_at).format(
										"D MMMM YYYY HH:mm",
									)}
								>
									Reported {dayjs(fieldUsage.last_reported_at).fromNow()}
								</span>
							)}
						</button>
					</li>
				);
			})}
		</ul>
	);
}
