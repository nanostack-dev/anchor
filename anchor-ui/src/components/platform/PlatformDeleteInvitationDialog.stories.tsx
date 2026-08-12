import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { Button } from "../ui/button";
import { PlatformDeleteInvitationDialog } from "./PlatformDeleteInvitationDialog";

const meta = {
	title: "Platform/PlatformDeleteInvitationDialog",
	component: PlatformDeleteInvitationDialog,
	tags: ["autodocs"],
	args: {
		invitationId: "inv_2Nq8xKf3pLmR",
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof PlatformDeleteInvitationDialog>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * The row action is ONE button.
 *
 * It used to be two. The component wrapped whatever the caller passed in a bare
 * `<button type="button" onClick={...}>` before handing it to
 * `AlertDialogTrigger`, and the invitations table passed a `Tooltip` that itself
 * rendered a `Button` — so the DOM held a `<button>` inside a `<button>`. That is
 * invalid HTML, it puts two tab stops on one control, and assistive tech
 * announces a button nested in a button of the same name.
 *
 * Counting the buttons is the assertion: it only passes while the trigger
 * renders a single element.
 */
export const Default: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const trigger = canvas.getByRole("button", { name: "Delete invitation" });
		await expect(trigger).toBeInTheDocument();
		await expect(canvas.getAllByRole("button")).toHaveLength(1);
		await expect(trigger.querySelector("button")).toBeNull();
	},
};

/**
 * The confirmation names the action and its consequence, and the affirmative
 * control is styled as destructive rather than as the default primary action.
 *
 * `AlertDialogAction` is a plain `Button`, whose default variant is
 * `bg-primary` — so an unstyled confirm reads as the safe, encouraged choice on
 * an irreversible delete. Asserting the variant on the class attribute (not a
 * computed style) pins that it carries the destructive treatment.
 */
export const OpensConfirmation: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Delete invitation" }),
		);

		// Dialog content is portalled outside canvasElement.
		await expect(
			await screen.findByRole("heading", { name: "Delete Invitation?" }),
		).toBeInTheDocument();
		await expect(
			screen.getByText(/This action cannot be undone/),
		).toBeInTheDocument();

		const confirm = screen.getByRole("button", { name: "Delete" });
		await expect(confirm).toHaveClass("text-destructive");
		await expect(confirm).not.toHaveClass("bg-primary");

		await expect(
			screen.getByRole("button", { name: "Cancel" }),
		).toBeInTheDocument();
	},
};

/**
 * Cancelling is a real escape route, not just a visual one — the dialog closes
 * and nothing is deleted.
 */
export const CancelDismisses: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", { name: "Delete invitation" }),
		);
		await userEvent.click(
			await screen.findByRole("button", { name: "Cancel" }),
		);

		await expect(
			screen.queryByRole("heading", { name: "Delete Invitation?" }),
		).not.toBeInTheDocument();
	},
};

/**
 * A caller-supplied trigger still opens the dialog.
 *
 * This is the contract the nested-`<button>` wrapper was there to fake: Base UI
 * hands the trigger's own props to whatever element it renders, so the passed
 * element has to be a single DOM-forwarding one. `Button` qualifies; a `Tooltip`
 * root or another composite does not, and would swallow the open behaviour
 * silently.
 */
export const CustomTrigger: Story = {
	args: {
		trigger: <Button variant="outlineDestructive">Revoke invite</Button>,
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const trigger = canvas.getByRole("button", { name: "Revoke invite" });
		await expect(canvas.getAllByRole("button")).toHaveLength(1);

		await userEvent.click(trigger);
		await expect(
			await screen.findByRole("heading", { name: "Delete Invitation?" }),
		).toBeInTheDocument();
	},
};
