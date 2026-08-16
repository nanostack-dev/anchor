import { LicenseUsageStatus } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useEffect, useState } from "react";
import { LicenseValueFields } from "../LicenseValueFields";
import { OrganizationLicenseHistoryView } from "../OrganizationLicenseHistoryView";
import {
	UsageHistoryChartView,
	type UsageRangeValue,
} from "../UsageHistoryChartView";
import { usageStatusLabel, usageStatusTone } from "../license-usage-status";
import { SERIES_POINTS } from "./license-detail-fixture";
import type { LicenseDetailLayoutProps } from "./types";

/**
 * Candidate A — tabs, and one limit at a time behind a dropdown.
 *
 * The most compact of the three: the page is never longer than one chart, and
 * twenty limits cost twenty dropdown entries rather than twenty rows. What it
 * gives up is the scan — an operator asking "is this customer near anything"
 * has to open the dropdown and read it as a menu.
 */
export function DetailTabbedSelect({
	usage,
	fields,
	values,
	history,
	templateName,
	onLoadSeries,
	onLoadHistory,
}: LicenseDetailLayoutProps) {
	const limits = Object.keys(usage).sort();
	const [field, setField] = useState<string>(limits[0] ?? "");
	const [range, setRange] = useState<UsageRangeValue>("7d");
	const [historyOpened, setHistoryOpened] = useState(false);

	useEffect(() => {
		if (field) onLoadSeries?.(field);
	}, [field, onLoadSeries]);

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
					Usage
					{exceeded > 0 && (
						<StatusBadge tone="destructive" className="ml-2">
							{exceeded} over
						</StatusBadge>
					)}
				</TabsTrigger>
				<TabsTrigger value="history">Change history</TabsTrigger>
				<TabsTrigger value="values">All values</TabsTrigger>
			</TabsList>

			<TabsContent value="usage" className="flex flex-col gap-4 pt-4">
				<div className="flex max-w-sm flex-col gap-2">
					<label htmlFor="limit-select" className="text-sm font-medium">
						Limit
					</label>
					<Select
						items={limits.map((name) => ({ value: name, label: name }))}
						value={field}
						onValueChange={(value) => setField(value ?? "")}
					>
						<SelectTrigger id="limit-select">
							<SelectValue placeholder="Select a limit..." />
						</SelectTrigger>
						<SelectContent>
							{limits.map((name) => (
								<SelectItem key={name} value={name}>
									<span className="flex w-full items-center justify-between gap-3">
										<span className="font-mono text-xs">{name}</span>
										<StatusBadge tone={usageStatusTone(usage[name].status)}>
											{usageStatusLabel(usage[name].status)}
										</StatusBadge>
									</span>
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				</div>

				{field && (
					<UsageHistoryChartView
						field={field}
						limit={usage[field].limit}
						rangeValue={range}
						onRangeChange={setRange}
						points={SERIES_POINTS}
					/>
				)}
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
