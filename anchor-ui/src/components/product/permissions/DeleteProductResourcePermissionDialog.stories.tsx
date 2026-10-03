import type { ProductRoleResponse, ProductRoleSearchRequest } from "@/client";
import { client } from "@/client/client.gen";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { DeleteProductResourcePermissionDialog } from "./DeleteProductResourcePermissionDialog";

const PRODUCT_ID = "prd_2Nq8xKf3pLmR";

function role(name: string): ProductRoleResponse {
	return {
		id: `role_${name}`,
		product_id: PRODUCT_ID,
		name,
		permissions: [],
		created_at: "2026-07-14T09:12:00Z",
		updated_at: "2026-07-14T09:12:00Z",
	};
}

type RoleSearchReply =
	| { items: ProductRoleResponse[]; total?: number }
	| "fail";

const roleSearches: ProductRoleSearchRequest[] = [];

function mockRoleSearch(reply: RoleSearchReply) {
	roleSearches.length = 0;
	const previousConfig = client.getConfig();
	client.setConfig({
		baseUrl: window.location.origin,
		fetch: async (input) => {
			if (!(input instanceof Request)) throw new Error("Expected a Request");
			roleSearches.push(await input.json());
			if (reply === "fail") {
				return Response.json(
					{ errors: [{ code: "UNEXPECTED_ERROR", message: "Unavailable" }] },
					{ status: 500 },
				);
			}
			return Response.json({
				items: reply.items,
				total: reply.total ?? reply.items.length,
				count: reply.items.length,
			});
		},
	});
	const restore = () =>
		client.setConfig({
			...previousConfig,
			fetch: previousConfig.fetch ?? globalThis.fetch.bind(globalThis),
		});
	return restore;
}

const meta = {
	title: "Product/DeleteProductResourcePermissionDialog",
	component: DeleteProductResourcePermissionDialog,
	tags: ["autodocs"],
	args: {
		productId: PRODUCT_ID,
		permission: {
			product_id: PRODUCT_ID,
			name: "invoices:read",
			description: "Read invoices belonging to the organization",
			created_at: "2026-07-14T09:12:00Z",
			updated_at: "2026-07-14T09:12:00Z",
		},
	},
	beforeEach: () => mockRoleSearch({ items: [] }),
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
		await expect(confirm).toHaveClass(
			"bg-destructive",
			"text-destructive-foreground",
		);
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

		await waitFor(async () => {
			await expect(
				screen.queryByRole("heading", { name: "Delete Product Permission" }),
			).not.toBeInTheDocument();
		});
	},
};

async function openDialog(canvasElement: HTMLElement) {
	await userEvent.click(
		within(canvasElement).getByRole("button", { name: "Delete permission" }),
	);
	await screen.findByRole("heading", { name: "Delete Product Permission" });
}

/**
 * The roles that hold the permission are named before the user confirms, and
 * the lookup is scoped to this permission.
 */
export const NamesTheRolesThatLoseIt: Story = {
	beforeEach: () =>
		mockRoleSearch({ items: [role("Editor"), role("Reviewer")] }),
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement);

		await waitFor(async () => {
			await expect(
				await screen.findByText("These roles lose it: Editor, Reviewer"),
			).toBeVisible();
		});
		await waitFor(async () => {
			await expect(
				screen.getByText(
					/API keys currently using it will lose this permission/,
				),
			).toBeVisible();
		});
		await waitFor(() =>
			expect(roleSearches.at(-1)?.filter?.permissions).toEqual([
				"invoices:read",
			]),
		);
	},
};

/**
 * When the page holds fewer roles than match, the rest are counted, not hidden.
 */
export const CountsRolesBeyondTheList: Story = {
	beforeEach: () =>
		mockRoleSearch({
			items: [role("Editor"), role("Reviewer")],
			total: 5,
		}),
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement);

		await waitFor(async () => {
			await expect(
				await screen.findByText(
					"These roles lose it: Editor, Reviewer, and 3 more",
				),
			).toBeVisible();
		});
	},
};

/**
 * A permission no role holds says so, instead of showing an empty callout.
 */
export const NoRoleHoldsIt: Story = {
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement);

		await waitFor(async () => {
			await expect(
				await screen.findByText("No role holds this permission."),
			).toBeVisible();
		});
		await waitFor(async () => {
			await expect(
				screen.queryByText(/These roles lose it/),
			).not.toBeInTheDocument();
		});
	},
};

/**
 * When the lookup fails the dialog falls back to the generic warning, so the
 * delete is never presented as free.
 */
export const FallsBackWhenLookupFails: Story = {
	beforeEach: () => mockRoleSearch("fail"),
	play: async ({ canvasElement }) => {
		await openDialog(canvasElement);

		await waitFor(async () => {
			await expect(
				await screen.findByText(
					/Any roles or API keys currently using it will lose this permission immediately/,
				),
			).toBeVisible();
		});
	},
};
