import { LicenseFieldType } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { TemplateValuesDiff } from "./TemplateValuesDiff";

const meta = {
	title: "License/TemplateValuesDiff",
	component: TemplateValuesDiff,
	parameters: { layout: "padded" },
} satisfies Meta<typeof TemplateValuesDiff>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WhatChangesBetweenTwoTiers: Story = {
	args: {
		fromLabel: "Beta",
		toLabel: "Pro",
		unchangedCount: 2,
		changes: [
			{
				field: "max_flows",
				type: LicenseFieldType.LIMIT,
				from: 500,
				to: 5000,
			},
			{
				field: "sso",
				type: LicenseFieldType.BOOLEAN,
				from: false,
				to: true,
			},
			{
				field: "support_tier",
				type: LicenseFieldType.ENUM,
				from: "basic",
				to: "priority",
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("500")).toBeVisible();
		await expect(canvas.getByText("5000")).toBeVisible();
		// A boolean reads as Yes/No, never as true/false.
		await expect(canvas.getByText("Yes")).toBeVisible();
		await expect(
			canvas.getByText("2 other license fields unchanged."),
		).toBeVisible();
	},
};

export const ValuesKeptForACustomerReadTheOtherWayRound: Story = {
	args: {
		fromLabel: "Pro grants",
		toLabel: "These customers keep",
		changes: [
			{
				field: "max_flows",
				type: LicenseFieldType.LIMIT,
				from: 5000,
				to: 8000,
				carried: true,
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// The tier's own value is what gets struck through; the bespoke value is
		// what survives. Marking it is the only thing that tells the two apart.
		await expect(canvas.getByText("kept for this customer")).toBeVisible();
		await expect(canvas.getByText("8000")).toBeVisible();
	},
};

export const NoSingleTierToCompareAgainst: Story = {
	args: {
		fromLabel: "Current tier",
		toLabel: "Enterprise",
		changes: [
			{
				field: "max_flows",
				type: LicenseFieldType.LIMIT,
				from: undefined,
				to: 5000,
			},
			{
				field: "sso",
				type: LicenseFieldType.BOOLEAN,
				from: undefined,
				to: true,
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// A selection spread over several tiers has no single before. Nothing is
		// struck out, rather than an em-dash that reads as a rendering fault.
		await expect(canvas.getByText("5000")).toBeVisible();
		await expect(canvas.queryByText("—")).toBeNull();
	},
};

export const TiersThatGrantTheSameThing: Story = {
	args: {
		fromLabel: "Pro",
		toLabel: "Pro (renamed)",
		changes: [],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(
			canvas.getByText("The two tiers grant exactly the same values."),
		).toBeVisible();
	},
};
