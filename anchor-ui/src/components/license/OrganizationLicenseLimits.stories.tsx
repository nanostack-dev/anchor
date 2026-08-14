import { type LicenseFieldUsageResponse, LicenseUsageStatus } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import { OrganizationLicenseLimits } from "./OrganizationLicenseLimits";

const minutesAgo = (minutes: number) =>
	new Date(Date.now() - minutes * 60_000).toISOString();

const EVERY_STATUS: Record<string, LicenseFieldUsageResponse> = {
	api_calls: {
		limit: 1_000_000,
		usage: 1_450_320,
		status: LicenseUsageStatus.EXCEEDED,
		last_reported_at: minutesAgo(4),
	},
	max_flows: {
		limit: 500,
		usage: 340,
		status: LicenseUsageStatus.WITHIN_LIMIT,
		last_reported_at: minutesAgo(9),
	},
	seats: {
		limit: 25,
		usage: 25,
		status: LicenseUsageStatus.AT_LIMIT,
		last_reported_at: minutesAgo(10 * 60),
	},
	webhooks: {
		limit: 50,
		status: LicenseUsageStatus.STALE,
	},
};

const meta = {
	title: "License/OrganizationLicenseLimits",
	component: OrganizationLicenseLimits,
	parameters: { layout: "padded" },
} satisfies Meta<typeof OrganizationLicenseLimits>;

export default meta;
type Story = StoryObj<typeof meta>;

export const EveryStatusIsDistinguishable: Story = {
	args: {
		usage: EVERY_STATUS,
		selectedField: "api_calls",
		onSelectField: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("Exceeded")).toBeVisible();
		await expect(canvas.getByText("At limit")).toBeVisible();
		await expect(canvas.getByText("Within limit")).toBeVisible();
		await expect(canvas.getByText("Never reported")).toBeVisible();
	},
};

export const NeverReportedShowsNoUsageNumber: Story = {
	args: {
		usage: { webhooks: EVERY_STATUS.webhooks },
		selectedField: null,
		onSelectField: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("Never reported")).toBeVisible();
		await expect(canvas.getByText("limit 50")).toBeVisible();
	},
};

export const LargeNumbersStayReadable: Story = {
	args: {
		usage: { api_calls: EVERY_STATUS.api_calls },
		selectedField: null,
		onSelectField: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("1.5M")).toBeVisible();
		await expect(canvas.getByText("of 1M")).toBeVisible();
	},
};

export const NoLimitFieldsDeclared: Story = {
	args: { usage: {}, selectedField: null, onSelectField: () => {} },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText(/no limit fields/i)).toBeVisible();
	},
};

function SelectionHarness() {
	const [selectedField, setSelectedField] = useState<string | null>(
		"max_flows",
	);
	return (
		<OrganizationLicenseLimits
			usage={EVERY_STATUS}
			selectedField={selectedField}
			onSelectField={setSelectedField}
		/>
	);
}

export const SelectingALimitMarksItPressed: Story = {
	args: {
		usage: EVERY_STATUS,
		selectedField: null,
		onSelectField: () => {},
	},
	render: () => <SelectionHarness />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const seats = canvas.getByRole("button", { name: /seats/ });
		await expect(seats).toHaveAttribute("aria-pressed", "false");

		await userEvent.click(seats);

		await expect(seats).toHaveAttribute("aria-pressed", "true");
	},
};
