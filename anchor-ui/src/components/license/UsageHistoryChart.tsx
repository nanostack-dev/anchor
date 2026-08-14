import { getOrganizationUsageSeriesOptions } from "@/client/@tanstack/react-query.gen";
import { getErrorDetail } from "@/lib/api-error";
import { useQuery } from "@tanstack/react-query";
import dayjs from "dayjs";
import { useMemo, useState } from "react";
import {
	UsageHistoryChartView,
	type UsageRangeValue,
	usageRanges,
} from "./UsageHistoryChartView";

export interface UsageHistoryChartProps {
	productId: string;
	organizationId: string;
	field: string;
	limit: number;
}

export function UsageHistoryChart({
	productId,
	organizationId,
	field,
	limit,
}: UsageHistoryChartProps) {
	const [rangeValue, setRangeValue] = useState<UsageRangeValue>("7d");
	const range =
		usageRanges.find((candidate) => candidate.value === rangeValue) ??
		usageRanges[1];

	const from = useMemo(
		() => dayjs().subtract(range.days, "day").toISOString(),
		[range.days],
	);

	const seriesQuery = useQuery({
		...getOrganizationUsageSeriesOptions({
			path: { product_id: productId, organization_id: organizationId },
			query: {
				key: field,
				granularity: range.granularity,
				from,
				limit: 1000,
			},
		}),
		retry: false,
	});

	const points = useMemo(
		() =>
			(seriesQuery.data?.items ?? []).map((point) => ({
				bucket: point.bucket,
				value: point.value,
			})),
		[seriesQuery.data],
	);

	return (
		<UsageHistoryChartView
			field={field}
			limit={limit}
			rangeValue={rangeValue}
			onRangeChange={setRangeValue}
			points={points}
			isLoading={seriesQuery.isLoading}
			errorMessage={
				seriesQuery.error
					? (getErrorDetail(seriesQuery.error) ??
						"The series could not be read. Try again, or pick a shorter range.")
					: null
			}
			onRetry={() => void seriesQuery.refetch()}
		/>
	);
}
