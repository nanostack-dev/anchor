import {
	LicenseChangeType,
	type OrganizationLicenseChangeResponse,
} from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { OrganizationLicenseHistoryView } from "./OrganizationLicenseHistoryView";

const minutesAgo = (minutes: number) =>
	new Date(Date.now() - minutes * 60_000).toISOString();

const base = {
	product_id: "prd_2ikcVW44U7UtqJHCOTqHuwkgrBb",
	organization_id: "org_2ikcVW44U7UtqJHCOTqHuwkgrBc",
	license_id: "lic_2ikcVW44U7UtqJHCOTqHuwkgrBd",
} as const;

const instantiated: OrganizationLicenseChangeResponse = {
	...base,
	id: "lchg_2ikcVW44U7UtqJHCOTqHuwkgrBe",
	type: LicenseChangeType.INSTANTIATED,
	template_id: "ltpl_2ikcVW44U7UtqJHCOTqHuwkgrBf",
	new_value: {
		flows: 500,
		sso: true,
		support_tier: "priority",
		region: "ca-central",
	},
	changed_at: minutesAgo(3 * 24 * 60),
};

const raisedFlows: OrganizationLicenseChangeResponse = {
	...base,
	id: "lchg_2ikcVW44U7UtqJHCOTqHuwkgrBg",
	type: LicenseChangeType.ADJUSTED,
	field: "flows",
	old_value: 500,
	new_value: 800,
	changed_at: minutesAgo(90),
};

const sameMomentSso: OrganizationLicenseChangeResponse = {
	...base,
	id: "lchg_2ikcVW44U7UtqJHCOTqHuwkgrBh",
	type: LicenseChangeType.ADJUSTED,
	field: "sso",
	old_value: true,
	new_value: false,
	changed_at: minutesAgo(20),
};

const sameMomentFlows: OrganizationLicenseChangeResponse = {
	...base,
	id: "lchg_2ikcVW44U7UtqJHCOTqHuwkgrBi",
	type: LicenseChangeType.ADJUSTED,
	field: "flows",
	old_value: 800,
	new_value: 900,
	changed_at: minutesAgo(20),
};

const meta = {
	title: "License/OrganizationLicenseHistoryView",
	component: OrganizationLicenseHistoryView,
	parameters: { layout: "padded" },
	args: {
		items: [],
		total: 0,
	},
} satisfies Meta<typeof OrganizationLicenseHistoryView>;

export default meta;
type Story = StoryObj<typeof meta>;

export const InstantiatedFromATemplate: Story = {
	args: { items: [instantiated], total: 1 },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("Instantiated")).toBeVisible();
		await expect(
			canvas.getByText(instantiated.template_id ?? ""),
		).toBeVisible();
		await expect(canvas.getByText("flows")).toBeVisible();
		await expect(canvas.getByText("500")).toBeVisible();
		await expect(canvas.getByText("Yes")).toBeVisible();
	},
};

export const OneFieldMoved: Story = {
	args: { items: [raisedFlows, instantiated], total: 2 },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("Adjusted")).toBeVisible();
		await expect(canvas.getAllByText("flows").length).toBeGreaterThan(0);
		await expect(canvas.getByText("800")).toBeVisible();
	},
};

export const SeveralFieldsOneMoment: Story = {
	args: {
		items: [sameMomentFlows, sameMomentSso, instantiated],
		total: 3,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getAllByText("Adjusted")).toHaveLength(1);
		await expect(canvas.getAllByText("flows").length).toBeGreaterThan(0);
		await expect(canvas.getAllByText("sso").length).toBeGreaterThan(0);
		await expect(canvas.getByText("No")).toBeVisible();
	},
};

export const EmptyHistory: Story = {
	args: { items: [], total: 0 },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(canvas.getByText("No changes yet")).toBeVisible();
	},
};

export const LoadFailed: Story = {
	args: {
		items: [],
		total: 0,
		errorMessage: "The server responded with HTTP 500.",
		onRetry: () => {},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await expect(
			canvas.getByText("Couldn’t load license history"),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};

export const Loading: Story = {
	args: { items: [], total: 0, isLoading: true },
};

export const AMigrationNamesBothTiers: Story = {
	args: {
		total: 1,
		templateName: (id: string) =>
			({ ltpl_beta: "Early Access", ltpl_pro: "Pro" })[id] ?? id,
		items: [
			{
				id: "lchg_migrated",
				product_id: "prd_1",
				organization_id: "org_1",
				license_id: "lic_1",
				type: LicenseChangeType.SET,
				template_id: "ltpl_pro",
				previous_template_id: "ltpl_beta",
				old_value: { max_flows: 50 },
				new_value: { max_flows: 500, sso: true },
				changed_at: "2026-08-16T10:30:00Z",
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// A tier move reads as one stamped moment, and names where it came from
		// as well as where it went — by name, never by identifier.
		await expect(canvas.getByText("Moved to another tier")).toBeVisible();
		await expect(canvas.getByText("Early Access")).toBeVisible();
		await expect(canvas.getByText("Pro")).toBeVisible();
		await expect(canvas.queryByText(/ltpl_/)).toBeNull();
	},
};

export const AGrantNamesOnlyWhereItLanded: Story = {
	args: {
		total: 1,
		templateName: (id: string) => ({ ltpl_pro: "Pro" })[id] ?? id,
		items: [
			{
				id: "lchg_granted",
				product_id: "prd_1",
				organization_id: "org_1",
				license_id: "lic_1",
				type: LicenseChangeType.SET,
				template_id: "ltpl_pro",
				new_value: { max_flows: 500, sso: true },
				changed_at: "2026-08-16T10:30:00Z",
			},
		],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		// Granted through the same route a move uses, but with no
		// previous_template_id there is nowhere to say it came from.
		await expect(canvas.getByText("Licensed for the first time")).toBeVisible();
		await expect(canvas.queryByText("Moved from")).toBeNull();
		await expect(canvas.getByText("Pro")).toBeVisible();
	},
};
