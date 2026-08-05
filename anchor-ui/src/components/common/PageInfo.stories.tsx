import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { StoryRouter } from "@/lib/storybook/story-router";

import { PageInfo } from "./PageInfo";

const meta = {
	title: "Common/PageInfo",
	component: PageInfo,
	tags: ["autodocs"],
	// PageInfo renders a router `<Link>` when given one, so every story needs a
	// router in scope even if it never navigates.
	decorators: [
		(Story) => (
			<StoryRouter>
				<Story />
			</StoryRouter>
		),
	],
	args: {
		title: "Products scope every API key",
		description:
			"A key issued here can only read and write inside the product it belongs to.",
	},
	parameters: {
		layout: "padded",
	},
} satisfies Meta<typeof PageInfo>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("Products scope every API key"),
		).toBeInTheDocument();
		await expect(canvas.queryByRole("link")).not.toBeInTheDocument();
	},
};

/**
 * The link only renders when BOTH `linkTo` and `linkText` are supplied — a
 * half-configured link should stay invisible rather than render a dead anchor.
 */
export const WithLink: Story = {
	args: {
		linkTo: "/products",
		linkText: "Manage products",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const link = canvas.getByRole("link", { name: "Manage products" });
		await expect(link).toHaveAttribute("href", "/products");
	},
};

export const LinkTextWithoutTarget: Story = {
	args: {
		linkText: "Manage products",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.queryByRole("link")).not.toBeInTheDocument();
	},
};
