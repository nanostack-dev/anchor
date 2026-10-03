import type { ProductResponse } from "@/client";
import { StoryQuery } from "@/lib/storybook/story-query";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";
import { ProductDeleteDialog } from "./ProductDeleteDialog";

const PRODUCT: ProductResponse = {
	id: "prd_2Nq8xKf3pLmR",
	tenant_id: "ten_2Nq8xKf3pLmR",
	name: "Echopoint",
	config: { protected: false, organization_api_keys: { prefix: "echopoint" } },
	created_at: "2026-08-01T09:00:00Z",
	updated_at: "2026-08-01T09:00:00Z",
};
const meta = {
	title: "Product/ProductDeleteDialog",
	component: ProductDeleteDialog,
	args: { product: PRODUCT },
	decorators: [
		(Story) => (
			<StoryQuery>
				<Story />
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductDeleteDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Protected: Story = {
	args: {
		product: { ...PRODUCT, config: { ...PRODUCT.config, protected: true } },
	},
	play: async ({ canvasElement }) => {
		await expect(
			within(canvasElement).getByRole("button", {
				name: "Product protected from deletion",
			}),
		).toBeDisabled();
	},
};

export const Unprotected: Story = {
	play: async ({ canvasElement }) => {
		await userEvent.click(
			within(canvasElement).getByRole("button", { name: "Delete product" }),
		);
		await expect(within(document.body).getByRole("dialog")).toBeVisible();
	},
};
