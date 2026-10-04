import {
	LicenseDifferenceKind,
	LicenseFieldType,
	LicenseMigrationDifferencePolicy,
	LicenseMigrationOutcome,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
	type OrganizationLicenseMigrationResponse,
	client,
} from "@/client";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { LicenseMigrationDialog } from "./LicenseMigrationDialog";

const productId = "prd_migration_story";
const timestamps = {
	created_at: "2026-10-03T12:00:00Z",
	updated_at: "2026-10-03T12:00:00Z",
};
const growth: LicenseTemplateResponse = {
	...timestamps,
	id: "ltpl_growth",
	product_id: productId,
	name: "Growth",
	status: LicenseTemplateStatus.ACTIVE,
	values: { max_flows: 500 },
};
const requestBody = fn();
const receipt: OrganizationLicenseMigrationResponse = {
	template_id: growth.id,
	migrated_at: timestamps.created_at,
	count: 1,
	changed: 1,
	unchanged: 0,
	failed: 0,
	results: [
		{
			organization_id: "org_migration_story",
			outcome: LicenseMigrationOutcome.CHANGED,
			count: 1,
			changes: [
				{
					field: "max_flows",
					kind: LicenseDifferenceKind.ONLY_IN_TEMPLATE,
					license_value: null,
					template_value: 500,
				},
			],
		},
	],
};

const meta = {
	title: "License/LicenseMigrationDialog",
	component: LicenseMigrationDialog,
	parameters: { layout: "padded" },
	args: {
		open: true,
		onOpenChange: fn(),
		productId,
		selection: [
			{
				organization_id: "org_migration_story",
				organization_name: "Acme Studio",
			},
		],
		templates: [
			growth,
			{
				...growth,
				id: "ltpl_archived",
				name: "Retired tier",
				status: LicenseTemplateStatus.ARCHIVED,
			},
		],
		fields: [
			{
				...timestamps,
				id: "lfd_migration_story",
				name: "max_flows",
				type: LicenseFieldType.LIMIT,
				rules: { min: 0 },
			},
		],
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
	beforeEach: () => {
		requestBody.mockClear();
		const previous = client.getConfig();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				expect(request.method).toBe("POST");
				expect(new URL(request.url).pathname).toBe(
					`/v1/products/${productId}/licensing/organization-licenses/migrate`,
				);
				requestBody(await request.json());
				return Response.json(receipt);
			},
		});
		return () => client.setConfig(previous);
	},
} satisfies Meta<typeof LicenseMigrationDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Tier labels stay readable while the migration request keeps the template identifier. */
export const TierLabelKeepsItsIdentifierInTheRequest: Story = {
	play: async ({ canvasElement }) => {
		const page = within(canvasElement.ownerDocument.body);
		const dialog = within(
			await page.findByRole("dialog", { name: "Move 1 organization" }),
		);
		const target = dialog.getByRole("combobox", { name: "Move to tier" });
		await expect(target).toHaveTextContent("Select a tier...");
		await expect(
			dialog.getByRole("button", { name: "Move 1 organization" }),
		).toBeDisabled();
		await userEvent.click(target);
		await expect(
			await page.findByRole("listbox", { name: "Target license tier options" }),
		).toBeVisible();
		await expect(
			page.queryByRole("option", { name: "Retired tier" }),
		).not.toBeInTheDocument();
		await userEvent.click(await page.findByRole("option", { name: "Growth" }));
		await expect(target).toHaveTextContent("Growth");
		await expect(target).not.toHaveTextContent(growth.id);
		await userEvent.click(
			dialog.getByRole("button", { name: "Move 1 organization" }),
		);
		await waitFor(() =>
			expect(requestBody).toHaveBeenCalledWith({
				template_id: growth.id,
				organization_ids: ["org_migration_story"],
				on_difference: LicenseMigrationDifferencePolicy.CARRY_FORWARD,
			}),
		);
		await expect(
			await page.findByRole("heading", { name: "Migration results" }),
		).toBeVisible();
		await expect(page.getByText("Acme Studio")).toBeVisible();
	},
};
