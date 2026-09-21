import type { ProductResponse } from "@/client";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { StoryQuery } from "@/lib/storybook/story-query";

import { ProductEventsForm } from "./ProductEventsForm";

const PRODUCT: ProductResponse = {
	id: "prd_2Nq8xKf3pLmR",
	tenant_id: "ten_2Nq8xKf3pLmR",
	name: "Echopoint",
	description: "Webhook testing",
	config: {
		organization_api_keys: { prefix: "echopoint" },
	},
	created_at: "2026-08-01T09:00:00Z",
	updated_at: "2026-08-01T09:00:00Z",
};

const meta = {
	title: "Product/ProductEventsForm",
	component: ProductEventsForm,
	tags: ["autodocs"],
	args: {
		product: PRODUCT,
		productId: PRODUCT.id,
	},
	parameters: {
		layout: "padded",
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<div className="w-full max-w-xl">
					<Story />
				</div>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductEventsForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("textbox", { name: "Event endpoint URL" }),
		).toHaveValue("");
		await expect(
			canvas.getByRole("button", { name: "Save endpoint" }),
		).toBeDisabled();
	},
};

export const Configured: Story = {
	args: {
		product: {
			...PRODUCT,
			config: {
				organization_api_keys: { prefix: "echopoint" },
				events: {
					endpoint_url: "https://example.com/anchor/events",
					signing_secret_obfuscated: "whsec_••••",
				},
			},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			canvas.getByRole("textbox", { name: "Event endpoint URL" }),
		).toHaveValue("https://example.com/anchor/events");
		await expect(
			canvas.getByText(/Stored secret: whsec_••••/),
		).toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: "Save endpoint" }),
		).toBeDisabled();
	},
};

export const Dirty: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const input = canvas.getByRole("textbox", { name: "Event endpoint URL" });
		await userEvent.type(input, "https://example.com/anchor/events");
		await expect(
			canvas.getByRole("button", { name: "Save endpoint" }),
		).toBeEnabled();
	},
};
