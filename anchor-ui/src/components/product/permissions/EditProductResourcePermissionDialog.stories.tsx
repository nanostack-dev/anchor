import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { TooltipProvider } from "../../ui/tooltip";
import { EditProductResourcePermissionDialog } from "./EditProductResourcePermissionDialog";

const meta = {
	title: "Product/EditProductResourcePermissionDialog",
	component: EditProductResourcePermissionDialog,
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
				<TooltipProvider>
					<Story />
				</TooltipProvider>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof EditProductResourcePermissionDialog>;

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
			canvas.getByRole("button", { name: "Edit permission" }),
		).toBeInTheDocument();
	},
};

/**
 * Hovering the trigger shows the "Edit permission" tooltip; unhovering hides
 * it again.
 *
 * The row used to wrap this trigger in a `Tooltip` whose `render` target was
 * the whole `EditProductResourcePermissionDialog` component. `TooltipTrigger`
 * clones its hover/focus handlers onto whatever `render` points at, but a
 * custom component only receives them as inert props on its own interface —
 * they never reached the `<button>` the dialog actually renders, so the
 * tooltip never opened. Fixed by moving the `Tooltip` inside this component,
 * around `DialogTrigger` (which forwards `{...props}`) instead of wrapping
 * the component from outside.
 *
 * Two elements read "Edit permission" once open: the trigger's `sr-only`
 * label and the tooltip popup content, so the pin counts occurrences rather
 * than asserting a role — the tooltip popup carries no ARIA role of its own.
 */
export const TooltipAppearsOnHover: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const trigger = canvas.getByRole("button", { name: "Edit permission" });

		await expect(screen.getAllByText("Edit permission")).toHaveLength(1);

		await userEvent.hover(trigger);
		await waitFor(() => {
			expect(screen.getAllByText("Edit permission")).toHaveLength(2);
		});

		await userEvent.unhover(trigger);
		await waitFor(() => {
			expect(screen.getAllByText("Edit permission")).toHaveLength(1);
		});
	},
};

/**
 * Clicking the trigger still opens the edit dialog, pre-filled with the
 * current description — wrapping it in a `Tooltip` must not swallow the
 * dialog's own open behaviour.
 */
export const OpensEditDialog: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Edit permission" }),
		);

		await expect(
			await screen.findByRole("heading", { name: "Edit Product Permission" }),
		).toBeInTheDocument();
		await expect(
			screen.getByDisplayValue("Read invoices belonging to the organization"),
		).toBeInTheDocument();
	},
};
