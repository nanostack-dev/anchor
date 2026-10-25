import { OrganizationRoleOrigin } from "@/client";
import {
	type AccessBackend,
	DOCUMENT_ROLES,
	DOCUMENT_TREE,
	LONG_NAME,
	STORY_ORGANIZATION_ID,
	STORY_PRODUCT_ID,
	mockAccessBackend,
	storyOrganizationRole,
	storyRole,
} from "@/lib/storybook/access-backend";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor, within } from "storybook/test";

import { OrganizationRolesTable } from "./OrganizationRolesTable";

const ROLES = [
	storyOrganizationRole(DOCUMENT_ROLES[0], {
		customized: true,
		description: "Our own admin.",
	}),
	...DOCUMENT_ROLES.slice(1).map((role) => storyOrganizationRole(role)),
	storyOrganizationRole(
		storyRole("Billing auditor", "organization", [
			"billing:read",
			"document:view",
		]),
		{
			id: "orole_billing",
			template_id: undefined,
			origin: OrganizationRoleOrigin.CUSTOM,
		},
	),
];

const meta = {
	title: "Organization/OrganizationRolesTable",
	component: OrganizationRolesTable,
	parameters: {
		layout: "padded",
		backend: {
			types: DOCUMENT_TREE,
			organizationRoles: ROLES,
		} satisfies AccessBackend,
	},
	args: { productId: STORY_PRODUCT_ID, organizationId: STORY_ORGANIZATION_ID },
	beforeEach: ({ parameters }) =>
		mockAccessBackend(parameters.backend as AccessBackend).restore,
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof OrganizationRolesTable>;

export default meta;
type Story = StoryObj<typeof meta>;

const rowOf = (canvasElement: HTMLElement, name: string) =>
	within(canvasElement)
		.getAllByRole("row")
		.find(
			(row) => within(row).queryAllByText(name, { exact: true }).length > 0,
		) as HTMLElement;

/** Each role is marked template, customized or custom, read only. */
export const Origins: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findAllByText("Billing auditor");
		await expect(
			within(rowOf(canvasElement, "admin")).getByText("Customized"),
		).toBeVisible();
		await expect(
			within(rowOf(canvasElement, "member")).getByText("Template"),
		).toBeVisible();
		await expect(
			within(rowOf(canvasElement, "member")).getByText("Default"),
		).toBeVisible();
		await expect(
			within(rowOf(canvasElement, "Billing auditor")).getByText("Custom"),
		).toBeVisible();
		await waitFor(() =>
			expect(
				within(rowOf(canvasElement, "project-admin")).getByText("Project"),
			).toBeVisible(),
		);
		await expect(
			canvas.queryByRole("checkbox", { name: "Select row" }),
		).toBeNull();
		await expect(
			canvas.queryByRole("button", { name: /^(Create|Delete|Edit)/ }),
		).toBeNull();
	},
};

export const LongNames: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			organizationRoles: [
				storyOrganizationRole(
					storyRole(LONG_NAME, "organization", [], {
						key: LONG_NAME.toLowerCase(),
					}),
					{ origin: OrganizationRoleOrigin.CUSTOM, template_id: undefined },
				),
			],
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		const [name] = await within(canvasElement).findAllByText(LONG_NAME, {
			selector: "span",
		});
		await expect(name.getBoundingClientRect().width).toBeLessThanOrEqual(256);
	},
};

export const Empty: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			organizationRoles: [],
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByText("No roles yet"),
		).toBeVisible();
	},
};

export const Loading: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			organizationRoles: "pending",
		} satisfies AccessBackend,
	},
};

export const LoadFails: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			organizationRoles: "fail",
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};
