import {
	type AccessBackend,
	DOCUMENT_PERMISSIONS,
	DOCUMENT_ROLES,
	DOCUMENT_TREE,
	LONG_NAME,
	type RecordedCall,
	STORY_PRODUCT_ID,
	mockAccessBackend,
	storyPermission,
	storyRole,
	storyType,
} from "@/lib/storybook/access-backend";
import { StoryQuery } from "@/lib/storybook/story-query";
import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor, within } from "storybook/test";

import { ProductResourcePermissionDatatable } from "./ProductResourcePermissionDatatable";

let calls: RecordedCall[] = [];

const meta = {
	title: "Product/ProductResourcePermissionDatatable",
	component: ProductResourcePermissionDatatable,
	parameters: {
		layout: "padded",
		backend: {
			types: DOCUMENT_TREE,
			permissions: DOCUMENT_PERMISSIONS,
			roles: DOCUMENT_ROLES,
		} satisfies AccessBackend,
	},
	args: { productId: STORY_PRODUCT_ID },
	beforeEach: ({ parameters }) => {
		const backend = mockAccessBackend(parameters.backend as AccessBackend);
		calls = backend.calls;
		return backend.restore;
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<StoryRouter>
					<Story />
				</StoryRouter>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductResourcePermissionDatatable>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Each permission shows its scope and the roles that bundle it, from one roles search for the whole page. */
export const ScopesAndRoles: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const bundling = await canvas.findByLabelText(
			"Roles bundling document:view: admin, project-admin, folder-viewer, document-viewer",
			{},
			{ timeout: 5000 },
		);
		await expect(within(bundling).queryByText(/more$/)).toBeNull();
		await expect(canvas.getAllByText("Document").length).toBeGreaterThan(0);
		const roleSearches = calls.filter((call) =>
			call.path.endsWith("/roles/search"),
		);
		await expect(roleSearches).toHaveLength(1);
		await expect(
			(roleSearches[0].body.filter as { permissions: string[] }).permissions,
		).toHaveLength(DOCUMENT_PERMISSIONS.length);
	},
};

/** A permission no role bundles says so. */
export const NoRoles: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			permissions: DOCUMENT_PERMISSIONS,
			roles: [],
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await waitFor(() =>
			expect(within(canvasElement).getAllByText("No roles")).toHaveLength(
				DOCUMENT_PERMISSIONS.length,
			),
		);
	},
};

/** Filtered to one type from the canvas, Create Permission keeps that type. */
export const FilteredByType: Story = {
	args: { resourceTypes: ["folder"] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("folder:view")).toBeVisible();
		await expect(canvas.queryByText("billing:read")).toBeNull();
		await expect(
			canvas.getByRole("link", { name: "Create Permission" }),
		).toHaveAttribute(
			"href",
			"/products/resources/new-permission?resource_type=folder",
		);
	},
};

/** Many long role names stay inside the column: five are listed, the rest counted. */
export const ManyLongRoles: Story = {
	parameters: {
		backend: {
			types: [
				storyType("organization", "Organization"),
				storyType("evidence", LONG_NAME),
			],
			permissions: [
				storyPermission(`evidence:${LONG_NAME.toLowerCase()}`, "evidence"),
			],
			roles: Array.from({ length: 25 }, (_, index) =>
				storyRole(`${LONG_NAME} ${index}`, "organization", [
					`evidence:${LONG_NAME.toLowerCase()}`,
				]),
			),
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("+20 more")).toBeVisible();
	},
};

export const RolesLoading: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			permissions: DOCUMENT_PERMISSIONS,
			roles: "pending",
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await expect(
			(await within(canvasElement).findAllByText("Loading roles")).length,
		).toBe(DOCUMENT_PERMISSIONS.length);
	},
};

export const RolesFail: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			permissions: DOCUMENT_PERMISSIONS,
			roles: "fail",
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		await waitFor(() =>
			expect(
				within(canvasElement).getAllByText("Couldn't load roles"),
			).toHaveLength(DOCUMENT_PERMISSIONS.length),
		);
	},
};

export const Empty: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			permissions: [],
			roles: [],
		} satisfies AccessBackend,
	},
};

export const Loading: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			permissions: "pending",
			roles: [],
		} satisfies AccessBackend,
	},
};
