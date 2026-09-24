import type {
	ProductEventDeliveryStatusResponse,
	ProductResponse,
} from "@/client";
import {
	getProductEventDeliveryStatusQueryKey,
	getProductEventsCatalogQueryKey,
} from "@/client/@tanstack/react-query.gen";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { useQueryClient } from "@tanstack/react-query";
import * as React from "react";
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
			group_type: "internal" as const,
			group_name: "Organizations",
		},
		{
			type: "organization.updated",
			name: "Organization updated",
			description: "Emitted when an organization's details are updated.",
			group_type: "internal" as const,
			group_name: "Organizations",
		},
		{
			type: "workspace.created",
			name: "Workspace created",
			description: "Emitted when a workspace is created.",
			group_type: "internal" as const,
			group_name: "Workspaces",
		},
		{
			type: "clerk.user.created",
			name: "Clerk user created",
			description:
				"Emitted when a product user is created from a Clerk webhook.",
			group_type: "integration" as const,
			group_name: "CLERK",
		},
	],
};

function StoryCatalogSeeder({
	children,
	deliveryStatus = { failed_count: 0, retrying_count: 0 },
}: {
	children: React.ReactNode;
	deliveryStatus?: ProductEventDeliveryStatusResponse;
}) {
	const queryClient = useQueryClient();
	const key = getProductEventsCatalogQueryKey({
		path: { product_id: PRODUCT.id },
	});
	queryClient.setQueryData(key, CATALOG_DATA);
	queryClient.setQueryDefaults(key, {
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: Number.POSITIVE_INFINITY,
	});
	const statusKey = getProductEventDeliveryStatusQueryKey({
		path: { product_id: PRODUCT.id },
	});
	queryClient.setQueryData(statusKey, deliveryStatus);
	queryClient.setQueryDefaults(statusKey, {
		staleTime: Number.POSITIVE_INFINITY,
		gcTime: Number.POSITIVE_INFINITY,
	});
	return <>{children}</>;
}

function DeferredCatalogSeeder({ children }: { children: React.ReactNode }) {
	const queryClient = useQueryClient();
	const queryKey = getProductEventsCatalogQueryKey({
		path: { product_id: PRODUCT.id },
	});
	queryClient.setQueryDefaults(queryKey, { enabled: false });
	return (
		<>
			<button
				type="button"
				onClick={() =>
					queryClient
						.getQueryCache()
						.find({ queryKey })
						?.setState({
							error: new Error("Catalog request failed"),
							status: "error",
							fetchStatus: "idle",
						})
				}
			>
				Fail catalog
			</button>
			<button
				type="button"
				onClick={() => queryClient.setQueryData(queryKey, CATALOG_DATA)}
			>
				Load catalog
			</button>
			{children}
		</>
	);
}

function ProductRefreshFixture({ product }: { product: ProductResponse }) {
	const [currentProduct, setCurrentProduct] = React.useState(product);
	return (
		<>
			<button
				type="button"
				onClick={() =>
					setCurrentProduct({
						...currentProduct,
						updated_at: "2026-08-02T09:00:00Z",
					})
				}
			>
				Refresh product
			</button>
			<ProductEventsForm product={currentProduct} />
		</>
	);
}

const meta = {
	title: "Product/ProductEventsForm",
	component: ProductEventsForm,
	tags: ["autodocs"],
	args: {
		product: PRODUCT,
	},
	parameters: {
		layout: "fullscreen",
	},
	decorators: [
		(Story, context) => (
			<StoryQuery>
				{context.parameters.catalogDeferred ? (
					<DeferredCatalogSeeder>
						<div className="w-full p-6 lg:p-8">
							<Story />
						</div>
					</DeferredCatalogSeeder>
				) : (
					<StoryCatalogSeeder
						deliveryStatus={context.parameters.deliveryStatus}
					>
						<div className="w-full p-6 lg:p-8">
							<Story />
						</div>
					</StoryCatalogSeeder>
				)}
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

export const FailedDelivery: Story = {
	args: Configured.args,
	parameters: {
		deliveryStatus: {
			failed_count: 1,
			retrying_count: 2,
			last_failure: {
				event_type: "organization.created",
				attempts: 6,
				error: "events: delivery status 503",
				failed_at: "2026-08-01T09:05:00Z",
			},
		},
	},
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(canvas.getByText(/1 failed · 2 retrying/)).toBeInTheDocument();
		await expect(canvas.getByText(/after 6 attempts/)).toBeInTheDocument();
		await expect(canvas.getByText(/delivery status 503/)).toBeInTheDocument();
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

export const CatalogUnavailable: Story = {
	parameters: { catalogDeferred: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Fail catalog" }));
		await expect(
			await canvas.findByText("Event catalog unavailable"),
		).toBeVisible();
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Event endpoint URL" }),
			"https://example.com/anchor/events",
		);
		await expect(
			canvas.getByRole("button", { name: "Save endpoint" }),
		).toBeDisabled();
	},
};

export const PreservesUnsavedEdits: Story = {
	render: (args) => <ProductRefreshFixture product={args.product} />,
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const input = canvas.getByRole("textbox", { name: "Event endpoint URL" });
		await userEvent.type(input, "https://example.com/anchor/events");
		await userEvent.click(canvas.getByRole("button", { name: "Deselect all" }));
		await userEvent.click(
			canvas.getByRole("button", { name: "Refresh product" }),
		);
		await expect(input).toHaveValue("https://example.com/anchor/events");
		await expect(canvas.getByText("0 selected")).toBeInTheDocument();
	},
};

export const CatalogLoadsAfterUrlEdit: Story = {
	parameters: { catalogDeferred: true },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Fail catalog" }));
		await expect(
			await canvas.findByText("Event catalog unavailable"),
		).toBeVisible();
		await userEvent.type(
			canvas.getByRole("textbox", { name: "Event endpoint URL" }),
			"https://example.com/anchor/events",
		);
		await userEvent.click(canvas.getByRole("button", { name: "Load catalog" }));
		await expect(await canvas.findByText("4 selected")).toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: "Save endpoint" }),
		).toBeEnabled();
	},
};
