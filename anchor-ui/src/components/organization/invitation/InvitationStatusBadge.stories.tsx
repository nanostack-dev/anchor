import { OrganizationInvitationStatus } from "@/client";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { InvitationStatusBadge } from "./InvitationStatusBadge";

const meta = {
	title: "Organization/InvitationStatusBadge",
	component: InvitationStatusBadge,
	tags: ["autodocs"],
	args: { status: OrganizationInvitationStatus.PENDING },
} satisfies Meta<typeof InvitationStatusBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Pending: Story = {
	play: async ({ canvasElement }) => {
		await expect(within(canvasElement).getByText("Pending")).toBeVisible();
	},
};

export const Accepted: Story = {
	args: { status: OrganizationInvitationStatus.ACCEPTED },
	play: async ({ canvasElement }) => {
		await expect(within(canvasElement).getByText("Accepted")).toBeVisible();
	},
};

export const Expired: Story = {
	args: { status: OrganizationInvitationStatus.EXPIRED },
	play: async ({ canvasElement }) => {
		await expect(within(canvasElement).getByText("Expired")).toBeVisible();
	},
};

/**
 * Each status reads as a distinct label, so the table never depends on colour
 * alone to tell a pending invitation from an expired one.
 */
export const AllStatuses: Story = {
	render: () => (
		<Box className="flex gap-2">
			{Object.values(OrganizationInvitationStatus).map((status) => (
				<InvitationStatusBadge key={status} status={status} />
			))}
		</Box>
	),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		for (const label of ["Pending", "Accepted", "Expired"]) {
			await expect(canvas.getByText(label)).toBeVisible();
		}
	},
};
