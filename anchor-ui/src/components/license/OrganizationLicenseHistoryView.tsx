import {
	LicenseChangeType,
	type OrganizationLicenseChangeResponse,
} from "@/client";
import { StatusBadge } from "@/components/common/StatusBadge";
import { cn } from "@/lib/utils";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	ArrowRightIcon as ArrowRight,
	ClockCounterClockwiseIcon as History,
	SlidersHorizontalIcon as SlidersHorizontal,
	WarningIcon as TriangleAlert,
} from "@phosphor-icons/react";
import dayjs from "dayjs";
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
			<Stack space="sm">
				<Skeleton height="xl" />
				<Skeleton height="xl" />
				<Skeleton height="xl" />
			</Stack>
		);
	}

	if (errorMessage) {
		return (
			<Empty>
				<EmptyHeader>
					<EmptyMedia icon={TriangleAlert} />
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
					<EmptyMedia icon={History} />
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
		<Stack space="md">
			<Box
				as="ol"
				className="divide-y divide-border rounded-lg border border-border"
			>
				{moments.map((group) => (
					<HistoryMoment
						key={group[0].id}
						entries={group}
						templateName={templateName}
					/>
				))}
			</Box>
			{hasMore && (
				<Spread space="md">
					<Text size="xs" tone="muted">
						Showing {items.length} of {total}
					</Text>
					<Button
						variant="outline"
						size="sm"
						onClick={onLoadMore}
						disabled={isLoadingMore}
					>
						{isLoadingMore ? "Loading…" : "Load older changes"}
					</Button>
				</Spread>
			)}
		</Stack>
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
	const isAdjustment = first.type === LicenseChangeType.ADJUSTED;
	const when = dayjs(first.changed_at).format("D MMMM YYYY H:mm");
	const stampsWholeSet =
		first.type === LicenseChangeType.INSTANTIATED ||
		first.type === LicenseChangeType.SET ||
		first.type === LicenseChangeType.TEMPLATE_SYNCED;

	return (
		<Box
			as="li"
			className={cn(
				"flex flex-col gap-3 p-3 first:rounded-t-lg last:rounded-b-lg",
				isAdjustment && "bg-accent/50",
			)}
		>
			<Box className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
				{isAdjustment ? (
					<Box
						as="span"
						className="inline-flex items-center gap-2 text-sm font-medium text-accent-foreground"
					>
						<SlidersHorizontal aria-hidden className="size-4 shrink-0" />
						{entries.length} {entries.length === 1 ? "field" : "fields"}{" "}
						customized
					</Box>
				) : (
					<StatusBadge tone={stampsWholeSet ? "info" : "neutral"}>
						{changeTypeLabel(first)}
					</StatusBadge>
				)}
				<time
					dateTime={first.changed_at}
					className="text-xs text-muted-foreground tabular-nums"
					title={dayjs(first.changed_at).toISOString()}
				>
					{when}
				</time>
			</Box>

			{stampsWholeSet ? (
				<InstantiationBody entry={first} templateName={templateName} />
			) : (
				<Stack space="sm" as="ul">
					{entries.map((entry) => (
						<AdjustmentRow key={entry.id} entry={entry} />
					))}
				</Stack>
			)}
		</Box>
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
		<Stack space="sm">
			{entry.previous_template_id && (
				<Text>
					<Text as="span" tone="muted">
						Moved from{" "}
					</Text>
					<Text as="span" weight="medium">
						{templateName(entry.previous_template_id)}
					</Text>
				</Text>
			)}
			{entry.template_id && (
				<Text>
					<Text as="span" tone="muted">
						Template{" "}
					</Text>
					<Text as="span" weight="medium">
						{templateName(entry.template_id)}
					</Text>
				</Text>
			)}
			{names.length > 0 && (
				<dl className="flex flex-col gap-1">
					{names.map((name) => (
						<Box key={name} className="flex flex-wrap items-baseline gap-x-2">
							<Text as="dt" font="mono">
								{name}
							</Text>
							<Text as="dd" tabular>
								{formatHistoryValue(values?.[name])}
							</Text>
						</Box>
					))}
				</dl>
			)}
		</Stack>
	);
}

function AdjustmentRow({
	entry,
}: {
	entry: OrganizationLicenseChangeResponse;
}) {
	return (
		<Box as="li" className="flex flex-wrap items-baseline gap-x-2 gap-y-1">
			<Text as="span" font="mono">
				{entry.field ?? "—"}
			</Text>
			<Box
				as="span"
				className="inline-flex items-center gap-1.5 text-sm tabular-nums"
			>
				<Text as="span" tone="muted">
					{formatHistoryValue(entry.old_value)}
				</Text>
				<ArrowRight
					aria-hidden
					className="size-3.5 shrink-0 text-muted-foreground"
				/>
				<Text as="span" weight="medium">
					{formatHistoryValue(entry.new_value)}
				</Text>
			</Box>
		</Box>
	);
}
