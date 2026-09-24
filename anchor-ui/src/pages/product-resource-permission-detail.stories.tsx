import { type ProductResourcePermissionResponse, client } from "@/client";
import { ProductResourcePermissionDatatable } from "@/components/product/permissions/ProductResourcePermissionDatatable";
import { ProductProvider } from "@/context/product/ProductContext";
import { ROUTE_PATHS } from "@/routes/routePaths";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
	Outlet,
	RouterProvider,
	createMemoryHistory,
	createRootRoute,
	createRoute,
	createRouter,
} from "@tanstack/react-router";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";
import ProductResourcePermissionDetailPage from "./product-resource-permission-detail";

const PERMISSION: ProductResourcePermissionResponse = {
	product_id: "prd_test",
	name: "invoices:read",
	description: "Read invoices belonging to the organization",
	created_at: "2026-07-14T09:12:00Z",
	updated_at: "2026-07-14T09:12:00Z",
};
const LIST_PATH = ROUTE_PATHS.PRODUCT_RESOURCES_PERMISSIONS;
const DETAIL_PATH = "/products/resources/permissions/invoices%3Aread";

function PermissionRouteHarness({ initialPath }: { initialPath: string }) {
	const [queryClient] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	const [router] = useState(() => {
		const root = createRootRoute({
			component: () => (
				<ProductProvider>
					<Outlet />
				</ProductProvider>
			),
		});
		const list = createRoute({
			getParentRoute: () => root,
			path: LIST_PATH,
			component: () => (
				<ProductResourcePermissionDatatable productId="prd_test" />
			),
		});
		const detail = createRoute({
			getParentRoute: () => root,
			path: ROUTE_PATHS.PRODUCT_RESOURCE_PERMISSION_DETAIL,
			validateSearch: (search: Record<string, unknown>) => ({
				edit: search.edit === true,
			}),
			component: () => {
				const { permissionName } = detail.useParams();
				const { edit } = detail.useSearch();
				return (
					<ProductResourcePermissionDetailPage
						permissionName={permissionName}
						editing={!!edit}
					/>
				);
			},
		});
		return createRouter({
			routeTree: root.addChildren([list, detail]),
			history: createMemoryHistory({ initialEntries: [initialPath] }),
		});
	});
	return (
		<QueryClientProvider client={queryClient}>
			<RouterProvider router={router} />
		</QueryClientProvider>
	);
}

const meta = {
	title: "Product/ResourcePermissionPages",
	component: PermissionRouteHarness,
	args: { initialPath: LIST_PATH },
	parameters: { layout: "fullscreen", saveStatus: 200 },
	beforeEach: ({ parameters }) => {
		const previous = client.getConfig();
		let saved = PERMISSION;
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				const path = decodeURIComponent(new URL(request.url).pathname);
				if (path === "/v1/products/search")
					return Response.json({
						items: [
							{
								id: "prd_test",
								tenant_id: "tenant_test",
								name: "Echopoint",
								created_at: PERMISSION.created_at,
								updated_at: PERMISSION.updated_at,
							},
						],
						total: 1,
					});
				if (path.endsWith("/resource-permissions/search"))
					return Response.json({ items: [saved], total: 1 });
				if (path.endsWith("/resource-permissions/invoices:read")) {
					if (request.method === "PUT") {
						if (parameters.saveStatus !== 200)
							return Response.json(
								{ message: "Update failed" },
								{ status: parameters.saveStatus },
							);
						saved = { ...saved, ...(await request.json()) };
					}
					return Response.json(saved);
				}
				throw new Error(`Unexpected request: ${request.method} ${path}`);
			},
		});
		return () => client.setConfig(previous);
	},
} satisfies Meta<typeof PermissionRouteHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const RowAndActionsNavigate: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByText("invoices:read"));
		await expect(
			await canvas.findByRole("heading", { name: "Edit invoices:read" }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("button", { name: "Cancel" }));
		await expect(
			await canvas.findByRole("heading", { name: "invoices:read", level: 1 }),
		).toBeVisible();
		await userEvent.click(
			canvas.getByRole("link", { name: "All resource permissions" }),
		);
		await userEvent.click(
			await canvas.findByRole("link", { name: "View invoices:read" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "invoices:read", level: 1 }),
		).toBeVisible();
		await userEvent.click(
			canvas.getByRole("link", { name: "Edit permission" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "Edit invoices:read" }),
		).toBeVisible();
	},
};

export const Edit: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
};

export const SaveReturnsToView: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const description = await canvas.findByRole("textbox", {
			name: "Description",
		});
		await userEvent.clear(description);
		await userEvent.type(description, "Read customer invoices");
		await userEvent.click(canvas.getByRole("button", { name: "Save changes" }));
		await waitFor(() =>
			expect(canvas.getAllByText("Read customer invoices")).toHaveLength(2),
		);
		await expect(
			canvas.getByRole("heading", { name: "invoices:read", level: 1 }),
		).toBeVisible();
	},
};

export const FailedSaveKeepsDraft: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
	parameters: { saveStatus: 500 },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const description = await canvas.findByRole("textbox", {
			name: "Description",
		});
		await userEvent.clear(description);
		await userEvent.type(description, "Unsent draft");
		await userEvent.click(canvas.getByRole("button", { name: "Save changes" }));
		await expect(await canvas.findByRole("alert")).toBeVisible();
		await expect(description).toHaveValue("Unsent draft");
	},
};
