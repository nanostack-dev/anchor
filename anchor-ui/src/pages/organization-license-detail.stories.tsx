import "@/context/auth/AuthContext";
import { LicenseTemplateStatus, client } from "@/client";
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
import { expect, within } from "storybook/test";
import OrganizationLicenseDetailPage from "./organization-license-detail";

const TIMESTAMP = "2026-08-16T17:27:00Z";
const LICENSE = {
	id: "lic_test",
	template_id: "ltm_test",
	values: { max_flows: 20000 },
	adjusted_fields: ["max_flows"],
	instantiated_at: TIMESTAMP,
};

function LicenseDetailHarness() {
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
			path: ROUTE_PATHS.ORGANIZATION_LICENSE_DETAIL,
			component: OrganizationLicenseDetailPage,
		});
		return createRouter({
			routeTree: root.addChildren([detail]),
			history: createMemoryHistory({
				initialEntries: ["/organizations/license/org_test"],
			}),
		});
	});
	return (
		<QueryClientProvider client={queryClient}>
			<RouterProvider router={router} />
		</QueryClientProvider>
	);
}

const meta = {
	title: "License/OrganizationLicenseDetail",
	component: LicenseDetailHarness,
	parameters: {
		layout: "fullscreen",
		templateStatus: LicenseTemplateStatus.ACTIVE,
	},
	beforeEach: ({ parameters }) => {
		const previous = client.getConfig();
		client.setConfig({
			baseUrl: window.location.origin,
			fetch: async (request) => {
				if (!(request instanceof Request)) throw new Error("Expected Request");
				const path = new URL(request.url).pathname;
				if (path === "/v1/products/search")
					return Response.json({
						items: [{ id: "prd_test", name: "Echopoint" }],
						total: 1,
					});
				if (path.endsWith("/licensing/templates"))
					return Response.json({
						items: [
							{
								id: "ltm_test",
								name: "Enterprise",
								status: parameters.templateStatus,
								values: { max_flows: 10000 },
							},
						],
					});
				if (path.endsWith("/licensing/organization-licenses/search"))
					return Response.json({
						items: [
							{
								organization_id: "org_test",
								organization_name: "Acme",
								license: LICENSE,
							},
						],
					});
				if (path.endsWith("/organizations/org_test/license"))
					return Response.json(LICENSE);
				throw new Error(`Unexpected request: ${request.method} ${path}`);
			},
		});
		return () => client.setConfig(previous);
	},
} satisfies Meta<typeof LicenseDetailHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const CustomizedLicenseWithoutBadge: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("Enterprise")).toBeVisible();
		await expect(canvas.getByRole("heading", { name: "Acme" })).toBeVisible();
		await expect(canvas.queryByText("Adjusted")).not.toBeInTheDocument();
	},
};

export const WithdrawnTierStillVisible: Story = {
	parameters: { templateStatus: LicenseTemplateStatus.ARCHIVED },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(await canvas.findByText("Tier withdrawn")).toBeVisible();
		await expect(canvas.queryByText("Adjusted")).not.toBeInTheDocument();
	},
};
