import {
	LicenseFieldType,
	type LicenseSchemaResponse,
	type LicenseTemplateResponse,
	LicenseTemplateStatus,
	UsageShape,
	client,
} from "@/client";
import { LicenseTemplateDatatable } from "@/components/license/LicenseTemplateDatatable";
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
import { expect, userEvent, within } from "storybook/test";
import LicenseTemplatePage from "./license-template";

const TIMESTAMPS = {
	created_at: "2026-08-16T17:27:00Z",
	updated_at: "2026-08-16T17:27:00Z",
};
const TEMPLATE: LicenseTemplateResponse = {
	...TIMESTAMPS,
	id: "ltm_test",
	product_id: "prd_test",
	name: "Enterprise",
	status: LicenseTemplateStatus.ACTIVE,
	values: { max_flows: 10000 },
};
const SCHEMA: LicenseSchemaResponse = {
	...TIMESTAMPS,
	id: "lsc_test",
	product_id: "prd_test",
	fields: [
		{
			...TIMESTAMPS,
			id: "lfd_flows",
			name: "max_flows",
			type: LicenseFieldType.LIMIT,
			usage_shape: UsageShape.GAUGE,
			rules: {},
		},
	],
};
const DETAIL_PATH = "/products/licensing/templates/ltm_test";

function TemplateRouteHarness({ initialPath }: { initialPath: string }) {
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
		const detail = createRoute({
			getParentRoute: () => root,
			path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_DETAIL,
			validateSearch: (search: Record<string, unknown>) => ({
				edit: search.edit === true,
			}),
			component: () => {
				const { templateId } = detail.useParams();
				const { edit } = detail.useSearch();
				return <LicenseTemplatePage templateId={templateId} editing={edit} />;
			},
		});
		const list = createRoute({
			getParentRoute: () => root,
			path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATES,
			component: () => <LicenseTemplateDatatable productId="prd_test" />,
		});
		const create = createRoute({
			getParentRoute: () => root,
			path: ROUTE_PATHS.PRODUCT_LICENSE_TEMPLATE_NEW,
			component: () => <LicenseTemplatePage />,
		});
		return createRouter({
			routeTree: root.addChildren([detail, list, create]),
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
	title: "License/LicenseTemplateRoutes",
	component: TemplateRouteHarness,
	args: { initialPath: DETAIL_PATH },
	parameters: { layout: "fullscreen", templateStatus: 200 },
	beforeEach: ({ parameters }) => {
		const previous = client.getConfig();
		let saved = TEMPLATE;
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				const path = new URL(request.url).pathname;
				if (path === "/v1/products/search")
					return Response.json({
						items: [
							{
								...TIMESTAMPS,
								id: "prd_test",
								tenant_id: "tenant_test",
								name: "Echopoint",
							},
						],
						total: 1,
					});
				if (path.endsWith("/licensing/schema")) return Response.json(SCHEMA);
				if (path.endsWith("/licensing/templates"))
					return Response.json({ items: [saved], count: 1 });
				if (path.endsWith("/licensing/templates/ltm_test")) {
					if (parameters.templateStatus !== 200)
						return new Response(null, { status: parameters.templateStatus });
					if (request.method === "PUT")
						saved = { ...saved, ...(await request.json()) };
					return Response.json(saved);
				}
				throw new Error(`Unexpected request: ${request.method} ${path}`);
			},
		});
		return () => client.setConfig(previous);
	},
} satisfies Meta<typeof TemplateRouteHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const DirectLinkLoadsTemplate: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("heading", { name: "Enterprise", level: 1 }),
		).toBeVisible();
		await expect(canvas.getByText("10000")).toBeVisible();
	},
};
export const DirectEditSaveAndBack: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const name = await canvas.findByRole("textbox", { name: "Name" });
		await userEvent.clear(name);
		await userEvent.type(name, "Enterprise Plus");
		await userEvent.click(
			canvas.getByRole("button", { name: "Save Template" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "Enterprise Plus", level: 1 }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("link", { name: "All templates" }));
		await expect(
			await canvas.findByRole("link", { name: "Enterprise Plus" }),
		).toHaveAttribute("href", DETAIL_PATH);
		await userEvent.click(
			canvas.getByRole("link", { name: "Edit Enterprise Plus" }),
		);
		await expect(
			await canvas.findByRole("textbox", { name: "Name" }),
		).toHaveValue("Enterprise Plus");
	},
};
export const MissingTemplate: Story = {
	parameters: { templateStatus: 404 },
	play: async ({ canvasElement }) => {
		await expect(
			await within(canvasElement).findByText("Template not found"),
		).toBeVisible();
	},
};
export const BackendFailureIsNotNotFound: Story = {
	parameters: { templateStatus: 500 },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByText("Couldn’t load this template"),
		).toBeVisible();
		await expect(
			canvas.queryByText("Template not found"),
		).not.toBeInTheDocument();
		await expect(
			canvas.getByRole("button", { name: "Try again" }),
		).toBeVisible();
	},
};
