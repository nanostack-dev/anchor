import { LicenseUsageStatus } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ChevronDown } from "lucide-react";
import { useState } from "react";
import { LicenseValueFields } from "../LicenseValueFields";
import { OrganizationLicenseHistoryView } from "../OrganizationLicenseHistoryView";
import {
	UsageHistoryChartView,
	type UsageRangeValue,
} from "../UsageHistoryChartView";
import {
	formatUsageNumber,
	usageBarPercent,
	usageStatusLabel,
	usageStatusTone,
} from "../license-usage-status";
import { SERIES_POINTS } from "./license-detail-fixture";
import type { LicenseDetailLayoutProps } from "./types";

const barTone: Record<LicenseUsageStatus, string> = {
	[LicenseUsageStatus.WITHIN_LIMIT]: "bg-success",
	[LicenseUsageStatus.AT_LIMIT]: "bg-warning",
	[LicenseUsageStatus.EXCEEDED]: "bg-destructive",
	[LicenseUsageStatus.STALE]: "bg-muted-foreground/30",
};

/**
 * Candidate B — tabs, and every limit still listed; the chart opens in place.
 *
 * Keeps the scan that the dropdown gives up: twenty statuses are readable
 * without opening anything. The chart appears under the row that was clicked,
 * so it never scrolls away from the thing it describes, and its series is
 * fetched only on that click.
 */
export function DetailTabbedList({
	usage,
	fields,
	values,
	history,
	templateName,
	onLoadSeries,
	onLoadHistory,
}: LicenseDetailLayoutProps) {
	const limits = Object.keys(usage).sort();
	const [openField, setOpenField] = useState<string | null>(null);
	const [range, setRange] = useState<UsageRangeValue>("7d");
	const [historyOpened, setHistoryOpened] = useState(false);

	const toggle = (name: string) => {
		const next = openField === name ? null : name;
		setOpenField(next);
		if (next) onLoadSeries?.(next);
	};

	const exceeded = limits.filter(
		(name) => usage[name].status === LicenseUsageStatus.EXCEEDED,
	).length;

	return (
		<Tabs
			defaultValue="usage"
			onValueChange={(value) => {
				if (value === "history" && !historyOpened) {
					setHistoryOpened(true);
					onLoadHistory?.();
				}
			}}
		>
			<TabsList>
				<TabsTrigger value="usage">
					Limits
					{exceeded > 0 && (
						<StatusBadge tone="destructive" className="ml-2">
							{exceeded} over
						</StatusBadge>
					)}
				</TabsTrigger>
				<TabsTrigger value="history">Change history</TabsTrigger>
				<TabsTrigger value="values">All values</TabsTrigger>
			</TabsList>

			<TabsContent value="usage" className="pt-4">
				<ul className="divide-y divide-border rounded-lg border border-border">
					{limits.map((name) => {
						const entry = usage[name];
						const open = openField === name;
						return (
							<li key={name}>
								<button
									type="button"
									aria-expanded={open}
									onClick={() => toggle(name)}
									className="flex w-full items-center gap-3 px-3 py-2.5 text-left transition-colors hover:bg-muted/50"
								>
									<ChevronDown
										aria-hidden
										className={`size-4 shrink-0 text-muted-foreground transition-transform ${open ? "rotate-180" : ""}`}
									/>
									<span className="min-w-0 flex-1">
										<span className="block truncate font-mono text-xs text-muted-foreground">
											{name}
										</span>
										<span className="mt-1 flex items-baseline gap-1.5">
											<span className="text-sm font-semibold tabular-nums">
												{entry.usage === undefined
													? "—"
													: formatUsageNumber(entry.usage)}
											</span>
											<span className="text-xs text-muted-foreground tabular-nums">
												of {formatUsageNumber(entry.limit)}
											</span>
										</span>
										<span className="mt-1.5 block h-1 w-full overflow-hidden rounded-full bg-muted">
											<span
												className={`block h-full rounded-full ${barTone[entry.status]}`}
												style={{
													width: `${usageBarPercent(entry.usage ?? 0, entry.limit)}%`,
												}}
											/>
										</span>
									</span>
									<StatusBadge tone={usageStatusTone(entry.status)}>
										{usageStatusLabel(entry.status)}
									</StatusBadge>
								</button>

								{open && (
									<div className="border-t border-border bg-muted/30 p-3">
										<UsageHistoryChartView
											field={name}
											limit={entry.limit}
											rangeValue={range}
											onRangeChange={setRange}
											points={SERIES_POINTS}
										/>
									</div>
								)}
							</li>
						);
					})}
				</ul>
			</TabsContent>

			<TabsContent value="history" className="pt-4">
				<OrganizationLicenseHistoryView
					items={history}
					total={history.length}
					templateName={templateName}
				/>
			</TabsContent>

			<TabsContent value="values" className="pt-4">
				<LicenseValueFields fields={fields} values={values} />
			</TabsContent>
		</Tabs>
	);
}
