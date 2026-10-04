import type { ProductResponse } from "@/client";
import { client } from "@/client/client.gen";
import { StoryQuery } from "@/lib/storybook/story-query";
import {
	Card,
	CardContent,
	CardDescription,
	CardHeader,
	CardTitle,
} from "@nanostackorg/design-system/components/card";
import { Box } from "@nanostackorg/design-system/layout/box";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, waitFor, within } from "storybook/test";
import { ProductEditForm } from "./ProductEditForm";

const PRODUCT: ProductResponse = {
	id: "prd_2Nq8xKf3pLmR",
	tenant_id: "ten_2Nq8xKf3pLmR",
	name: "Echopoint",
	description: "Webhook testing",
	config: { protected: false, organization_api_keys: { prefix: "echopoint" } },
	created_at: "2026-08-01T09:00:00Z",
	updated_at: "2026-08-01T09:00:00Z",
};

const meta = {
	title: "Product/ProductEditForm",
	component: ProductEditForm,
	args: { product: PRODUCT, productId: PRODUCT.id },
	parameters: { layout: "fullscreen" },
	decorators: [
		(Story) => (
			<StoryQuery>
				<Box className="p-8">
					<Card>
						<CardHeader>
							<CardTitle>Product</CardTitle>
							<CardDescription>
								Update product information and configuration.
							</CardDescription>
						</CardHeader>
						<CardContent>
							<Story />
						</CardContent>
					</Card>
				</Box>
			</StoryQuery>
		),
	],
} satisfies Meta<typeof ProductEditForm>;

export default meta;
type Story = StoryObj<typeof meta>;
export const Configuration: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("tab", { name: "Config" }));
		await expect(
			canvas.getByRole("switch", { name: "Protected product" }),
		).not.toBeChecked();
		await expect(
			canvas.getByRole("button", { name: "Update Product" }),
		).toBeDisabled();
	},
};

export const Protected: Story = {
	args: {
		product: { ...PRODUCT, config: { ...PRODUCT.config, protected: true } },
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("tab", { name: "Config" }));
		await expect(
			canvas.getByRole("switch", { name: "Protected product" }),
		).toBeChecked();
	},
};

async function toggleAndSave(canvasElement: HTMLElement, value: boolean) {
	const canvas = within(canvasElement);
	await userEvent.click(canvas.getByRole("tab", { name: "Config" }));
	const toggle = canvas.getByRole("switch", { name: "Protected product" });
	await expect(toggle).toHaveAttribute("aria-checked", String(!value));
	const previous = client.getConfig();
	let requestBody: unknown;
	client.setConfig({
		baseUrl: "https://anchor.test",
		fetch: async (input, init) => {
			const request =
				input instanceof Request ? input : new Request(input, init);
			requestBody = await request.json();
			return Response.json({
				...PRODUCT,
				config: { ...PRODUCT.config, protected: value },
			});
		},
	});
	try {
		await userEvent.click(toggle);
		await userEvent.click(
			canvas.getByRole("button", { name: "Update Product" }),
		);
		await waitFor(() =>
			expect(requestBody).toMatchObject({
				name: PRODUCT.name,
				config: {
					protected: value,
					organization_api_keys: { prefix: "echopoint" },
				},
			}),
		);
		await expect(
			canvas.getByRole("button", { name: "Update Product" }),
		).toBeEnabled();
	} finally {
		client.setConfig(previous);
	}
}

export const EnableProtection: Story = {
	play: async ({ canvasElement }) => toggleAndSave(canvasElement, true),
};

export const DisableProtection: Story = {
	args: Protected.args,
	play: async ({ canvasElement }) => toggleAndSave(canvasElement, false),
};
