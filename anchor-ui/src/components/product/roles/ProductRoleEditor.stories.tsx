import {
	type AccessBackend,
	DOCUMENT_PERMISSIONS,
	DOCUMENT_ROLES,
	DOCUMENT_TREE,
	FIFTY_ONE_TYPES,
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
import { expect, fn, screen, userEvent, waitFor, within } from "storybook/test";

import { ProductRoleEditor } from "./ProductRoleEditor";

let calls: RecordedCall[] = [];

const meta = {
	title: "Product/ProductRoleEditor",
	component: ProductRoleEditor,
	parameters: {
		layout: "padded",
		backend: {
			types: DOCUMENT_TREE,
			permissions: DOCUMENT_PERMISSIONS,
			roles: DOCUMENT_ROLES,
		} satisfies AccessBackend,
	},
	args: {
		productId: STORY_PRODUCT_ID,
		mode: "create",
		onSaved: fn(),
		onCancel: fn(),
	},
	beforeEach: ({ parameters }) => {
		const backend = mockAccessBackend(parameters.backend as AccessBackend);
		calls = backend.calls;
		return backend.restore;
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<StoryRouter>
					<div style={{ maxWidth: 760 }}>
						<Story />
					</div>
				</StoryRouter>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductRoleEditor>;

export default meta;
type Story = StoryObj<typeof meta>;

async function chooseScope(canvasElement: HTMLElement, name: string) {
	const canvas = within(canvasElement);
	const select = canvas.getByRole("combobox", { name: "Resource type" });
	await waitFor(() => expect(select).toBeEnabled());
	await userEvent.click(select);
	await userEvent.click(await screen.findByRole("option", { name }));
}

/** The wizard starts with no scope, and can't continue until the user names one. */
export const ScopeRequired: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const select = canvas.getByRole("combobox", { name: "Resource type" });
		await waitFor(() => expect(select).toBeEnabled());
		await expect(select).toHaveTextContent("Choose a resource type");
		await expect(
			canvas.getByRole("button", { name: "Continue" }),
		).toBeDisabled();
		await chooseScope(canvasElement, "Folder");
		await expect(
			canvas.getByText("Not offered: Organization, Project.", { exact: false }),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Continue" }),
		).toBeEnabled();
	},
};

/** Creating a project role sends its scope and only the permissions it may hold. */
export const CreatesScopedRole: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await chooseScope(canvasElement, "Project");
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Name" }),
			"Release manager",
		);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.click(
			await canvas.findByRole("checkbox", { name: "project:manage" }),
		);
		await userEvent.click(
			canvas.getByRole("checkbox", { name: "document:view" }),
		);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.click(canvas.getByRole("button", { name: "Create role" }));
		await waitFor(() => expect(args.onSaved).toHaveBeenCalled());
		const created = calls.find(
			(call) => call.method === "POST" && call.path.endsWith("/roles"),
		);
		await expect(created?.body).toMatchObject({
			name: "Release manager",
			resource_type: "project",
			permissions: ["project:manage", "document:view"],
		});
		const search = calls.find((call) =>
			call.path.endsWith("/resource-permissions/search"),
		);
		await expect(search?.body.filter).toEqual({
			resource_types: ["project", "folder", "document"],
		});
	},
};

/** Editing keeps the scope fixed and offers the same filtered permissions. */
export const EditKeepsScope: Story = {
	args: {
		mode: "edit",
		existingRole: DOCUMENT_ROLES[3],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("textbox", { name: "Resource type" }),
			).toHaveValue("Folder"),
		);
		await expect(
			canvas.queryByRole("combobox", { name: "Resource type" }),
		).toBeNull();
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await expect(
			await canvas.findByRole("checkbox", { name: "folder:view" }),
		).toBeChecked();
		await expect(
			canvas.queryByRole("checkbox", { name: "project:manage" }),
		).toBeNull();
	},
};

/** A refused create keeps the draft and says why. */
export const CreateRefused: Story = {
	parameters: {
		backend: {
			types: DOCUMENT_TREE,
			permissions: DOCUMENT_PERMISSIONS,
			roles: DOCUMENT_ROLES,
			saveStatus: 409,
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await chooseScope(canvasElement, "Document");
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Name" }),
			"viewer",
		);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.click(
			await canvas.findByRole("button", { name: "Continue" }),
		);
		await userEvent.click(canvas.getByRole("button", { name: "Create role" }));
		await expect(
			await canvas.findByText("A role with this name already exists."),
		).toBeVisible();
		await expect(args.onSaved).not.toHaveBeenCalled();
	},
};

/** The largest catalog still lists every type in the scope picker, in tree order. */
export const FiftyOneTypes: Story = {
	parameters: {
		backend: {
			types: FIFTY_ONE_TYPES,
			permissions: [],
			roles: [],
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		const select = within(canvasElement).getByRole("combobox", {
			name: "Resource type",
		});
		await waitFor(() => expect(select).toBeEnabled());
		await userEvent.click(select);
		const options = await screen.findAllByRole("option");
		await expect(options).toHaveLength(51);
		await expect(options[0]).toHaveTextContent("Organization");
	},
};

/** While the types load, nothing can be chosen. */
export const TypesLoading: Story = {
	parameters: { backend: { types: "pending" } satisfies AccessBackend },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("combobox", { name: "Resource type" }),
		).toBeDisabled();
		await expect(
			canvas.getByRole("button", { name: "Continue" }),
		).toBeDisabled();
	},
};

/** When the types fail to load, nothing is picked for the user and the wizard stays on the scope. */
export const TypesFail: Story = {
	parameters: { backend: { types: "fail" } satisfies AccessBackend },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("button", { name: "Try again" }),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Continue" }),
		).toBeDisabled();
	},
};

/** An Organization role with no permission is allowed: a member role often holds nothing. */
export const NoPermissions: Story = {
	args: {
		mode: "edit",
		existingRole: storyRole("member", "organization", []),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			await canvas.findByRole("button", { name: "Continue" }),
		);
		await waitFor(() =>
			expect(canvas.getByRole("button", { name: "Continue" })).toBeEnabled(),
		);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await expect(
			canvas.getByText(/None\. Members holding this role/),
		).toBeVisible();
	},
};

/** Long type and permission names stay inside a phone-width review instead of widening the page. */
export const LongNamesOnPhone: Story = {
	parameters: {
		backend: {
			types: [
				storyType("organization", "Organization"),
				storyType("evidence", LONG_NAME),
			],
			permissions: [
				storyPermission(`evidence:${LONG_NAME.toLowerCase()}`, "evidence"),
			],
			roles: [],
		} satisfies AccessBackend,
	},
	decorators: [
		(Story) => (
			<div style={{ width: 358 }}>
				<Story />
			</div>
		),
	],
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await chooseScope(canvasElement, LONG_NAME);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Name" }),
			"Auditor",
		);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await userEvent.click(
			await canvas.findByRole("checkbox", {
				name: `evidence:${LONG_NAME.toLowerCase()}`,
			}),
		);
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await expect(
			canvas.getByRole("button", { name: "Create role" }),
		).toBeVisible();
		const frame = canvasElement.firstElementChild as HTMLElement;
		await expect(frame.scrollWidth).toBeLessThanOrEqual(frame.clientWidth + 1);
	},
};

/** In edit mode, types that fail to load offer a retry instead of loading forever. */
export const EditTypesFail: Story = {
	args: { mode: "edit", existingRole: DOCUMENT_ROLES[3] },
	parameters: {
		backend: {
			types: "fail",
			permissions: DOCUMENT_PERMISSIONS,
			roles: DOCUMENT_ROLES,
		} satisfies AccessBackend,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("button", { name: "Try again" }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Continue" }));
		await expect(
			await canvas.findByText("Couldn't load the resource types"),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Continue" }),
		).toBeDisabled();
	},
};
