import {
	type AccessBackend,
	DOCUMENT_TREE,
	type RecordedCall,
	STORY_PRODUCT_ID,
	mockAccessBackend,
} from "@/lib/storybook/access-backend";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, screen, userEvent, waitFor, within } from "storybook/test";

import { CreateProductResourcePermissionForm } from "./CreateProductResourcePermissionForm";

let calls: RecordedCall[] = [];

const meta = {
	title: "Product/CreateProductResourcePermissionForm",
	component: CreateProductResourcePermissionForm,
	parameters: {
		layout: "padded",
		backend: { types: DOCUMENT_TREE } satisfies AccessBackend,
	},
	args: { productId: STORY_PRODUCT_ID, onCreated: fn(), onCancel: fn() },
	beforeEach: ({ parameters }) => {
		const backend = mockAccessBackend(parameters.backend as AccessBackend);
		calls = backend.calls;
		return backend.restore;
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<div style={{ maxWidth: 640 }}>
					<Story />
				</div>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof CreateProductResourcePermissionForm>;

export default meta;
type Story = StoryObj<typeof meta>;

/** No scope is picked for the user; choosing one pre-fills the name with its slug. */
export const ChooseScopeFirst: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		const select = canvas.getByRole("combobox", { name: "Resource type" });
		await waitFor(() => expect(select).toBeEnabled());
		await expect(select).toHaveTextContent("Choose a resource type");
		await expect(
			canvas.getByRole("button", { name: "Create permission" }),
		).toBeDisabled();
		await userEvent.click(select);
		await userEvent.click(
			await screen.findByRole("option", { name: "Folder" }),
		);
		const name = canvas.getByRole("textbox", { name: "Name" });
		await expect(name).toHaveValue("folder:");
		await userEvent.type(name, "view");
		await userEvent.click(
			canvas.getByRole("button", { name: "Create permission" }),
		);
		await waitFor(() => expect(args.onCreated).toHaveBeenCalled());
		await expect(
			calls.find((call) => call.path.endsWith("/resource-permissions"))?.body,
		).toMatchObject({ name: "folder:view", resource_type: "folder" });
	},
};

/** Opened from a type, the form starts on that type. */
export const FromAType: Story = {
	args: { initialResourceType: "document" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("combobox", { name: "Resource type" }),
			).toHaveTextContent("Document"),
		);
		await expect(canvas.getByRole("textbox", { name: "Name" })).toHaveValue(
			"document:",
		);
	},
};

/** A refused create keeps the form and says why. */
export const Refused: Story = {
	args: { initialResourceType: "document" },
	parameters: {
		backend: { types: DOCUMENT_TREE, saveStatus: 409 } satisfies AccessBackend,
	},
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		await userEvent.type(canvas.getByRole("textbox", { name: "Name" }), "view");
		await userEvent.click(
			canvas.getByRole("button", { name: "Create permission" }),
		);
		await expect(
			await canvas.findByText("A permission with this name already exists."),
		).toBeVisible();
		await expect(args.onCreated).not.toHaveBeenCalled();
	},
};

export const TypesFail: Story = {
	parameters: { backend: { types: "fail" } satisfies AccessBackend },
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};

/** A type the product doesn't declare, from a stale link, is dropped instead of submitted. */
export const UnknownType: Story = {
	args: { initialResourceType: "workspace" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(() =>
			expect(
				canvas.getByRole("combobox", { name: "Resource type" }),
			).toHaveTextContent("Choose a resource type"),
		);
		await expect(
			canvas.getByRole("button", { name: "Create permission" }),
		).toBeDisabled();
	},
};
