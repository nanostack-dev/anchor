import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { Button } from "@/components/ui/button";
import { StoryRouter } from "@/lib/storybook/story-router";

import { Page } from "./Page";

const meta = {
	title: "Common/Page",
	component: Page,
	tags: ["autodocs"],
	// Breadcrumbs are derived from the router location, so the initial path is
	// what each story is really exercising.
	decorators: [
		(Story) => (
			<StoryRouter initialPath="/products/checkout-api/api-keys">
				<Story />
			</StoryRouter>
		),
	],
	args: {
		title: "API keys",
		description: "Keys are scoped to this product and cannot be read back.",
		children: <p>Table goes here.</p>,
	},
	parameters: {
		layout: "fullscreen",
	},
} satisfies Meta<typeof Page>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("heading", { name: "API keys", level: 1 }),
		).toBeInTheDocument();
		await expect(canvas.getByTestId("page-content")).toHaveTextContent(
			"Table goes here.",
		);
	},
};

/**
 * Breadcrumbs come from the path: the first crumb is Dashboard, the last is the
 * current page and is a `BreadcrumbPage` rather than a link.
 */
export const Breadcrumbs: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const nav = canvas.getByRole("navigation", { name: "Breadcrumb" });
		const crumbs = within(nav);

		await expect(
			crumbs.getByRole("link", { name: "Dashboard" }),
		).toHaveAttribute("href", "/");

		// The trail for /products/checkout-api/api-keys, in order.
		await expect(nav).toHaveTextContent("Dashboard");
		await expect(nav).toHaveTextContent("Products");
		await expect(nav).toHaveTextContent("Checkout Api");
		await expect(nav).toHaveTextContent("Api Keys");

		// The last crumb is the non-navigable current-page marker: role="link"
		// with aria-disabled and no href.
		const current = crumbs.getByText("Api Keys").closest("[data-slot]");
		await expect(current).toHaveAttribute("aria-disabled", "true");
		await expect(current).not.toHaveAttribute("href");
	},
};

/**
 * KNOWN DEFECT — deliberately asserted as-is so the fix has a failing baseline.
 *
 * A breadcrumb trail must carry exactly ONE `aria-current="page"`, on the final
 * crumb. Here every intermediate crumb is a TanStack `<Link>`, and the router
 * marks a link active on a PREFIX match — so at /products/checkout-api/api-keys
 * the /products and /products/checkout-api links both get `aria-current="page"`
 * too, on top of the `BreadcrumbPage`'s own. A screen reader is told three
 * different elements are the current page.
 *
 * The fix is `activeOptions={{ exact: true }}` on the crumb links. When that
 * lands, change the expected count to 1 — do not delete this story.
 */
export const BreadcrumbsOverclaimCurrentPage: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const nav = canvas.getByRole("navigation", { name: "Breadcrumb" });

		const current = nav.querySelectorAll('[aria-current="page"]');
		await expect(current).toHaveLength(3);
	},
};

export const WithActions: Story = {
	args: {
		actions: <Button>Create key</Button>,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", { name: "Create key" }),
		).toBeInTheDocument();
	},
};

export const WithPageInfo: Story = {
	args: {
		pageInfo: {
			title: "Keys are shown once",
			description: "Copy the secret before closing the dialog.",
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText("Keys are shown once")).toBeInTheDocument();
	},
};

export const WithoutBreadcrumbs: Story = {
	args: {
		breadCrumbs: false,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.queryByRole("navigation", { name: "Breadcrumb" }),
		).not.toBeInTheDocument();
	},
};

/**
 * `full` is the default and spans the region; the constrained variants are what
 * settings and detail routes use. Widths are a class concern, so the story is
 * here to be looked at rather than asserted.
 */
export const NarrowVariant: Story = {
	args: {
		variant: "narrow",
		title: "Rename product",
	},
};
