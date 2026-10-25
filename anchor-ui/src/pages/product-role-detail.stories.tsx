import type { ProductRoleResponse } from "@/client";
import { RouterLink } from "@/components/layout/router-link";
import { ProductRoleDatatable } from "@/components/product/roles/ProductRoleDatatable";
import { ProductProvider } from "@/context/product/ProductContext";
import {
	mockAccessBackend,
	storyPermission,
	storyType,
} from "@/lib/storybook/access-backend";
import { ROUTE_PATHS } from "@/routes/routePaths";
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
import { expect, userEvent, waitFor, within } from "storybook/test";
import ProductRoleDetailPage from "./product-role-detail";

const ROLE: ProductRoleResponse = {
	id: "role_test",
	product_id: "prd_2Nq8xKf3pLmR",
	name: "Editor",
	description: "Edit customer invoices",
	resource_type: "organization",
	key: "editor",
	is_default: false,
	permissions: [
		{
			product_id: "prd_test",
			product_role_id: "role_test",
			permission_name: "invoices:read",
		},
	],
	created_at: "2026-07-14T09:12:00Z",
	updated_at: "2026-07-14T09:12:00Z",
};
const LIST_PATH = ROUTE_PATHS.PRODUCT_ROLES;
const DETAIL_PATH = "/products/resources/roles/role_test";

function RoleRouteHarness({ initialPath }: { initialPath: string }) {
	const [queryClient] = useState(
		() => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
	);
	const [router] = useState(() => {
		const root = createRootRoute({
			component: () => (
				<DesignSystemProvider linkComponent={RouterLink}>
					<ProductProvider>
						<Outlet />
					</ProductProvider>
				</DesignSystemProvider>
			),
		});
		const list = createRoute({
			getParentRoute: () => root,
			path: LIST_PATH,
			component: () => <ProductRoleDatatable productId="prd_2Nq8xKf3pLmR" />,
		});
		const detail = createRoute({
			getParentRoute: () => root,
			path: ROUTE_PATHS.PRODUCT_ROLE_DETAIL,
			validateSearch: (search: Record<string, unknown>) => ({
				edit: search.edit === true || search.edit === "true",
			}),
			component: () => {
				const { roleId } = detail.useParams();
				const { edit } = detail.useSearch();
				return <ProductRoleDetailPage roleId={roleId} editing={!!edit} />;
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
	title: "Product/RolePages",
	component: RoleRouteHarness,
	args: { initialPath: LIST_PATH },
	parameters: { layout: "fullscreen", saveStatus: 200 },
	beforeEach: ({ parameters }) =>
		mockAccessBackend({
			types: [storyType("organization", "Organization")],
			roles: [ROLE],
			permissions: [
				storyPermission("invoices:read", "organization", "Read invoices"),
			],
			saveStatus: parameters.saveStatus,
		}).restore,
} satisfies Meta<typeof RoleRouteHarness>;
export default meta;
type Story = StoryObj<typeof meta>;

export const RowAndActionsNavigate: Story = {
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		await userEvent.click(await canvas.findByText("Editor"));
		await expect(
			await canvas.findByRole("heading", { name: "Edit Editor", level: 1 }),
		).toBeVisible();
		await userEvent.click(
			canvas.getByRole("button", { name: "Cancel editing" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "Editor", level: 1 }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("link", { name: "All roles" }));
		await userEvent.click(
			await canvas.findByRole("link", { name: "View Editor" }),
		);
		await expect(
			await canvas.findByRole("heading", { name: "Editor", level: 1 }),
		).toBeVisible();
		await userEvent.click(canvas.getByRole("link", { name: "Edit role" }));
		await expect(
			await canvas.findByRole("heading", { name: "Edit Editor", level: 1 }),
		).toBeVisible();
	},
};

export const Edit: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
};

async function advanceToReview(canvasElement: HTMLElement) {
	const canvas = within(canvasElement);
	await userEvent.click(
		await canvas.findByRole("button", { name: "Continue" }),
	);
	await userEvent.click(
		await canvas.findByRole("button", { name: "Continue" }),
	);
	await expect(
		await canvas.findByRole("button", { name: "Save changes" }),
	).toBeVisible();
}

export const SaveReturnsToView: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const name = await canvas.findByRole("textbox", { name: "Name" });
		await userEvent.clear(name);
		await userEvent.type(name, "Senior Editor");
		await advanceToReview(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Save changes" }));
		await waitFor(() =>
			expect(
				canvas.getByRole("heading", { name: "Senior Editor", level: 1 }),
			).toBeVisible(),
		);
		await expect(canvas.getByText("invoices:read")).toBeVisible();
	},
};

export const FailedSaveKeepsDraft: Story = {
	args: { initialPath: `${DETAIL_PATH}?edit=true` },
	parameters: { saveStatus: 500 },
	play: async ({ canvasElement }) => {
		const canvas = within(canvasElement);
		const name = await canvas.findByRole("textbox", { name: "Name" });
		await userEvent.clear(name);
		await userEvent.type(name, "Unsent Editor");
		await advanceToReview(canvasElement);
		await userEvent.click(canvas.getByRole("button", { name: "Save changes" }));
		await expect(await canvas.findByRole("alert")).toBeVisible();
		await expect(
			canvas.getByRole("heading", { name: "Edit Editor", level: 1 }),
		).toBeVisible();
		await expect(canvas.getByText("Unsent Editor")).toBeVisible();
	},
};
