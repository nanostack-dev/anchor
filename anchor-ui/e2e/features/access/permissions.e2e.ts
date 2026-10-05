import type {
	ProductPermissionListResponse,
	ProductResourcePermissionResponse,
	ProductRoleResponse,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { selectProduct } from "../../support/ui";
import { bulkDelete, facet, paths, permission, role, row } from "./helpers";

// Covers: PRODUCT_PERMISSIONS
test("access.catalog: built-in Anchor permissions are searchable, filtered, paginated and read-only", async ({
	page,
	world,
}) => {
	const catalog = await world.api.post<ProductPermissionListResponse>(
		`${world.productPath}/permissions/search`,
		{ pagination: { limit: 1, offset: 0 } },
		200,
	);
	expect(catalog.total).toBeGreaterThan(10);
	const first = catalog.items[0];
	await selectProduct(page, world.product);
	await page.goto("/products/permissions");
	await expect(
		page.getByRole("heading", { name: "Permissions", exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Next", exact: true }),
	).toBeEnabled();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(
		page.getByRole("button", { name: "Previous", exact: true }),
	).toBeEnabled();
	await page.getByRole("textbox").fill(first.name);
	await expect(row(page, first.name)).toBeVisible();
	await facet(page, "Name", first.name);
	await expect(row(page, first.name)).toBeVisible();
	await expect(
		page.getByRole("button", { name: /^(Create|Delete)( |$)/ }),
	).toHaveCount(0);
	await page.getByRole("textbox").fill(world.name("no-such-permission"));
	await expect(
		page.getByText("No permissions match your search and filters", {
			exact: true,
		}),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Clear search and filters", exact: true })
		.click();
	await expect(page.getByRole("textbox")).toHaveValue("");
	await expect(
		page.getByRole("button", { name: "Next", exact: true }),
	).toBeEnabled();
});

// Covers: PRODUCT_RESOURCES_PERMISSIONS
test("access.permission-list: sorting, pagination, name facets and empty search recovery", async ({
	page,
	world,
}) => {
	const grants = await Promise.all(
		Array.from({ length: 12 }, (_, index) =>
			permission(world, `catalog:${String(index).padStart(2, "0")}`),
		),
	);
	await selectProduct(page, world.product);
	await page.goto(paths.permissions);
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	await page
		.getByRole("columnheader", { name: "Name", exact: true })
		.getByRole("button")
		.click();
	await expect(row(page, grants[0].name)).toBeVisible();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(row(page, grants[11].name)).toBeVisible();
	await page.getByRole("textbox").fill(grants[0].name);
	await expect(row(page, grants[0].name)).toBeVisible();
	await expect(page.getByText("1 total", { exact: true })).toBeVisible();
	await facet(page, "Name", grants[0].name);
	await expect(row(page, grants[0].name)).toBeVisible();
	await page.getByRole("textbox").fill(world.name("missing-permission"));
	await expect(
		page.getByText("No resource permissions match your search and filters", {
			exact: true,
		}),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Clear search and filters", exact: true })
		.click();
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
});

// Covers: PRODUCT_RESOURCES_PERMISSIONS, PRODUCT_RESOURCE_PERMISSION_DETAIL
test("access.permission-lifecycle: create, view, cancel edit, persist description and delete permission", async ({
	page,
	world,
}) => {
	const name = world.name("document:read");
	await selectProduct(page, world.product);
	await page.goto(paths.permissions);
	await page
		.getByRole("button", { name: "Create Permission", exact: true })
		.click();
	const create = page.getByRole("dialog", {
		name: "Create Product Permission",
	});
	await expect(
		create.getByRole("button", { name: "Create Permission", exact: true }),
	).toBeDisabled();
	await create.getByLabel("Name", { exact: true }).fill(name);
	await create
		.getByLabel("Description", { exact: true })
		.fill("Initial description");
	await create
		.getByRole("button", { name: "Create Permission", exact: true })
		.click();
	await expect(create).toHaveCount(0);
	await expect(row(page, name)).toBeVisible();
	await row(page, name)
		.getByRole("link", { name: `View ${name}`, exact: true })
		.click();
	await expect(
		page.getByRole("region", { name: "Resource permission details" }),
	).toContainText("Initial description");
	await page
		.getByRole("link", { name: "Edit permission", exact: true })
		.click();
	await expect(page.getByLabel("Name", { exact: true })).toBeDisabled();
	await page
		.getByLabel("Description", { exact: true })
		.fill("Unsaved description");
	await page.getByRole("button", { name: "Cancel", exact: true }).click();
	await expect(
		page.getByRole("region", { name: "Resource permission details" }),
	).toContainText("Initial description");
	await page
		.getByRole("link", { name: "Edit permission", exact: true })
		.click();
	await page
		.getByLabel("Description", { exact: true })
		.fill("Updated description");
	await page.getByRole("button", { name: "Save changes", exact: true }).click();
	await expect(
		page.getByRole("region", { name: "Resource permission details" }),
	).toContainText("Updated description");
	const saved = await world.api.get<ProductResourcePermissionResponse>(
		`${world.productPath}/resource-permissions/${encodeURIComponent(name)}`,
	);
	expect(saved.description).toBe("Updated description");
	await page.reload();
	await expect(
		page.getByRole("region", { name: "Resource permission details" }),
	).toContainText("Updated description");
	await page
		.getByRole("link", { name: "All resource permissions", exact: true })
		.click();
	await row(page, name)
		.getByRole("button", { name: "Delete permission", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Cancel", exact: true })
		.click();
	await expect(row(page, name)).toBeVisible();
	await row(page, name)
		.getByRole("button", { name: "Delete permission", exact: true })
		.click();
	const deletion = page.waitForResponse(
		(response) =>
			response.request().method() === "DELETE" &&
			response
				.url()
				.endsWith(`/resource-permissions/${encodeURIComponent(name)}`),
	);
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete Permission", exact: true })
		.click();
	expect((await deletion).status()).toBe(204);
	await expect(row(page, name)).toHaveCount(0);
	await expect
		.poll(async () =>
			(
				await world.api.context.get(
					`${world.productPath}/resource-permissions/${encodeURIComponent(name)}`,
				)
			).status(),
		)
		.toBe(404);
});

// Covers: PRODUCT_RESOURCES_PERMISSIONS
test("access.permission-impact: deleting an assigned permission warns and removes the role grant", async ({
	page,
	world,
}) => {
	const grant = await permission(world);
	const holder = await role(world, [grant.name]);
	await selectProduct(page, world.product);
	await page.goto(paths.permissions);
	await row(page, grant.name)
		.getByRole("button", { name: "Delete permission", exact: true })
		.click();
	const confirmation = page.getByRole("alertdialog");
	await expect(
		confirmation.getByText(`These roles lose it: ${holder.name}`, {
			exact: true,
		}),
	).toBeVisible();
	await confirmation
		.getByRole("button", { name: "Delete Permission", exact: true })
		.click();
	await expect(row(page, grant.name)).toHaveCount(0);
	await expect
		.poll(
			async () =>
				(
					await world.api.get<ProductRoleResponse>(
						`${world.productPath}/roles/${holder.id}`,
					)
				).permissions,
		)
		.toEqual([]);
});

// Covers: PRODUCT_RESOURCES_PERMISSIONS
test("access.permission-bulk: bulk deletion keeps an unselected permission", async ({
	page,
	world,
}) => {
	const grants = await Promise.all([
		permission(world, "one:read"),
		permission(world, "two:read"),
		permission(world, "keep:read"),
	]);
	await selectProduct(page, world.product);
	await page.goto(paths.permissions);
	await bulkDelete(
		page,
		grants.slice(0, 2).map((grant) => grant.name),
	);
	await expect(row(page, grants[2].name)).toBeVisible();
	for (const grant of grants.slice(0, 2))
		expect(
			(
				await world.api.context.get(
					`${world.productPath}/resource-permissions/${encodeURIComponent(grant.name)}`,
				)
			).status(),
		).toBe(404);
	await world.api.get(
		`${world.productPath}/resource-permissions/${encodeURIComponent(grants[2].name)}`,
	);
});
