import type { ProductResponse } from "@/client";
import { getProductEventsCatalogQueryKey } from "@/client/@tanstack/react-query.gen";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useQueryClient } from "@tanstack/react-query";
import type * as React from "react";
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

const CATALOG_DATA = {
	items: [
		{
			type: "organization.created",
			name: "Organization created",
			description: "Emitted when a new organization is created.",
			group_type: "theme" as const,
			group_name: "Organizations",
			theme: "Organizations",
		},
		{
			type: "organization.updated",
			name: "Organization updated",
			description: "Emitted when an organization's details are updated.",
			group_type: "theme" as const,
			group_name: "Organizations",
			theme: "Organizations",
		},
		{
			type: "workspace.created",
			name: "Workspace created",
			description: "Emitted when a workspace is created.",
			group_type: "theme" as const,
			group_name: "Workspaces",
			theme: "Workspaces",
		},
		{
			type: "clerk.user.created",
			name: "Clerk user created",
			description:
				"Emitted when a product user is created from a Clerk webhook.",
			group_type: "integration" as const,
			group_name: "CLERK",
			integration: "CLERK",
		},
	],
};

function StoryCatalogSeeder({ children }: { children: React.ReactNode }) {
	const queryClient = useQueryClient();
	const key = getProductEventsCatalogQueryKey({
		path: { product_id: PRODUCT.id },
	});
	queryClient.setQueryData(key, CATALOG_DATA);
	queryClient.setQueryDefaults(key, {
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: Number.POSITIVE_INFINITY,
	});
	return <>{children}</>;
}

const meta = {
	title: "Product/ProductEventsForm",
	component: ProductEventsForm,
	tags: ["autodocs"],
	args: {
		product: PRODUCT,
		productId: PRODUCT.id,
	},
	parameters: {
		layout: "fullscreen",
	},
	decorators: [
		(Story) => (
			<StoryQuery>
				<StoryCatalogSeeder>
					<div className="w-full p-6 lg:p-8">
						<Story />
					</div>
				</StoryCatalogSeeder>
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
					events: ["organization.created", "workspace.created"],
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

export const WithSearchFilter: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const searchInput = canvas.getByPlaceholderText(/Filter events by name/);
		await userEvent.type(searchInput, "organization");
		await expect(await canvas.findByText("Organizations")).toBeInTheDocument();
		await expect(canvas.queryByText("Workspaces")).not.toBeInTheDocument();
	},
};
