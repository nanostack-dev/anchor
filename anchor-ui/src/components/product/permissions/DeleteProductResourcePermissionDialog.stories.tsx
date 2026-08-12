import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { DeleteProductResourcePermissionDialog } from "./DeleteProductResourcePermissionDialog";

const meta = {
	title: "Product/DeleteProductResourcePermissionDialog",
	component: DeleteProductResourcePermissionDialog,
	tags: ["autodocs"],
	args: {
		productId: "prd_2Nq8xKf3pLmR",
		permission: {
			product_id: "prd_2Nq8xKf3pLmR",
			name: "invoices:read",
			description: "Read invoices belonging to the organization",
			created_at: "2026-07-14T09:12:00Z",
			updated_at: "2026-07-14T09:12:00Z",
		},
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof DeleteProductResourcePermissionDialog>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * The icon-only row trigger carries an accessible name. The icon alone is not
 * one — without the `sr-only` text a screen reader announces only "button".
 */
export const Default: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("button", { name: "Delete permission" }),
		).toBeInTheDocument();
	},
};

/**
 * The confirm button is destructive, not primary.
 *
 * `AlertDialogAction` is a plain `Button` and `Button`'s default variant is
 * `bg-primary`, so leaving it unstyled — which is what this dialog did — painted
 * the irreversible "Delete Permission" action in the brand's affirmative colour,
 * the same treatment "Save" and "Create" get. Every other delete confirmation in
 * the app uses the destructive variant. Asserting on the class attribute rather
 * than a computed style keeps this honest in the browser runner, where a
 * component's own stylesheet is not loaded.
 */
export const ConfirmIsDestructive: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Delete permission" }),
		);

		// Dialog content is portalled outside canvasElement.
		await expect(
			await screen.findByRole("heading", { name: "Delete Product Permission" }),
		).toBeInTheDocument();

		const confirm = screen.getByRole("button", { name: /Delete Permission/ });
		await expect(confirm).toHaveClass("text-destructive");
		await expect(confirm).not.toHaveClass("bg-primary");
	},
};

/**
 * The dialog states which permission is going away and what it costs. The name
 * is the thing the user matches against the row they clicked, so it appears in
 * the body and not only in the prose.
 */
export const NamesThePermissionAndTheConsequence: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Delete permission" }),
		);
		await screen.findByRole("heading", { name: "Delete Product Permission" });

		await expect(screen.getByText("invoices:read")).toBeInTheDocument();
		await expect(
			screen.getByText(/will lose this permission immediately/),
		).toBeInTheDocument();
		await expect(
			screen.getByText("Read invoices belonging to the organization"),
		).toBeInTheDocument();
	},
};

/**
 * Cancelling closes without deleting.
 */
export const CancelDismisses: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Delete permission" }),
		);
		await userEvent.click(
			await screen.findByRole("button", { name: "Cancel" }),
		);

		await expect(
			screen.queryByRole("heading", { name: "Delete Product Permission" }),
		).not.toBeInTheDocument();
	},
};
