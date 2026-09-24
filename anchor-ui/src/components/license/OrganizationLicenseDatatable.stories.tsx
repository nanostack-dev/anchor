import {
	OrganizationLicenseSortField,
	type OrganizationLicenseSummaryResponse,
	SortDirection,
} from "@/client";
import {
	getLicenseSchemaQueryKey,
	listLicenseTemplatesQueryKey,
	searchOrganizationLicensesQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { expect, within } from "storybook/test";
import { OrganizationLicenseDatatable } from "./OrganizationLicenseDatatable";

const path = { product_id: "prd_test" };
const timestamps = {
	created_at: "2026-08-16T17:27:00Z",
	updated_at: "2026-08-16T17:27:00Z",
};
const standard = {
	organization_id: "org_standard",
	organization_name: "Northstar",
	license: {
		...timestamps,
		id: "lic_standard",
		product_id: path.product_id,
		organization_id: "org_standard",
		template_id: "ltpl_enterprise",
		instantiated_at: timestamps.created_at,
		values: { max_flows: 10000, sso: true },
		adjusted_fields: [],
	},
} satisfies OrganizationLicenseSummaryResponse;
const items: OrganizationLicenseSummaryResponse[] = [
	{
		...standard,
		organization_id: "org_custom",
		organization_name: "Acme Studio",
		license: {
			...standard.license,
			values: { max_flows: 20000, sso: false },
			adjusted_fields: ["max_flows", "sso"],
		},
	},
	standard,
	{
		...standard,
		organization_id: "org_pinned",
		organization_name: "Orbit Labs",
		license: { ...standard.license, adjusted_fields: ["max_flows"] },
	},
	{
		...standard,
		organization_id: "org_drift",
		organization_name: "Summit",
		license: { ...standard.license, values: { max_flows: 9000, sso: true } },
	},
	{ organization_id: "org_new", organization_name: "New organization" },
];

function TableHarness() {
	const [queryClient] = useState(() => {
		const client = new QueryClient({
			defaultOptions: { queries: { staleTime: Number.POSITIVE_INFINITY } },
		});
		client.setQueryData(getLicenseSchemaQueryKey({ path }), { fields: [] });
		client.setQueryData(listLicenseTemplatesQueryKey({ path }), {
			items: [
				{
					...timestamps,
					id: "ltpl_enterprise",
					product_id: path.product_id,
					name: "Enterprise",
					status: "ACTIVE",
					values: standard.license.values,
				},
			],
		});
		client.setQueryData(
			searchOrganizationLicensesQueryKey({
				path,
				body: {
					pagination: { limit: 20, offset: 0 },
					sort_by: OrganizationLicenseSortField.ORGANIZATION_NAME,
					sort_direction: SortDirection.ASC,
				},
			}),
			{ items, total: items.length },
		);
		return client;
	});
	return (
		<StoryRouter>
			<QueryClientProvider client={queryClient}>
				<OrganizationLicenseDatatable productId={path.product_id} />
			</QueryClientProvider>
		</StoryRouter>
	);
}

const meta = {
	title: "License/OrganizationLicenseDatatable",
	component: TableHarness,
	parameters: { layout: "padded" },
} satisfies Meta<typeof TableHarness>;

export default meta;
type Story = StoryObj<typeof meta>;

export const CustomValues: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Acme Studio")).toBeVisible();
		await expect(canvas.queryByText("Adjusted")).not.toBeInTheDocument();
		const customLink = canvas.getByRole("link", {
			name: "Acme Studio: 2 custom fields. Open license details",
		});
		await expect(customLink).toBeVisible();
		await expect(customLink).toHaveAttribute(
			"href",
			"/organizations/license/org_custom",
		);
		await expect(
			canvas.getByRole("link", {
				name: "Orbit Labs: 1 custom field. Open license details",
			}),
		).toBeVisible();
		await expect(canvas.getByText("Matches")).toBeVisible();
		await expect(canvas.getByText("Differs from template")).toBeVisible();
		await expect(canvas.getByText("No license")).toBeVisible();
		customLink.focus();
		await expect(customLink).toHaveFocus();
		customLink.blur();
	},
};
