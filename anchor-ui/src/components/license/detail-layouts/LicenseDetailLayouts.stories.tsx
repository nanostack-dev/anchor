import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { DetailSections } from "./DetailSections";
import { DetailTabbedList } from "./DetailTabbedList";
import { DetailTabbedSelect } from "./DetailTabbedSelect";
import {
	FEW_LIMITS,
	HISTORY,
	MANY_LIMITS,
	SCHEMA_FIELDS,
	VALUES,
	templateName,
} from "./license-detail-fixture";
import type { LicenseDetailLayoutProps } from "./types";

/**
 * Three answers to the same problem: one product declares twenty limits, and
 * the shipped detail renders one chart inline under a list of all of them.
 *
 * Read them against each other at the same viewport. The question is not which
 * looks tidiest with three limits — every layout does — but which one an
 * operator can still use at twenty.
 */
const meta = {
	title: "License/Detail layout candidates",
	parameters: { layout: "padded" },
} satisfies Meta;

export default meta;

const args: LicenseDetailLayoutProps = {
	usage: MANY_LIMITS,
	fields: SCHEMA_FIELDS,
	values: VALUES,
	history: HISTORY,
	templateName,
};

type Story = StoryObj<typeof meta>;

export const ATabsAndADropdown: Story = {
	name: "A · Tabs + dropdown (one limit at a time)",
	render: () => <DetailTabbedSelect {...args} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// Compact by construction: exactly one chart, whatever the limit count.
		await expect(canvas.getByRole("tab", { name: /Usage/ })).toBeVisible();
		await expect(
			canvas.getByRole("heading", { name: /History for concurrent_runs/ }),
		).toBeVisible();
		// The cost: nineteen statuses are behind the dropdown.
		await expect(canvas.queryByText("max_secrets")).toBeNull();
	},
};

export const BTabsAndAList: Story = {
	name: "B · Tabs + full list, chart opens in place",
	render: () => <DetailTabbedList {...args} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// Every limit stays scannable; nothing is fetched until a row is opened.
		await expect(canvas.getByText("max_secrets")).toBeVisible();
		await expect(
			canvas.queryByRole("heading", { name: /History for/ }),
		).toBeNull();

		await userEvent.click(canvas.getByText("max_flows"));
		await expect(
			canvas.getByRole("heading", { name: /History for max_flows/ }),
		).toBeVisible();
	},
};

export const CSectionsNoTabs: Story = {
	name: "C · No tabs, sections that open on demand",
	render: () => <DetailSections {...args} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// What is wrong leads; the healthy majority is folded away.
		await expect(canvas.getByText("Needs attention")).toBeVisible();
		await expect(canvas.getByText("Change history")).toBeVisible();
		await expect(
			canvas.queryByText(/^What this customer was given/),
		).toBeVisible();
	},
};

export const BAtThreeLimits: Story = {
	name: "B · the same layout with only three limits",
	render: () => <DetailTabbedList {...args} usage={FEW_LIMITS} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// A layout chosen for twenty must not feel over-built at three.
		await expect(canvas.getByText("max_flows")).toBeVisible();
	},
};
