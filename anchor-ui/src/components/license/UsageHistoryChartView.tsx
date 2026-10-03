import { UsageGranularity } from "@/client";
import { Button } from "@nanostackorg/design-system/components/button";
import {
	ChartContainer,
	ChartTooltip,
} from "@nanostackorg/design-system/components/chart";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@nanostackorg/design-system/components/empty";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Skeleton } from "@nanostackorg/design-system/components/skeleton";
import { Text } from "@nanostackorg/design-system/components/text";
import {
	ToggleGroup,
	ToggleGroupItem,
} from "@nanostackorg/design-system/components/toggle-group";
import { Spread } from "@nanostackorg/design-system/layout/spread";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import {
	ChartLineIcon as ChartSpline,
	WarningIcon as TriangleAlert,
} from "@phosphor-icons/react";
import dayjs from "dayjs";
import {
	CartesianGrid,
	Line,
	LineChart,
	ReferenceLine,
	XAxis,
	YAxis,
} from "recharts";
import {
	formatExactNumber,
	formatUsageNumber,
	niceCeiling,
} from "./license-usage-status";

/**
 * Granularity is derived from the range rather than chosen, because the
 * cascade retains finer levels for less time: asking for minutes across ninety
 * days returns an empty series rather than a coarse one.
 */
export const usageRanges = [
	{ value: "24h", label: "24h", days: 1, granularity: UsageGranularity.MINUTE },
	{ value: "7d", label: "7 days", days: 7, granularity: UsageGranularity.HOUR },
	{
		value: "30d",
		label: "30 days",
		days: 30,
		granularity: UsageGranularity.DAY,
	},
	{
		value: "90d",
		label: "90 days",
		days: 90,
		granularity: UsageGranularity.DAY,
	},
] as const;

export type UsageRangeValue = (typeof usageRanges)[number]["value"];

const bucketTickFormats: Record<UsageGranularity, string> = {
	[UsageGranularity.MINUTE]: "HH:mm",
	[UsageGranularity.HOUR]: "ddd HH:mm",
	[UsageGranularity.DAY]: "D MMM",
};

export interface UsageHistoryPoint {
	bucket: string;
	value: number;
}

export interface UsageHistoryChartViewProps {
	field: string;
	limit: number;
	rangeValue: UsageRangeValue;
	onRangeChange: (range: UsageRangeValue) => void;
	points: UsageHistoryPoint[];
	isLoading?: boolean;
	errorMessage?: string | null;
	onRetry?: () => void;
}

export function UsageHistoryChartView({
	field,
	limit,
	rangeValue,
	onRangeChange,
	points,
	isLoading,
	errorMessage,
	onRetry,
}: UsageHistoryChartViewProps) {
	const range =
		usageRanges.find((candidate) => candidate.value === rangeValue) ??
		usageRanges[1];

	return (
		<Stack space="md">
			<Spread space="sm">
				<Stack space="xxs">
					<Heading level={3}>
						History for{" "}
						<Text as="span" font="mono">
							{field}
						</Text>
					</Heading>
					<Text size="xs" tone="muted">
						Each point is the last value reported in its bucket, never a sum or
						an average.
					</Text>
				</Stack>
				<ToggleGroup
					size="sm"
					multiple={false}
					value={[rangeValue]}
					onValueChange={(value) => {
						const [next] = value;
						if (next) {
							onRangeChange(next as UsageRangeValue);
						}
					}}
					aria-label="Time range"
				>
					{usageRanges.map((option) => (
						<ToggleGroupItem
							key={option.value}
							value={option.value}
							aria-label={`Last ${option.label}`}
						>
							{option.label}
						</ToggleGroupItem>
					))}
				</ToggleGroup>
			</Spread>

			{isLoading ? (
				<Skeleton height="xxl" />
			) : errorMessage ? (
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={TriangleAlert} />
						<EmptyTitle>Couldn&rsquo;t load usage history</EmptyTitle>
						<EmptyDescription>{errorMessage}</EmptyDescription>
					</EmptyHeader>
					{onRetry && (
						<Button variant="outline" size="sm" onClick={onRetry}>
							Try again
						</Button>
					)}
				</Empty>
			) : points.length === 0 ? (
				<Empty>
					<EmptyHeader>
						<EmptyMedia icon={ChartSpline} />
						<EmptyTitle>Nothing reported in this range</EmptyTitle>
						<EmptyDescription>
							No usage was reported against{" "}
							<Text as="span" font="mono">
								{field}
							</Text>{" "}
							in the last {range.label}. Try a longer range, or check that the
							product is reporting this field.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : (
				<ChartContainer
					config={{ value: { label: field, color: "var(--chart-1)" } }}
				>
					<LineChart
						data={points}
						margin={{ left: 4, right: 12, top: 8 }}
						aria-label={`Reported ${field} usage`}
					>
						<CartesianGrid vertical={false} />
						<XAxis
							dataKey="bucket"
							tickLine={false}
							axisLine={false}
							tickMargin={8}
							minTickGap={24}
							tickFormatter={(bucket: string) =>
								dayjs(bucket).format(bucketTickFormats[range.granularity])
							}
						/>
						<YAxis
							tickLine={false}
							axisLine={false}
							tickMargin={8}
							width={56}
							domain={[
								0,
								(dataMax: number) =>
									niceCeiling(Math.max(dataMax, limit) * 1.05),
							]}
							tickFormatter={(value: number) => formatUsageNumber(value)}
						/>
						<ReferenceLine
							y={limit}
							stroke="var(--color-destructive)"
							strokeDasharray="4 4"
							label={{
								value: `Limit ${formatUsageNumber(limit)}`,
								position: "insideTopRight",
								fill: "var(--color-destructive)",
								fontSize: 11,
							}}
						/>
						<ChartTooltip
							labelFormatter={(_, payload) =>
								dayjs(payload?.[0]?.payload?.bucket).format("D MMM YYYY HH:mm")
							}
							formatter={(value) => formatExactNumber(Number(value))}
						/>
						<Line
							dataKey="value"
							type="monotone"
							stroke="var(--color-value)"
							strokeWidth={2}
							dot={false}
							activeDot={{ r: 4 }}
							isAnimationActive={false}
						/>
					</LineChart>
				</ChartContainer>
			)}
		</Stack>
	);
}
