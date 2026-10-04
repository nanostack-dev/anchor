import { type LicenseFieldUsageResponse, LicenseUsageStatus } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { cn } from "@/lib/utils";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { SlidersHorizontalIcon as SlidersHorizontal } from "@phosphor-icons/react";
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
	adjustedFields?: readonly string[];
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
	adjustedFields = [],
	selectedField,
	onSelectField,
}: OrganizationLicenseLimitsProps) {
	const limits = Object.entries(usage).sort(([a], [b]) => a.localeCompare(b));

	if (limits.length === 0) {
		return (
			<Text tone="muted">
				This product&rsquo;s license schema declares no limit fields, so there
				is no usage to measure.
			</Text>
		);
	}

	return (
		<Box
			as="ul"
			className="divide-y divide-border overflow-hidden rounded-lg border border-border"
		>
			{limits.map(([field, fieldUsage]) => {
				const isSelected = field === selectedField;
				const isCustomized = adjustedFields.includes(field);
				const hasUsage = typeof fieldUsage.usage === "number";
				const barPercent = hasUsage
					? usageBarPercent(fieldUsage.usage as number, fieldUsage.limit)
					: 0;

				return (
					<li key={field}>
						<button
							className={cn(
								"flex w-full flex-col gap-2 p-4 text-left transition-colors focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-ring/30 focus-visible:ring-inset",
								isSelected
									? "bg-accent"
									: isCustomized
										? "bg-accent/50 hover:bg-accent"
										: "hover:bg-accent/50",
							)}
							type="button"
							onClick={() => onSelectField(field)}
							aria-pressed={isSelected}
						>
							<Box className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
								<Box as="span" className="break-all font-mono text-sm">
									{field}
								</Box>
								<StatusBadge tone={usageStatusTone(fieldUsage.status)}>
									{usageStatusLabel(fieldUsage.status)}
								</StatusBadge>
							</Box>

							<Box className="flex flex-wrap items-baseline gap-1.5 tabular-nums">
								{hasUsage ? (
									<>
										<Text
											as="span"
											size="lg"
											weight="semibold"
											title={formatExactNumber(fieldUsage.usage as number)}
										>
											{formatUsageNumber(fieldUsage.usage as number)}
										</Text>
										<Text as="span" tone="muted">
											of {formatUsageNumber(fieldUsage.limit)}
										</Text>
									</>
								) : (
									<>
										<Text as="span" size="lg" tone="muted" weight="semibold">
											—
										</Text>
										<Text as="span" tone="muted">
											limit {formatUsageNumber(fieldUsage.limit)}
										</Text>
									</>
								)}
								{isCustomized && (
									<Box
										as="span"
										className="ml-auto inline-flex items-center gap-1.5 text-xs font-medium text-accent-foreground"
									>
										<SlidersHorizontal
											aria-hidden
											className="size-3.5 shrink-0"
										/>
										Custom limit
									</Box>
								)}
							</Box>

							<Box
								className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
								role="img"
								aria-label={
									hasUsage
										? `${formatExactNumber(fieldUsage.usage as number)} of ${formatExactNumber(fieldUsage.limit)} used`
										: `Nothing reported against a limit of ${formatExactNumber(fieldUsage.limit)}`
								}
							>
								<Box
									className={cn(
										"h-full rounded-full transition-[width] duration-200",
										barToneClasses[fieldUsage.status],
									)}
									style={{ width: `${barPercent}%` }}
								/>
							</Box>

							{fieldUsage.last_reported_at && (
								<Text
									as="span"
									size="xs"
									tone="muted"
									title={dayjs(fieldUsage.last_reported_at).format(
										"D MMMM YYYY HH:mm",
									)}
								>
									Reported {dayjs(fieldUsage.last_reported_at).fromNow()}
								</Text>
							)}
						</button>
					</li>
				);
			})}
		</Box>
	);
}
