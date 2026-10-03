import type { OrganizationInvitationResponse } from "@/client";
import { OrganizationInvitationStatus } from "@/client";
import { ProductProvider } from "@/context/product/ProductContext";
import {
	type InvitationBackendState,
	createInvitationBackend,
} from "@/lib/storybook/invitation-backend";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import dayjs from "dayjs";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { OrganizationInvitationDatatable } from "./OrganizationInvitationDatatable";

const PRODUCT_ID = "prd_2Nq8xKf3pLmR";
const ORGANIZATION_ID = "org_2Nq8xKf3pLmR";
const MEMBER_ROLE_ID = "role_2Nq8xKf3pLmR";
const ADMIN_ROLE_ID = "role_2Nq8xKf3pLmS";

const timestamps = {
	created_at: "2026-09-01T09:00:00Z",
	updated_at: "2026-09-01T09:00:00Z",
};

function invitation(
	overrides: Partial<OrganizationInvitationResponse>,
): OrganizationInvitationResponse {
	return {
		id: "oinv_2Nq8xKf3pLm0",
		organization_id: ORGANIZATION_ID,
		email: "ada@example.com",
		role_id: MEMBER_ROLE_ID,
		status: OrganizationInvitationStatus.PENDING,
		expires_at: dayjs().add(3, "day").toISOString(),
		...timestamps,
		...overrides,
	};
}

function backendState(
	overrides: Partial<InvitationBackendState> = {},
): InvitationBackendState {
	return {
		product: {
			id: PRODUCT_ID,
			tenant_id: "ten_2Nq8xKf3pLmR",
			name: "Echopoint",
			config: {
				protected: false,
				organization_api_keys: { prefix: "echopoint" },
			},
			...timestamps,
		},
		invitations: [
			invitation({}),
			invitation({
				id: "oinv_2Nq8xKf3pLm1",
				email: "grace@example.com",
				role_id: ADMIN_ROLE_ID,
				status: OrganizationInvitationStatus.ACCEPTED,
			}),
			invitation({
				id: "oinv_2Nq8xKf3pLm2",
				email: "alan@example.com",
				status: OrganizationInvitationStatus.EXPIRED,
				expires_at: dayjs().subtract(2, "day").toISOString(),
			}),
		],
		roles: [
			{
				id: MEMBER_ROLE_ID,
				product_id: PRODUCT_ID,
				name: "Member",
				permissions: [],
				...timestamps,
			},
			{
				id: ADMIN_ROLE_ID,
				product_id: PRODUCT_ID,
				name: "Admin",
				permissions: [],
				...timestamps,
			},
		],
		...overrides,
	};
}

const meta = {
	title: "Organization/OrganizationInvitationDatatable",
	component: OrganizationInvitationDatatable,
	tags: ["autodocs"],
	parameters: { layout: "padded" },
	args: { organizationId: ORGANIZATION_ID },
	decorators: [
		(Story) => (
			<StoryQuery>
				<ProductProvider>
					<Story />
				</ProductProvider>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof OrganizationInvitationDatatable>;

export default meta;

type Story = StoryObj<typeof meta>;

function rowFor(canvas: ReturnType<typeof within>, email: string) {
	return within(canvas.getByRole("row", { name: new RegExp(email) }));
}

/**
 * The list shows each invitation with its role name.
 */
export const ListsInvitationsWithRoleNames: Story = {
	beforeEach: () => {
		const backend = createInvitationBackend(backendState());
		return backend.install();
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("ada@example.com")).toBeVisible();
		await expect(
			rowFor(canvas, "ada@example.com").getByText("Member"),
		).toBeVisible();
		await expect(
			rowFor(canvas, "grace@example.com").getByText("Admin"),
		).toBeVisible();
	},
};

/**
 * Confirming the delete removes the row from the list.
 */
export const DeleteRemovesRow: Story = {
	beforeEach: () => {
		const backend = createInvitationBackend(backendState());
		return backend.install();
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			await canvas.findByRole("button", {
				name: "Delete invitation for grace@example.com",
			}),
		);
		await userEvent.click(
			await screen.findByRole("button", { name: "Delete" }),
		);

		await waitFor(() =>
			expect(canvas.queryByText("grace@example.com")).not.toBeInTheDocument(),
		);
		await expect(canvas.getByText("ada@example.com")).toBeVisible();
	},
};

/**
 * Cancel leaves the row and sends no delete.
 */
export const DeleteCancelKeepsRow: Story = {
	beforeEach: () => {
		const backend = createInvitationBackend(backendState());
		return backend.install();
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(
			await canvas.findByRole("button", {
				name: "Delete invitation for grace@example.com",
			}),
		);
		await userEvent.click(
			await screen.findByRole("button", { name: "Cancel" }),
		);

		await expect(canvas.getByText("grace@example.com")).toBeVisible();
	},
};

/**
 * Choosing Pending sends the status filter and keeps only pending rows.
 */
export const FilterKeepsOnlyChosenStatus: Story = {
	beforeEach: () => {
		const backend = createInvitationBackend(backendState());
		return backend.install();
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("grace@example.com")).toBeVisible();

		await userEvent.click(canvas.getByRole("button", { name: "Status" }));
		await userEvent.click(
			await screen.findByRole("option", { name: "Pending" }),
		);

		await waitFor(() =>
			expect(canvas.queryByText("grace@example.com")).not.toBeInTheDocument(),
		);
		await expect(
			canvas.queryByText("alan@example.com"),
		).not.toBeInTheDocument();
		await expect(canvas.getByText("ada@example.com")).toBeVisible();
	},
};

export const EmptyList: Story = {
	beforeEach: () => {
		const backend = createInvitationBackend(backendState({ invitations: [] }));
		return backend.install();
	},
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByText("No invitations yet"),
		).toBeVisible();
	},
};

export const LoadFailure: Story = {
	beforeEach: () => {
		const backend = createInvitationBackend(
			backendState({ failInvitationSearch: true }),
		);
		return backend.install();
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByText("Couldn’t load invitations"),
		).toBeVisible();
		await expect(
			canvas.getByText("The database is unreachable."),
		).toBeVisible();
		await expect(
			canvas.getByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};
