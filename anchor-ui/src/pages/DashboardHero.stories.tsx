import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { DashboardHero } from "./DashboardHero";

const meta = {
	title: "Pages/DashboardHero",
	component: DashboardHero,
	tags: ["autodocs"],
	args: {
		subtitle: "Welcome back, jamie@nanostack.dev",
	},
	parameters: {
		layout: "padded",
	},
} satisfies Meta<typeof DashboardHero>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * The heading carries the page's identity on its own — no eyebrow above it.
 * "Anchor · Organization-as-a-Service" restated app branding the sidebar
 * already shows on every route, so it added noise, not clarity.
 */
export const WelcomeBack: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("heading", { name: "Dashboard", level: 1 }),
		).toBeVisible();
		await expect(
			canvas.getByText("Welcome back, jamie@nanostack.dev"),
		).toBeInTheDocument();
		await expect(
			canvas.queryByText(/Organization-as-a-Service/i),
		).not.toBeInTheDocument();
	},
};

export const NoUserYet: Story = {
	args: {
		subtitle: "An overview of your workspace.",
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("An overview of your workspace."),
		).toBeInTheDocument();
	},
};
