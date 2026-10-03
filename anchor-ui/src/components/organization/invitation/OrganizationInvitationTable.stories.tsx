import type { OrganizationInvitationResponse } from "@/client";
import { OrganizationInvitationStatus } from "@/client";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import dayjs from "dayjs";
import { useState } from "react";
import { expect, fn, screen, userEvent, within } from "storybook/test";
import { waitFor } from "storybook/test";

import {
	EXPIRY_FORMAT,
	OrganizationInvitationTable,
} from "./OrganizationInvitationTable";

const ROLE_MEMBER = "role_2Nq8xKf3pLmR";
const ROLE_ADMIN = "role_2Nq8xKf3pLmS";

const roleNames = { [ROLE_MEMBER]: "Member", [ROLE_ADMIN]: "Admin" };

function invitation(
	overrides: Partial<OrganizationInvitationResponse>,
): OrganizationInvitationResponse {
	return {
		id: "oinv_2Nq8xKf3pLm0",
		organization_id: "org_2Nq8xKf3pLmR",
		email: "ada@example.com",
		role_id: ROLE_MEMBER,
		status: OrganizationInvitationStatus.PENDING,
		expires_at: dayjs().add(3, "day").toISOString(),
		created_at: "2026-09-01T09:00:00Z",
		updated_at: "2026-09-01T09:00:00Z",
		...overrides,
	};
}

const pendingInvitation = invitation({});
const acceptedInvitation = invitation({
	id: "oinv_2Nq8xKf3pLm1",
	email: "grace@example.com",
	role_id: ROLE_ADMIN,
	status: OrganizationInvitationStatus.ACCEPTED,
	accepted_at: "2026-09-02T09:00:00Z",
});
const expiredInvitation = invitation({
	id: "oinv_2Nq8xKf3pLm2",
	email: "alan@example.com",
	status: OrganizationInvitationStatus.EXPIRED,
	expires_at: dayjs().subtract(2, "day").toISOString(),
});
const allStatuses = [pendingInvitation, acceptedInvitation, expiredInvitation];

const meta = {
	title: "Organization/OrganizationInvitationTable",
	component: OrganizationInvitationTable,
	tags: ["autodocs"],
	parameters: { layout: "padded" },
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
	args: {
		invitations: allStatuses,
		total: allStatuses.length,
		roleNames,
		onDelete: fn().mockResolvedValue(undefined),
		statusFilter: [],
		onStatusFilterChange: fn(),
		pagination: { pageIndex: 0, pageSize: 10 },
		onPaginationChange: fn(),
		sorting: [{ id: "expires_at", desc: true }],
		onSortingChange: fn(),
	},
} satisfies Meta<typeof OrganizationInvitationTable>;

export default meta;

type Story = StoryObj<typeof meta>;

function rowFor(canvas: ReturnType<typeof within>, email: string) {
	return within(canvas.getByRole("row", { name: new RegExp(email) }));
}

/**
 * One row per status. Each row shows the status badge, the email, the role
 * name, the expiry and a delete button.
 */
export const EveryStatus: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		const pending = rowFor(canvas, "ada@example.com");
		await waitFor(async () => {
			await expect(pending.getByText("Pending")).toBeVisible();
		});
		await waitFor(async () => {
			await expect(pending.getByText("Member")).toBeVisible();
		});
		await waitFor(async () => {
			await expect(
				pending.getByText(
					dayjs(pendingInvitation.expires_at).format(EXPIRY_FORMAT),
				),
			).toBeVisible();
		});

		const accepted = rowFor(canvas, "grace@example.com");
		await waitFor(async () => {
			await expect(accepted.getByText("Accepted")).toBeVisible();
		});
		await waitFor(async () => {
			await expect(accepted.getByText("Admin")).toBeVisible();
		});

		const expired = rowFor(canvas, "alan@example.com");
		await waitFor(async () => {
			await expect(expired.getByText("Expired")).toBeVisible();
		});

		for (const email of [
			"ada@example.com",
			"grace@example.com",
			"alan@example.com",
		]) {
			await waitFor(async () => {
				await expect(
					rowFor(canvas, email).getByRole("button", {
						name: `Delete invitation for ${email}`,
					}),
				).toBeVisible();
			});
		}
	},
};

function DeleteFixture({ onDelete }: { onDelete: (id: string) => void }) {
	const [rows, setRows] = useState(allStatuses);
	return (
		<OrganizationInvitationTable
			{...meta.args}
			invitations={rows}
			total={rows.length}
			onDelete={async (deleted) => {
				onDelete(deleted.id);
				setRows((current) => current.filter((row) => row.id !== deleted.id));
			}}
		/>
	);
}

/**
 * Delete asks first. Confirming removes the row.
 */
export const DeleteRemovesRow: Story = {
	render: () => <DeleteFixture onDelete={fn()} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", {
				name: "Delete invitation for grace@example.com",
			}),
		);
		await waitFor(async () => {
			await expect(
				await screen.findByRole("heading", { name: "Delete invitation?" }),
			).toBeVisible();
		});
		await userEvent.click(screen.getByRole("button", { name: "Delete" }));

		await waitFor(async () => {
			await expect(
				canvas.queryByText("grace@example.com"),
			).not.toBeInTheDocument();
		});
		await waitFor(async () => {
			await expect(canvas.getByText("ada@example.com")).toBeVisible();
		});
	},
};

/**
 * Cancel closes the dialog, calls nothing, and leaves the row.
 */
export const DeleteCancelKeepsRow: Story = {
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", {
				name: "Delete invitation for grace@example.com",
			}),
		);
		await userEvent.click(
			await screen.findByRole("button", { name: "Cancel" }),
		);

		await waitFor(async () => {
			await expect(
				screen.queryByRole("heading", { name: "Delete invitation?" }),
			).not.toBeInTheDocument();
		});
		await expect(args.onDelete).not.toHaveBeenCalled();
		await waitFor(async () => {
			await expect(canvas.getByText("grace@example.com")).toBeVisible();
		});
	},
};

/**
 * A refused delete keeps the dialog open and shows the API text.
 */
export const DeleteFailureShowsApiError: Story = {
	args: {
		onDelete: fn(async () => {
			throw {
				errors: [
					{
						code: "ORGANIZATION_INVITATION_NOT_FOUND",
						message: "That invitation no longer exists.",
					},
				],
			};
		}),
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(
			canvas.getByRole("button", {
				name: "Delete invitation for grace@example.com",
			}),
		);
		await userEvent.click(
			await screen.findByRole("button", { name: "Delete" }),
		);

		await waitFor(async () => {
			await expect(
				await screen.findByText("That invitation no longer exists."),
			).toBeVisible();
		});
		await waitFor(async () => {
			await expect(
				screen.getByRole("heading", { name: "Delete invitation?" }),
			).toBeVisible();
		});
	},
};

/**
 * The status filter offers the three statuses and reports the choice.
 */
export const FilterByStatus: Story = {
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);

		await userEvent.click(canvas.getByRole("button", { name: "Status" }));
		await userEvent.click(
			await screen.findByRole("option", { name: "Pending" }),
		);

		await expect(args.onStatusFilterChange).toHaveBeenCalledWith([
			OrganizationInvitationStatus.PENDING,
		]);
	},
};

export const FilteredToPending: Story = {
	args: {
		invitations: [pendingInvitation],
		total: 1,
		statusFilter: [OrganizationInvitationStatus.PENDING],
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(async () => {
			await expect(
				canvas.getByRole("button", { name: "Status: Pending" }),
			).toBeVisible();
		});
		await waitFor(async () => {
			await expect(canvas.getByText("ada@example.com")).toBeVisible();
		});
		await waitFor(async () => {
			await expect(
				canvas.queryByText("grace@example.com"),
			).not.toBeInTheDocument();
		});
	},
};

export const Empty: Story = {
	args: { invitations: [], total: 0 },
	play: async ({ canvasElement }) => {
		await waitFor(async () => {
			await expect(
				within(canvasElement).getByText("No invitations yet"),
			).toBeVisible();
		});
	},
};

export const EmptyAfterFilter: Story = {
	args: {
		invitations: [],
		total: 0,
		statusFilter: [OrganizationInvitationStatus.EXPIRED],
	},
	play: async ({ canvasElement }) => {
		await waitFor(async () => {
			await expect(
				within(canvasElement).getByText("No invitations match your filters"),
			).toBeVisible();
		});
	},
};

export const Loading: Story = {
	args: { invitations: [], total: 0, loading: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(async () => {
			await expect(
				canvas.queryByText("No invitations yet"),
			).not.toBeInTheDocument();
		});
		await waitFor(async () => {
			await expect(
				canvas.queryByText("ada@example.com"),
			).not.toBeInTheDocument();
		});
	},
};

export const LoadError: Story = {
	args: {
		invitations: [],
		total: 0,
		error: {
			errors: [
				{ code: "UNEXPECTED_ERROR", message: "The database is unreachable" },
			],
		},
		onRetry: fn(),
	},
	play: async ({ args, canvasElement }) => {
		const canvas = within(canvasElement);
		await waitFor(async () => {
			await expect(canvas.getByText("Couldn’t load invitations")).toBeVisible();
		});
		await waitFor(async () => {
			await expect(
				canvas.getByText("The database is unreachable."),
			).toBeVisible();
		});
		await userEvent.click(canvas.getByRole("button", { name: "Try again" }));
		await expect(args.onRetry).toHaveBeenCalled();
	},
};
