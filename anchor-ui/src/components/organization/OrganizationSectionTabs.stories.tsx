import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { OrganizationSectionTabs } from "./OrganizationSectionTabs";

const meta = {
	title: "Organization/OrganizationSectionTabs",
	component: OrganizationSectionTabs,
	args: { organizationId: "org_2qL9aC3mE8xP1rT6vN4bK2sJ", active: "license" },
	decorators: [
		(Story) => (
			<StoryRouter>
				<Story />
			</StoryRouter>
		),
	],
} satisfies Meta<typeof OrganizationSectionTabs>;

export default meta;
type Story = StoryObj<typeof meta>;

/** License is the current section; Roles links to the organization's roles. */
export const License: Story = {
	play: async ({ canvasElement }) => {
		const nav = within(canvasElement).getByRole("navigation", {
			name: "Organization sections",
		});
		await expect(
			within(nav).getByRole("link", { name: "License" }),
		).toHaveAttribute("aria-current", "page");
		await expect(
			within(nav).getByRole("link", { name: "Roles" }),
		).toHaveAttribute(
			"href",
			"/organizations/license/org_2qL9aC3mE8xP1rT6vN4bK2sJ/roles",
		);
	},
};

export const Roles: Story = {
	args: { active: "roles" },
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByRole("link", { name: "Roles" }),
		).toHaveAttribute("aria-current", "page");
	},
};
