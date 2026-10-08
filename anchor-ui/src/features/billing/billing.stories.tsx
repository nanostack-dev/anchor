import { Page } from "@/components/common/Page";
import { OrganizationLicenseTabs } from "@/components/license/OrganizationLicenseTabs";
import { StoryRouter } from "@/lib/storybook/story-router";
import { Stack } from "@nanostackorg/design-system";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";
import type { State } from "./billing-types";
import { BillingWorkspace } from "./billing-workspace";
import {
	createFixtureAPI,
	emptyFixtureState,
	fixtureState,
	oneFixtureState,
	worstCaseFixtureState,
} from "./fixtures";

function FixtureBilling({
	state,
	organizationId,
	failRefreshOnce = false,
}: { state: State; organizationId?: string; failRefreshOnce?: boolean }) {
	const [api] = useState(() => {
		const fixture = createFixtureAPI(state);
		let loads = 0;
		return {
			...fixture,
			load: async () => {
				loads += 1;
				if (failRefreshOnce && loads === 2)
					throw new Error("Enable Stripe before managing billing.");
				return fixture.load();
			},
		};
	});
	const [queryClient] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	return (
		<StoryRouter
			initialPath={
				organizationId
					? `/organizations/license/${organizationId}/billing`
					: "/products/pricing"
			}
		>
			<QueryClientProvider client={queryClient}>
				<Page
					title={
						organizationId
							? state.organizations.find(
									(organization) => organization.id === organizationId,
								)?.name
							: "Pricing"
					}
					description={
						organizationId
							? "Organization license and billing"
							: "Connect recurring Stripe prices to license templates."
					}
				>
					<Stack space="lg">
						{organizationId && (
							<OrganizationLicenseTabs organizationId={organizationId} />
						)}
						<BillingWorkspace
							api={api}
							productId={state.product.id}
							organizationId={organizationId}
						/>
					</Stack>
				</Page>
			</QueryClientProvider>
		</StoryRouter>
	);
}
const meta = {
	title: "Billing/Workspace",
	component: FixtureBilling,
	parameters: { layout: "fullscreen" },
	args: { state: fixtureState },
} satisfies Meta<typeof FixtureBilling>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Pricing: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("heading", { name: "Pricing", level: 1 }),
		).toBeVisible();
		await expect(
			await canvas.findByRole("heading", { name: "Pro monthly" }),
		).toBeVisible();
	},
};
export const Empty: Story = {
	args: { state: emptyFixtureState },
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByText("No prices yet"),
		).toBeVisible();
	},
};
export const One: Story = {
	args: { state: oneFixtureState },
	play: async ({ canvasElement }) => {
		const list = await within(canvasElement).findByRole("list", {
			name: "Pricing catalog",
		});
		await expect(within(list).getAllByRole("listitem")).toHaveLength(1);
	},
};
export const WorstCase: Story = { args: { state: worstCaseFixtureState } };
export const FailedRefresh: Story = {
	args: { failRefreshOnce: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const name = await canvas.findByLabelText("Price name", { exact: true });
		await userEvent.type(name, "A preserved draft");
		await userEvent.click(
			canvas.getByRole("button", { name: "Reload billing" }),
		);
		await expect(
			await canvas.findByText("Enable Stripe before managing billing."),
		).toBeVisible();
		await expect(
			canvas.getByRole("heading", { name: "Pro monthly" }),
		).toBeVisible();
		await expect(name).toHaveValue("A preserved draft");
		await expect(
			canvas.getByRole("button", { name: "Create price" }),
		).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: "Archive Pro monthly" }),
		).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: "Reload billing" }),
		).toBeEnabled();
		await userEvent.click(canvas.getByRole("button", { name: "Try again" }));
		await expect(
			canvas.getByRole("button", { name: "Create price" }),
		).toBeEnabled();
		await expect(name).toHaveValue("A preserved draft");
	},
};
export const UnlicensedOrganization: Story = {
	args: { organizationId: "org_new" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("button", { name: "Create Checkout" }),
		).toBeVisible();
		await expect(
			within(
				canvas.getByRole("navigation", { name: "License sections" }),
			).getByRole("link", { name: "Billing" }),
		).toHaveAttribute("aria-current", "page");
	},
};
export const ActiveSubscription: Story = {
	args: { organizationId: "org_acme" },
};
export const OrganizationWorstCase: Story = {
	args: { state: worstCaseFixtureState, organizationId: "org_acme" },
};
