import { UsageGranularity } from "@/client";
import { Button } from "@/components/ui/button";
import {
	ChartContainer,
	ChartTooltip,
	ChartTooltipContent,
} from "@/components/ui/chart";
import {
	Empty,
	EmptyDescription,
	EmptyHeader,
	EmptyMedia,
	EmptyTitle,
} from "@/components/ui/empty";
import { Skeleton } from "@/components/ui/skeleton";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import dayjs from "dayjs";
import { ChartSpline, TriangleAlert } from "lucide-react";
import {
	CartesianGrid,
	Line,
	LineChart,
	ReferenceLine,
	XAxis,
	YAxis,
} from "recharts";
import { formatExactNumber, formatUsageNumber } from "./license-usage-status";

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
		<div className="flex flex-col gap-3">
			<div className="flex flex-wrap items-center justify-between gap-2">
				<div className="flex flex-col gap-0.5">
					<h3 className="text-sm font-semibold">
						History for <span className="font-mono">{field}</span>
					</h3>
					<p className="text-xs text-muted-foreground">
						Each point is the last value reported in its bucket, never a sum or
						an average.
					</p>
				</div>
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
			</div>

			{isLoading ? (
				<Skeleton className="aspect-video w-full" />
			) : errorMessage ? (
				<Empty>
					<EmptyHeader>
						<EmptyMedia variant="icon" className="text-destructive">
							<TriangleAlert />
						</EmptyMedia>
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
						<EmptyMedia variant="icon">
							<ChartSpline />
						</EmptyMedia>
						<EmptyTitle>Nothing reported in this range</EmptyTitle>
						<EmptyDescription>
							No usage was reported against{" "}
							<span className="font-mono">{field}</span> in the last{" "}
							{range.label}. Try a longer range, or check that the product is
							reporting this field.
						</EmptyDescription>
					</EmptyHeader>
				</Empty>
			) : (
				<ChartContainer
					config={{ value: { label: field, color: "var(--color-primary)" } }}
					className="aspect-video w-full"
				>
					<LineChart data={points} margin={{ left: 4, right: 12, top: 8 }}>
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
							width={44}
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
							content={
								<ChartTooltipContent
									labelFormatter={(_, payload) =>
										dayjs(payload?.[0]?.payload?.bucket).format(
											"D MMM YYYY HH:mm",
										)
									}
									formatter={(value) => formatExactNumber(Number(value))}
								/>
							}
						/>
						<Line
							dataKey="value"
							type="monotone"
							stroke="var(--color-primary)"
							strokeWidth={2}
							dot={false}
							activeDot={{ r: 4 }}
							isAnimationActive={false}
						/>
					</LineChart>
				</ChartContainer>
			)}
		</div>
	);
}
