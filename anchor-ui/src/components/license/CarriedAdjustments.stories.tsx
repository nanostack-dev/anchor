import { LicenseFieldType } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { CarriedAdjustments } from "./CarriedAdjustments";

const meta = {
	title: "License/CarriedAdjustments",
	component: CarriedAdjustments,
	parameters: { layout: "padded" },
} satisfies Meta<typeof CarriedAdjustments>;

export default meta;

type Story = StoryObj<typeof meta>;

const limit = (field: string, from: number, to: number) => ({
	field,
	type: LicenseFieldType.LIMIT,
	from,
	to,
	carried: true,
});

export const SeveralCustomers: Story = {
	args: {
		tierName: "Free",
		groups: [
			{
				organization: "Acme Corp",
				changes: [limit("max_flows", 10, 1200), limit("max_members", 3, 40)],
			},
			{ organization: "Globex", changes: [limit("max_flows", 10, 260)] },
			{ organization: "Initech", changes: [limit("max_flows", 10, 25000)] },
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// The defect this component exists for: three max_flows rows carrying
		// different values are only readable with the customer's name on them.
		await expect(canvas.getByText("Acme Corp")).toBeVisible();
		await expect(canvas.getByText("Globex")).toBeVisible();
		await expect(canvas.getByText("1200")).toBeVisible();
		await expect(canvas.getByText("260")).toBeVisible();
		await expect(canvas.getByText("3 organizations")).toBeVisible();
	},
};

export const MoreThanFits: Story = {
	args: {
		tierName: "Free",
		groups: Array.from({ length: 18 }, (_, index) => ({
			organization: `Customer ${index + 1}`,
			changes: [limit("max_flows", 10, 100 * (index + 1))],
		})),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("18 organizations")).toBeVisible();
		await expect(canvas.getByText("Customer 5")).toBeVisible();
		await expect(canvas.queryByText("Customer 6")).toBeNull();
		await expect(
			canvas.getByText(/13 more organizations keep 13 adjustments/),
		).toBeVisible();
	},
};

export const OneCustomerOneField: Story = {
	args: {
		tierName: "Pro",
		groups: [
			{
				organization: "Northwind Trading",
				changes: [limit("max_flows", 500, 900)],
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("1 organization")).toBeVisible();
	},
};
