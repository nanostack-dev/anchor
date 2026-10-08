import type { WorkflowActionResponse } from "@/client";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";
import { fixtureCatalog } from "../workflow-fixtures";
import { StepPalette, type StepPalettePlacement } from "./StepPalette";
import { ACTION_DRAG_TYPE } from "./canvas-context";

const action = (
	type: string,
	name: string,
	group: string,
): WorkflowActionResponse => ({
	type,
	name,
	group,
	description: "",
	writes: true,
	emits: [],
	params: [],
	outputs: [],
});

/**
 * The whole server catalog, plus a group the canvas has no icon for, a
 * translated-length name and an unbreakable one.
 */
const everyAction: WorkflowActionResponse[] = [
	action("organization.get", "Read organization", "Organizations"),
	action("organization.create", "Create organization", "Organizations"),
	action("organization.update", "Update organization", "Organizations"),
	action("workspace.create", "Create workspace", "Workspaces"),
	action("member.add", "Add member", "Members"),
	action("member.update_role", "Change member role", "Members"),
	action("member.remove", "Remove member", "Members"),
	action("invitation.create", "Invite to organization", "Members"),
	action("product_user.get", "Read product user", "Users"),
	action("license.instantiate", "License organization", "Licensing"),
	action("license.migrate", "Move to license template", "Licensing"),
	action("license.adjust", "Adjust license", "Licensing"),
	action("email.send", "Send email", "Email"),
	action("workflow.emit", "Start other workflows", "Custom"),
	action("http.request", "Call your backend", "Custom"),
	action(
		"partner.sync",
		"Synchronise Aleksandra Wiśniewska-Kowalczyk's partner-programme organisations with the billing reference",
		"Partner programme and reseller agreements",
	),
	action(
		"partner.notify",
		"notify_bartholomew.fitzgerald@northwind-industries-holdings.example.com",
		"Partner programme and reseller agreements",
	),
	action("x.go", "Go", "Partner programme and reseller agreements"),
];

function PaletteStory({
	actions,
	placement,
	onAdd,
	onDragChange,
}: {
	actions: WorkflowActionResponse[];
	placement: StepPalettePlacement;
	onAdd: (action: WorkflowActionResponse) => void;
	onDragChange: (dragging: boolean) => void;
}) {
	const [open, setOpen] = useState(true);
	return (
		<Box className="relative h-[560px] w-full max-w-[880px] overflow-hidden rounded-xl border border-border bg-surface-subtle">
			<StepPalette
				actions={actions}
				open={open}
				onOpenChange={setOpen}
				placement={placement}
				onAdd={onAdd}
				onDragChange={onDragChange}
			/>
		</Box>
	);
}

const meta = {
	title: "Workflow/StepPalette",
	component: PaletteStory,
	parameters: { layout: "padded" },
	args: {
		actions: fixtureCatalog.actions,
		placement: "panel",
		onAdd: fn(),
		onDragChange: fn(),
	},
} satisfies Meta<typeof PaletteStory>;

export default meta;

type Story = StoryObj<typeof meta>;

const isInside = (inner: Element, outer: Element) => {
	const child = inner.getBoundingClientRect();
	const parent = outer.getBoundingClientRect();
	return child.left >= parent.left - 0.5 && child.right <= parent.right + 0.5;
};

export const Demo: Story = {
	play: async ({ canvasElement, args }) => {
		const canvas = within(canvasElement);
		const users = canvas.getByRole("list", { name: "Users" });
		const readUser = within(users).getByRole("button", {
			name: "Add step: Read product user",
		});
		await expect(readUser).toHaveAttribute("draggable", "true");

		await userEvent.click(readUser);
		await expect(args.onAdd).toHaveBeenCalledWith(
			expect.objectContaining({ type: "product_user.get" }),
		);

		const dataTransfer = new DataTransfer();
		readUser.dispatchEvent(
			new DragEvent("dragstart", { bubbles: true, dataTransfer }),
		);
		await expect(dataTransfer.getData(ACTION_DRAG_TYPE)).toBe(
			"product_user.get",
		);
		await expect(dataTransfer.getData("text/plain")).toBe("Read product user");
		await expect(args.onDragChange).toHaveBeenLastCalledWith(true);
		readUser.dispatchEvent(
			new DragEvent("dragend", { bubbles: true, dataTransfer }),
		);
		await expect(args.onDragChange).toHaveBeenLastCalledWith(false);

		const toggle = canvas.getByRole("button", { name: "Steps" });
		await expect(toggle).toHaveAttribute("aria-expanded", "true");
		await userEvent.click(toggle);
		await expect(toggle).toHaveAttribute("aria-expanded", "false");
		await waitFor(() =>
			expect(canvas.queryByRole("region", { name: "Steps to add" })).toBeNull(),
		);
		await userEvent.click(toggle);
		const reopened = canvas.getByRole("region", { name: "Steps to add" });
		await waitFor(() => expect(getComputedStyle(reopened).opacity).toBe("1"));
	},
};

/**
 * Every catalog action and a few that are not, in an unknown group: the panel
 * scrolls inside its height, and long or unbreakable names wrap inside it.
 */
export const WorstCase: Story = {
	args: { actions: everyAction },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const palette = canvas.getByRole("region", { name: "Steps to add" });
		await expect(
			within(palette).getAllByRole("button", { name: /^Add step: / }),
		).toHaveLength(everyAction.length);
		await expect(within(palette).getAllByRole("list")).toHaveLength(8);
		const list = palette.querySelector(
			'[data-slot="scroll-area-viewport"]',
		) as HTMLElement;
		await expect(list.scrollHeight).toBeGreaterThan(list.clientHeight);
		await expect(palette.getBoundingClientRect().bottom).toBeLessThanOrEqual(
			canvasElement.getBoundingClientRect().bottom,
		);
		for (const name of [everyAction[15].name, everyAction[16].name]) {
			await expect(
				isInside(
					canvas.getByRole("button", { name: `Add step: ${name}` }),
					palette,
				),
			).toBe(true);
		}
		await expect(
			canvas.getByRole("button", { name: "Add step: Go" }),
		).toBeInTheDocument();
	},
};

/** The narrow-canvas tray: it spans the bottom and keeps most of the canvas in view. */
export const Tray: Story = {
	args: { actions: everyAction, placement: "tray" },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const palette = canvas.getByRole("region", { name: "Steps to add" });
		const frame = palette.parentElement as HTMLElement;
		await expect(palette.getBoundingClientRect().height).toBeLessThanOrEqual(
			frame.getBoundingClientRect().height * 0.4 + 1,
		);
		await expect(
			isInside(
				canvas.getByRole("button", {
					name: `Add step: ${everyAction[16].name}`,
				}),
				palette,
			),
		).toBe(true);
	},
};

export const Empty: Story = {
	args: { actions: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByText("No steps are available to add."),
		).toBeVisible();
		await expect(
			canvas.queryAllByRole("button", { name: /^Add step: / }),
		).toHaveLength(0);
	},
};
