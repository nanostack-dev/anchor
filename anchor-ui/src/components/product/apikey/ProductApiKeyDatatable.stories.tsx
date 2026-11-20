import { type ProductApiKeyResponse, ProductApiKeyStatus } from "@/client";
import { client } from "@/client/client.gen";
import { StoryQuery } from "@/lib/storybook/story-query";
import { StoryRouter } from "@/lib/storybook/story-router";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, screen, userEvent, waitFor, within } from "storybook/test";

import { ProductApiKeyDatatable } from "./ProductApiKeyDatatable";

const PRODUCT_ID = "prd_2Nq8xKf3pLmR";

function apiKey(id: string, name: string): ProductApiKeyResponse {
	return {
		id,
		product_id: PRODUCT_ID,
		name,
		mutable: false,
		obfuscated_value: "anc_****",
		permissions: [],
		status: ProductApiKeyStatus.ACTIVE,
		created_at: "2026-09-01T09:00:00Z",
		updated_at: "2026-09-01T09:00:00Z",
	};
}

function installApiKeyBackend(keys: ProductApiKeyResponse[]) {
	let stored = [...keys];
	const previousConfig = client.getConfig();
	client.setConfig({
		baseUrl: window.location.origin,
		fetch: async (input) => {
			if (!(input instanceof Request)) throw new Error("Expected a Request");
			const path = new URL(input.url).pathname;
			if (path.endsWith("/api-keys/search"))
				return Response.json({
					items: stored,
					total: stored.length,
					count: stored.length,
				});
			const single = path.match(/\/api-keys\/([^/]+)$/);
			if (single && input.method === "DELETE") {
				stored = stored.filter((key) => key.id !== single[1]);
				return new Response(null, { status: 204 });
			}
			return Response.json(
				{ errors: [{ code: "NOT_FOUND", message: `No fake for ${path}` }] },
				{ status: 404 },
			);
		},
	});
	return () => {
		client.setConfig({
			...previousConfig,
			fetch: previousConfig.fetch ?? globalThis.fetch.bind(globalThis),
		});
	};
}

const meta = {
	title: "Product/ProductApiKeyDatatable",
	component: ProductApiKeyDatatable,
	parameters: { layout: "padded" },
	args: { productId: PRODUCT_ID },
	decorators: [
		(Story) => (
			<StoryQuery>
				<StoryRouter>
					<Story />
				</StoryRouter>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductApiKeyDatatable>;

export default meta;

type Story = StoryObj<typeof meta>;

export const DeletingAKeyRemovesItsRow: Story = {
	beforeEach: () =>
		installApiKeyBackend([
			apiKey("pak_2Nq8xKf3pLm0", "billing-sync"),
			apiKey("pak_2Nq8xKf3pLm1", "audit-export"),
		]),
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const row = within(
			await canvas.findByRole("row", { name: /billing-sync/ }),
		);
		await userEvent.click(row.getByRole("button", { name: "Delete API Key" }));
		await userEvent.click(
			within(await screen.findByRole("dialog")).getByRole("button", {
				name: "Delete API Key",
			}),
		);
		await waitFor(() =>
			expect(
				canvas.queryByRole("row", { name: /billing-sync/ }),
			).not.toBeInTheDocument(),
		);
		await expect(
			canvas.getByRole("row", { name: /audit-export/ }),
		).toBeVisible();
	},
};
