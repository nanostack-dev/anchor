import {
	LicenseChangeType,
	type OrganizationLicenseChangeResponse,
} from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { Button } from "@/components/ui/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import dayjs from "dayjs";
import { ArrowRight, History, TriangleAlert } from "lucide-react";
import {
	asValueSet,
	changeTypeLabel,
	formatHistoryValue,
	groupHistoryByMoment,
} from "./license-history-format";

export interface OrganizationLicenseHistoryViewProps {
	items: OrganizationLicenseChangeResponse[];
	total: number;
	isLoading?: boolean;
	errorMessage?: string | null;
	onRetry?: () => void;
	onLoadMore?: () => void;
	isLoadingMore?: boolean;
	/**
	 * Resolves a template identifier to the name an operator knows it by. An
	 * entry naming `ltpl_3I11xG...` says nothing on a page whose job is to
	 * explain what a customer was given.
	 */
	templateName?: (templateId: string) => string;
}

export function OrganizationLicenseHistoryView({
	items,
	total,
	isLoading = false,
	errorMessage = null,
	onRetry,
	onLoadMore,
	isLoadingMore = false,
	templateName = (templateId) => templateId,
}: OrganizationLicenseHistoryViewProps) {
	if (isLoading) {
		return (
			<div className="flex flex-col gap-2">
				<Skeleton className="h-16 w-full" />
				<Skeleton className="h-16 w-full" />
				<Skeleton className="h-16 w-full" />
			</div>
		);
	}

	if (errorMessage) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon" className="text-destructive">
						<TriangleAlert />
					</EmptyMedia>
					<EmptyTitle>Couldn&rsquo;t load license history</EmptyTitle>
					<EmptyDescription>{errorMessage}</EmptyDescription>
				</EmptyHeader>
				{onRetry && (
					<Button variant="outline" size="sm" onClick={onRetry}>
						Try again
					</Button>
				)}
			</Empty>
		);
	}

	if (items.length === 0) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia variant="icon">
						<History />
					</EmptyMedia>
					<EmptyTitle>No changes yet</EmptyTitle>
					<EmptyDescription>
						Nothing has been written to this organization&rsquo;s license.
						Instantiation and later adjustments appear here, newest first.
					</EmptyDescription>
				</EmptyHeader>
			</Empty>
		);
	}

	const moments = groupHistoryByMoment(items);
	const hasMore = Boolean(onLoadMore) && items.length < total;

	return (
		<div className="flex flex-col gap-3">
			<ol className="divide-y divide-border rounded-lg border border-border">
				{moments.map((group) => (
					<HistoryMoment
						key={group[0].id}
						entries={group}
						templateName={templateName}
					/>
				))}
			</ol>
			{hasMore && (
				<div className="flex items-center justify-between gap-3">
					<p className="text-xs text-muted-foreground">
						Showing {items.length} of {total}
					</p>
					<Button
						variant="outline"
						size="sm"
						onClick={onLoadMore}
						disabled={isLoadingMore}
					>
						{isLoadingMore ? "Loading…" : "Load older changes"}
					</Button>
				</div>
			)}
		</div>
	);
}

function HistoryMoment({
	entries,
	templateName,
}: {
	entries: OrganizationLicenseChangeResponse[];
	templateName: (templateId: string) => string;
}) {
	const first = entries[0];
	const when = dayjs(first.changed_at).format("D MMMM YYYY H:mm");
	// A migration replaces the whole set exactly as an instantiation does, so it
	// reads as one stamped moment rather than as a list of field adjustments.
	const isMigration = first.type === LicenseChangeType.MIGRATED;
	const stampsWholeSet =
		first.type === LicenseChangeType.INSTANTIATED || isMigration;

	return (
		<li className="flex flex-col gap-3 p-3">
			<div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
				<StatusBadge tone={stampsWholeSet ? "info" : "neutral"}>
					{changeTypeLabel(first.type)}
				</StatusBadge>
				<time
					dateTime={first.changed_at}
					className="text-xs text-muted-foreground tabular-nums"
					title={dayjs(first.changed_at).toISOString()}
				>
					{when}
				</time>
			</div>

			{stampsWholeSet ? (
				<InstantiationBody entry={first} templateName={templateName} />
			) : (
				<ul className="flex flex-col gap-2">
					{entries.map((entry) => (
						<AdjustmentRow key={entry.id} entry={entry} />
					))}
				</ul>
			)}
		</li>
	);
}

function InstantiationBody({
	entry,
	templateName,
}: {
	entry: OrganizationLicenseChangeResponse;
	templateName: (templateId: string) => string;
}) {
	const values = asValueSet(entry.new_value);
	const names = values
		? Object.keys(values).sort((a, b) => a.localeCompare(b))
		: [];

	return (
		<div className="flex flex-col gap-2">
			{entry.previous_template_id && (
				<p className="text-sm">
					<span className="text-muted-foreground">Moved from </span>
					<span className="font-medium">
						{templateName(entry.previous_template_id)}
					</span>
				</p>
			)}
			{entry.template_id && (
				<p className="text-sm">
					<span className="text-muted-foreground">Template </span>
					<span className="font-medium">{templateName(entry.template_id)}</span>
				</p>
			)}
			{names.length > 0 && (
				<dl className="flex flex-col gap-1">
					{names.map((name) => (
						<div key={name} className="flex flex-wrap items-baseline gap-x-2">
							<dt className="font-mono text-sm">{name}</dt>
							<dd className="text-sm tabular-nums">
								{formatHistoryValue(values?.[name])}
							</dd>
						</div>
					))}
				</dl>
			)}
		</div>
	);
}

function AdjustmentRow({
	entry,
}: {
	entry: OrganizationLicenseChangeResponse;
}) {
	return (
		<li className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
			<span className="font-mono text-sm">{entry.field ?? "—"}</span>
			<span className="inline-flex items-center gap-1.5 text-sm tabular-nums">
				<span className="text-muted-foreground">
					{formatHistoryValue(entry.old_value)}
				</span>
				<ArrowRight
					aria-hidden
					className="size-3.5 shrink-0 text-muted-foreground"
				/>
				<span className="font-medium">
					{formatHistoryValue(entry.new_value)}
				</span>
			</span>
		</li>
	);
}
