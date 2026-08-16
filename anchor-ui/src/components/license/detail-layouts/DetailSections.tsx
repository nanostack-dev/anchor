import { LicenseUsageStatus } from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Button } from "@/components/ui/button";
import { ChevronDown } from "lucide-react";
import { type ReactNode, useState } from "react";
import { LicenseValueFields } from "../LicenseValueFields";
import { OrganizationLicenseHistoryView } from "../OrganizationLicenseHistoryView";
import {
	UsageHistoryChartView,
	type UsageRangeValue,
} from "../UsageHistoryChartView";
import {
	formatUsageNumber,
	usageStatusLabel,
	usageStatusTone,
} from "../license-usage-status";
import { SERIES_POINTS } from "./license-detail-fixture";
import type { LicenseDetailLayoutProps } from "./types";

function Section({
	title,
	summary,
	defaultOpen = false,
	onFirstOpen,
	children,
}: {
	title: string;
	summary: string;
	defaultOpen?: boolean;
	onFirstOpen?: () => void;
	children: ReactNode;
}) {
	const [open, setOpen] = useState(defaultOpen);
	const [opened, setOpened] = useState(defaultOpen);

	return (
		<section className="rounded-lg border border-border">
			<button
				type="button"
				aria-expanded={open}
				onClick={() => {
					const next = !open;
					setOpen(next);
					if (next && !opened) {
						setOpened(true);
						onFirstOpen?.();
					}
				}}
				className="flex w-full items-center gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/50"
			>
				<ChevronDown
					aria-hidden
					className={`size-4 shrink-0 text-muted-foreground transition-transform ${open ? "rotate-180" : ""}`}
				/>
				<span className="min-w-0 flex-1">
					<span className="block text-sm font-medium">{title}</span>
					<span className="block text-xs text-muted-foreground">{summary}</span>
				</span>
			</button>
			{open && <div className="border-t border-border p-4">{children}</div>}
		</section>
	);
}

/**
 * Candidate C — no tabs. Stacked sections, each opening on demand.
 *
 * Rejects the premise that these are separate places: an operator on a support
 * call often wants a limit *and* what changed, and tabs make the second one
 * invisible. Everything is one scroll, nothing is hidden behind a click that
 * looks like navigation, and each section still pays for itself only when
 * opened. The cost is a taller page than either tabbed candidate.
 */
export function DetailSections({
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

	const attention = limits.filter((name) =>
		[LicenseUsageStatus.EXCEEDED, LicenseUsageStatus.AT_LIMIT].includes(
			usage[name].status,
		),
	);
	const rest = limits.filter((name) => !attention.includes(name));

	const row = (name: string) => {
		const entry = usage[name];
		const open = openField === name;
		return (
			<li key={name} className="flex flex-col">
				<button
					type="button"
					aria-expanded={open}
					onClick={() => {
						const next = open ? null : name;
						setOpenField(next);
						if (next) onLoadSeries?.(next);
					}}
					className="flex items-center justify-between gap-3 px-3 py-2 text-left transition-colors hover:bg-muted/50"
				>
					<span className="min-w-0 truncate font-mono text-xs">{name}</span>
					<span className="flex shrink-0 items-center gap-3">
						<span className="text-sm tabular-nums">
							{entry.usage === undefined ? "—" : formatUsageNumber(entry.usage)}{" "}
							<span className="text-muted-foreground">
								/ {formatUsageNumber(entry.limit)}
							</span>
						</span>
						<StatusBadge tone={usageStatusTone(entry.status)}>
							{usageStatusLabel(entry.status)}
						</StatusBadge>
					</span>
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
	};

	return (
		<div className="flex flex-col gap-4">
			{attention.length > 0 && (
				<section className="flex flex-col gap-2">
					<h2 className="text-sm font-medium">
						Needs attention
						<span className="ml-2 font-normal text-muted-foreground tabular-nums">
							{attention.length}
						</span>
					</h2>
					{/* The whole reason a customer book page gets opened. Never folded
						away, and never buried under nineteen healthy rows. */}
					<ul className="divide-y divide-border rounded-lg border border-border">
						{attention.map(row)}
					</ul>
				</section>
			)}

			<Section
				title="All limits"
				summary={`${limits.length} declared · open one to see its history`}
			>
				<ul className="divide-y divide-border rounded-lg border border-border">
					{rest.map(row)}
				</ul>
			</Section>

			<Section
				title="Change history"
				summary="What this customer was given, and every later change"
				onFirstOpen={onLoadHistory}
			>
				<OrganizationLicenseHistoryView
					items={history}
					total={history.length}
					templateName={templateName}
				/>
			</Section>

			<Section
				title="All license values"
				summary={`${fields.length} declared fields, limits included`}
			>
				<LicenseValueFields fields={fields} values={values} />
			</Section>

			<div className="flex justify-end">
				<Button variant="ghost" size="sm" disabled>
					{limits.length} limits · {history.length} recorded changes
				</Button>
			</div>
		</div>
	);
}
