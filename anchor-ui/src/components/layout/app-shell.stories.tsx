import { type ProductResponse, SortDirection, client } from "@/client";
import {
	getCurrentUserQueryKey,
	searchProductsQueryKey,
} from "@/client/@tanstack/react-query.gen";
import { useProduct } from "@/context/product/ProductContext";
import { ROUTE_PATHS } from "@/routes/routePaths";
import { Heading } from "@nanostackorg/design-system/components/heading";
import { Text } from "@nanostackorg/design-system/components/text";
import { Box } from "@nanostackorg/design-system/layout/box";
import { Stack } from "@nanostackorg/design-system/layout/stack";
import { DesignSystemProvider } from "@nanostackorg/design-system/provider";
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
import { expect, screen, userEvent, waitFor, within } from "storybook/test";
import { AuthenticatedLayout } from "./AuthenticatedLayout";
import { RouterLink } from "./router-link";

const products: ProductResponse[] = ["Echopoint", "Anchor"].map((name) => ({
	id: `prd_${name.toLowerCase()}`,
	tenant_id: "tenant_story",
	name,
	config: { protected: false, organization_api_keys: { prefix: "org_" } },
	created_at: "2026-10-03T12:00:00Z",
	updated_at: "2026-10-03T12:00:00Z",
}));

const worstCaseProducts: ProductResponse[] = [
	{
		...products[0],
		id: "prd_northwind",
		name: "NorthwindEnterpriseCustomerIdentityAndAccessManagementProductionPlatformForEuropeanFinancialServices",
		description:
			"Production identity and access management for European financial services teams, customer onboarding, regional compliance operations and partner integrations across multiple organizations.",
	},
	{ ...products[1], id: "prd_ai", name: "AI", description: undefined },
	{
		...products[0],
		id: "prd_tokyo",
		name: "日本市場の顧客認証基盤 🛡️",
		description: "Customer authentication for the Japanese market.",
	},
	{
		...products[1],
		id: "prd_zurich",
		name: "Zürich Enterprise Operations – Customer Identity & Partner Access",
		description: "",
	},
];

function ShellDestination({ section }: { section: string }) {
	const { currentProduct } = useProduct();
	return (
		<Box className="min-w-0 wrap-anywhere">
			<Stack space="md">
				<Heading level={1}>
					{section} for {currentProduct?.name ?? "no selected product"}
				</Heading>
				<Text>Content follows the current route and selected product.</Text>
			</Stack>
		</Box>
	);
}

function ShellHarness({
	availableProducts,
}: { availableProducts: ProductResponse[] }) {
	const [queryClient] = useState(() => {
		const queryClient = new QueryClient({
			defaultOptions: {
				queries: {
					enabled: false,
					retry: false,
					staleTime: Number.POSITIVE_INFINITY,
				},
			},
		});
		queryClient.setQueryData(getCurrentUserQueryKey(), {
			id: "user_story",
			email: "admin@example.com",
			role: "ADMIN",
		});
		queryClient.setQueryData(
			searchProductsQueryKey({
				body: {
					pagination: { limit: 100, offset: 0 },
					sort_by: "name",
					sort_direction: SortDirection.ASC,
				},
			}),
			{ items: availableProducts, total: availableProducts.length },
		);
		return queryClient;
	});
	const [router] = useState(() => {
		const root = createRootRoute({
			component: () => (
				<DesignSystemProvider linkComponent={RouterLink}>
					<AuthenticatedLayout>
						<Outlet />
					</AuthenticatedLayout>
				</DesignSystemProvider>
			),
		});
		const routes = [
			{ path: ROUTE_PATHS.INDEX, section: "Overview" },
			{ path: ROUTE_PATHS.PRODUCT_ROLES, section: "Roles" },
			{ path: ROUTE_PATHS.PRODUCTS, section: "Products" },
		].map(({ path, section }) =>
			createRoute({
				getParentRoute: () => root,
				path,
				component: () => <ShellDestination section={section} />,
			}),
		);
		return createRouter({
			routeTree: root.addChildren(routes),
			history: createMemoryHistory({ initialEntries: [ROUTE_PATHS.INDEX] }),
		});
	});
	return (
		<QueryClientProvider client={queryClient}>
			<RouterProvider router={router} />
		</QueryClientProvider>
	);
}

const meta = {
	title: "Layout/AppShell",
	component: ShellHarness,
	args: { availableProducts: products },
	parameters: {
		layout: "fullscreen",
		viewport: {
			options: {
				phone: { name: "Phone", styles: { width: "390px", height: "844px" } },
				smallPhone: {
					name: "Small phone",
					styles: { width: "320px", height: "851px" },
				},
				reviewPhone: {
					name: "Review phone",
					styles: { width: "393px", height: "851px" },
				},
				tablet: {
					name: "Tablet",
					styles: { width: "768px", height: "1024px" },
				},
				desktop: {
					name: "Desktop",
					styles: { width: "1200px", height: "900px" },
				},
			},
		},
	},
	globals: { viewport: { value: "desktop", isRotated: false } },
	beforeEach: () => {
		const previousProduct = localStorage.getItem("selectedProductId");
		localStorage.removeItem("selectedProductId");
		const previousConfig = client.getConfig();
		client.setConfig({
			fetch: async () => {
				throw new Error(
					"Shell stories must use cached fixtures without backend calls",
				);
			},
		});
		return () => {
			client.setConfig(previousConfig);
			if (previousProduct === null)
				localStorage.removeItem("selectedProductId");
			else localStorage.setItem("selectedProductId", previousProduct);
		};
	},
} satisfies Meta<typeof ShellHarness>;

export default meta;
type Story = StoryObj<typeof meta>;

function sidebarTrigger(canvasElement: HTMLElement) {
	const trigger = within(canvasElement)
		.getAllByRole("button", { name: "Toggle Sidebar" })
		.find((button) => button.tabIndex === 0);
	if (!trigger)
		throw new Error("The keyboard-accessible sidebar trigger is missing");
	return trigger;
}

export const WorstCase: Story = {
	args: { availableProducts: worstCaseProducts },
};

export const One: Story = {
	args: { availableProducts: [worstCaseProducts[1]] },
};

export const Empty: Story = {
	args: { availableProducts: [] },
};

function expectInsideViewport(element: HTMLElement) {
	const bounds = element.getBoundingClientRect();
	expect(bounds.width).toBeGreaterThan(0);
	expect(bounds.left).toBeGreaterThanOrEqual(0);
	expect(bounds.right).toBeLessThanOrEqual(
		document.documentElement.clientWidth,
	);
}

async function verifyProductHeader(
	canvasElement: HTMLElement,
	showPrefix: boolean,
) {
	const canvas = within(canvasElement);
	const selectedProduct = worstCaseProducts[0];
	await canvas.findByRole("heading", {
		name: `Overview for ${selectedProduct.name}`,
	});
	const selector = canvas.getByRole("button", {
		name: `Working on: ${selectedProduct.name}`,
	});
	const refresh = canvas.getByRole("button", { name: "Refresh products" });
	const prefix = within(selector).getByText("Working on:", { exact: true });
	await waitFor(() => {
		expectInsideViewport(selector);
		expectInsideViewport(refresh);
		if (showPrefix) {
			expect(prefix).toBeVisible();
			const range = document.createRange();
			range.selectNodeContents(prefix);
			const text = range.getBoundingClientRect();
			const bounds = prefix.getBoundingClientRect();
			expect(text.left).toBeGreaterThanOrEqual(bounds.left);
			expect(text.right).toBeLessThanOrEqual(bounds.right);
		} else {
			expect(prefix).not.toBeVisible();
		}
		expect(
			within(selector)
				.getByText(selectedProduct.name, { exact: true })
				.getBoundingClientRect().width,
		).toBeGreaterThan(16);
	});
	await expect(selector).toHaveAttribute("title", selectedProduct.name);
	await userEvent.click(selector);
	const menu = await screen.findByRole("menu");
	await waitFor(() => {
		expectInsideViewport(menu);
		const bounds = menu.getBoundingClientRect();
		for (const product of worstCaseProducts) {
			const name = within(menu).getByText(product.name, { exact: true });
			const range = document.createRange();
			range.selectNodeContents(name);
			const text = range.getBoundingClientRect();
			expect(text.left).toBeGreaterThanOrEqual(bounds.left);
			expect(text.right).toBeLessThanOrEqual(bounds.right);
		}
	});
	await userEvent.keyboard("{Escape}");
	await waitFor(() => expect(menu).not.toBeInTheDocument());
	await expect(selector).toHaveFocus();
}

export const SmallPhoneProductHeader: Story = {
	args: { availableProducts: worstCaseProducts },
	globals: { viewport: { value: "smallPhone", isRotated: false } },
	play: async ({ canvasElement }) => verifyProductHeader(canvasElement, false),
};

export const MobileProductHeader: Story = {
	args: { availableProducts: worstCaseProducts },
	globals: { viewport: { value: "reviewPhone", isRotated: false } },
	play: async ({ canvasElement }) => verifyProductHeader(canvasElement, false),
};

export const TabletProductHeader: Story = {
	args: { availableProducts: worstCaseProducts },
	globals: { viewport: { value: "tablet", isRotated: false } },
	play: async ({ canvasElement }) => verifyProductHeader(canvasElement, true),
};

export const LargeTextProductHeader: Story = {
	args: { availableProducts: worstCaseProducts },
	globals: { viewport: { value: "smallPhone", isRotated: false } },
	play: async ({ canvasElement }) => {
		const previousSize = document.documentElement.style.fontSize;
		try {
			document.documentElement.style.fontSize = "32px";
			await verifyProductHeader(canvasElement, false);
		} finally {
			document.documentElement.style.fontSize = previousSize;
		}
	},
};

export const DesktopNavigationAndProductSwitch: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await expect(
			await canvas.findByRole("heading", { name: "Overview for Echopoint" }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("link", { name: "Roles" }));
		await expect(
			await canvas.findByRole("heading", { name: "Roles for Echopoint" }),
		).toBeVisible();
		await expect(canvas.getByRole("link", { name: "Roles" })).toHaveAttribute(
			"aria-current",
			"page",
		);
		await userEvent.click(
			canvas.getByRole("button", { name: "Working on: Echopoint" }),
		);
		await userEvent.click(
			await screen.findByRole("menuitem", { name: "Anchor" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "Roles for Anchor" }),
		).toBeVisible();
		await expect(localStorage.getItem("selectedProductId")).toBe(
			products[1].id,
		);
	},
};

export const DesktopCollapseAndSkipLink: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("heading", { name: "Overview for Echopoint" });
		const dashboard = canvas.getByRole("link", {
			name: "Dashboard",
		});
		const roles = canvas.getByRole("link", { name: "Roles" });
		const trigger = sidebarTrigger(canvasElement);
		await expect(dashboard.getBoundingClientRect().width).toBeGreaterThan(100);
		trigger.focus();
		await userEvent.keyboard("{Enter}");
		await waitFor(() =>
			expect(dashboard.getBoundingClientRect().width).toBeLessThan(40),
		);
		await expect(roles).not.toBeVisible();
		await expect(trigger).toHaveFocus();
		await userEvent.keyboard("{Enter}");
		await waitFor(() => expect(roles).toBeVisible());

		const skipLink = canvas.getByRole("link", { name: "Skip to Main Content" });
		skipLink.focus();
		await userEvent.keyboard("{Enter}");
		await waitFor(() =>
			expect(document.activeElement?.id).toBe("app-shell-content"),
		);
	},
};

export const MobileMenuNavigationAndFocus: Story = {
	globals: { viewport: { value: "phone", isRotated: false } },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("heading", { name: "Overview for Echopoint" });
		await expect(
			canvas.queryByRole("link", { name: "Roles" }),
		).not.toBeInTheDocument();
		const trigger = sidebarTrigger(canvasElement);
		await userEvent.click(trigger);
		const sidebar = await screen.findByRole("dialog", { name: "Sidebar" });
		await waitFor(() =>
			expect(sidebar).toContainElement(document.activeElement as HTMLElement),
		);
		await userEvent.click(within(sidebar).getByRole("link", { name: "Roles" }));
		await expect(
			await canvas.findByText("Roles for Echopoint"),
		).toBeInTheDocument();
		await userEvent.keyboard("{Escape}");
		await waitFor(() => expect(sidebar).not.toBeInTheDocument());
		await expect(
			canvas.getByRole("heading", { name: "Roles for Echopoint" }),
		).toBeVisible();
		await expect(trigger).toHaveFocus();
		await userEvent.click(
			canvas.getByRole("button", { name: "Working on: Echopoint" }),
		);
		const productMenu = await screen.findByRole("menu");
		await userEvent.click(
			within(productMenu).getByRole("menuitem", { name: "Anchor" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "Roles for Anchor" }),
		).toBeVisible();
		await waitFor(() => expect(productMenu).not.toBeInTheDocument());
	},
};

export const ProductNavigationRequiresSelectedProduct: Story = {
	args: { availableProducts: [] },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await canvas.findByRole("heading", {
			name: "Overview for no selected product",
		});
		const roles = canvas.getByRole("link", { name: "Roles" });
		await expect(roles).toHaveAttribute("aria-disabled", "true");
		await expect(roles).toHaveAttribute("tabindex", "-1");
		await expect(
			canvas.getByRole("button", { name: "Product API Keys" }),
		).toBeDisabled();
		await userEvent.click(canvas.getByRole("link", { name: "Products" }));
		await expect(
			await canvas.findByRole("heading", {
				name: "Products for no selected product",
			}),
		).toBeVisible();
	},
};
