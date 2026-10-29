import {
	type AccessBackend,
	DOCUMENT_ROLES,
	DOCUMENT_TREE,
	FIFTY_ONE_TYPES,
	LONG_NAME,
	STORY_PRODUCT_ID,
	mockAccessBackend,
	storyRole,
	storyType,
} from "@/lib/storybook/access-backend";
import { StoryQuery } from "@/lib/storybook/story-query";
import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { ProductRoleDatatable } from "./ProductRoleDatatable";

const meta = {
	title: "Product/ProductRoleDatatable",
	component: ProductRoleDatatable,
	parameters: {
		layout: "padded",
		backend: {
			types: DOCUMENT_TREE,
			roles: DOCUMENT_ROLES,
		} satisfies AccessBackend,
	},
	args: { productId: STORY_PRODUCT_ID },
	beforeEach: ({ parameters }) =>
		mockAccessBackend(parameters.backend as AccessBackend).restore,
	decorators: [
		(Story) => (
			<StoryQuery>
				<StoryRouter>
					<Story />
				</StoryRouter>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductRoleDatatable>;

export default meta;
type Story = StoryObj<typeof meta>;

const rowOf = (canvasElement: HTMLElement, name: string) =>
	within(canvasElement)
		.getAllByRole("row")
		.find(
			(row) => within(row).queryAllByText(name, { exact: true }).length > 0,
		) as HTMLElement;

/** Each role shows its scope and how many permissions it holds. */
export const Scopes: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findAllByText("project-admin");
		await waitFor(() =>
			expect(
				within(rowOf(canvasElement, "project-admin")).getByText("Project"),
			).toBeVisible(),
		);
		await expect(
			within(rowOf(canvasElement, "project-admin")).getByRole("cell", {
				name: "5",
			}),
		).toBeVisible();
		await expect(
			canvas.getByRole("link", { name: "Create Role" }),
		).toHaveAttribute("href", "/products/resources/roles/new");
	},
};

/** A filter that came in the URL narrows the search to that scope. */
export const FilteredByType: Story = {
	args: { resourceTypes: ["folder"] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			(await canvas.findAllByText("folder-viewer"))[0],
		).toBeVisible();
		await expect(canvas.queryByText("project-admin")).toBeNull();
	},
};

/** The resource type filter lists every type in tree order, even fifty-one of them. */
export const FiftyOneTypeFilter: Story = {
	parameters: {
		backend: {
			types: FIFTY_ONE_TYPES,
			roles: [storyRole("Auditor", "type_49", [])],
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				within(rowOf(canvasElement, "Auditor")).getByText(
					"Resource type number 49",
				),
			).toBeVisible(),
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "Resource type" }),
		);
		const options = await screen.findAllByRole("option");
		await expect(options[0]).toHaveTextContent("Organization");
		await expect(options.length).toBeGreaterThanOrEqual(51);
	},
};

/** Long role and scope names wrap inside their column. */
export const LongNames: Story = {
	parameters: {
		backend: {
			types: [
				storyType("organization", "Organization"),
				storyType("evidence", LONG_NAME),
			],
			roles: [storyRole(LONG_NAME, "evidence", ["evidence:read"])],
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
		backend: { types: DOCUMENT_TREE, roles: [] } satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByText("No roles yet"),
		).toBeVisible();
	},
};

export const Loading: Story = {
	parameters: {
		backend: { types: DOCUMENT_TREE, roles: "pending" } satisfies AccessBackend,
	},
};

export const LoadFails: Story = {
	parameters: {
		backend: { types: DOCUMENT_TREE, roles: "fail" } satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByRole("button", {
				name: /Try again|Retry/,
			}),
		).toBeVisible();
	},
};
