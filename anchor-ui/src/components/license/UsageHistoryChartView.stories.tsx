import type { Decorator, Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import {
	UsageHistoryChartView,
	type UsageRangeValue,
} from "./UsageHistoryChartView";

function hourlySeries(buckets: number, startValue: number, step: number) {
	const bucketMs = 60 * 60 * 1000;
	const start = Date.now() - buckets * bucketMs;
	return Array.from({ length: buckets }, (_, index) => ({
		bucket: new Date(start + index * bucketMs).toISOString(),
		value: startValue + index * step,
	}));
}

/**
 * Recharts measures its container, so a story without explicit width renders an
 * empty chart and every assertion against a plotted element silently passes on
 * nothing.
 */
const sizedCanvas: Decorator = (Story) => (
	<div style={{ width: 720, height: 460 }}>
		<Story />
	</div>
);

const meta = {
	title: "License/UsageHistoryChartView",
	component: UsageHistoryChartView,
	parameters: { layout: "padded" },
	decorators: [sizedCanvas],
	args: {
		field: "api_calls",
		limit: 800,
		rangeValue: "7d" as UsageRangeValue,
		onRangeChange: () => {},
		points: [],
	},
} satisfies Meta<typeof UsageHistoryChartView>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Recharts sizes itself from a ResizeObserver that never fires in the browser
 * test runner, so the plotted SVG is absent here and present in a real browser.
 * These stories therefore assert which branch rendered, and the plotted content
 * itself is reviewed visually in Storybook.
 */
export const CrossesTheLimit: Story = {
	args: { points: hourlySeries(48, 400, 12) },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		expect(canvas.queryByText("Nothing reported in this range")).toBeNull();
		expect(canvas.queryByText("Couldn’t load usage history")).toBeNull();
		await expect(
			canvas.getByRole("group", { name: "Time range" }),
		).toBeVisible();
	},
};

export const StaysWellBelowTheLimit: Story = {
	args: { points: hourlySeries(48, 40, 1) },
};

export const FlatSeries: Story = {
	args: { points: hourlySeries(48, 250, 0) },
};

export const NothingReportedInRange: Story = {
	args: { points: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(
			canvas.getByText("Nothing reported in this range"),
		).toBeVisible();
	},
};

export const SeriesFailsToLoad: Story = {
	args: {
		points: [],
		errorMessage: "A usage range cannot be longer than one year.",
		onRetry: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("Couldn’t load usage history")).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};

export const LoadingTheSeries: Story = {
	args: { points: [], isLoading: true },
};

function RangeHarness() {
	const [rangeValue, setRangeValue] = useState<UsageRangeValue>("7d");
	return (
		<UsageHistoryChartView
			field="api_calls"
			limit={800}
			rangeValue={rangeValue}
			onRangeChange={setRangeValue}
			points={hourlySeries(48, 400, 12)}
		/>
	);
}

export const ChangingTheRange: Story = {
	render: () => <RangeHarness />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByRole("button", { name: "Last 30 days" }));

		await expect(
			canvas.getByRole("button", { name: "Last 30 days" }),
		).toHaveAttribute("aria-pressed", "true");
	},
};
