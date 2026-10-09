import type {
	ProductRoleListResponse,
	ProductRoleResponse,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import { selectProduct } from "../../support/ui";
import {
	bulkDelete,
	facet,
	organization,
	paths,
	permission,
	role,
	row,
	user,
} from "./helpers";

// Covers: PRODUCT_ROLE_DETAIL
test("access.role-empty-update: an explicit empty API replacement persists after reload", async ({
	page,
	world,
}, testInfo) => {
	const grant = await permission(world);
	const savedRole = await role(world, [grant.name]);
	await selectProduct(page, world.product);
	await page.goto(`${paths.roles}/${savedRole.id}`);
	const details = page.getByRole("region", { name: "Role details" });
	await expect(details).toContainText(grant.name);
	await captureReviewCheckpoint(page, testInfo, "assigned-role");

	const updated = await world.api.put<ProductRoleResponse>(
		`${world.productPath}/roles/${savedRole.id}`,
		{ name: savedRole.name, permissions: [] },
	);
	expect(updated.permissions).toEqual([]);
	await page.reload();
	await expect(details).toContainText("No permissions assigned");
	await expect(details).not.toContainText(grant.name);
	const persisted = await world.api.get<ProductRoleResponse>(
		`${world.productPath}/roles/${savedRole.id}`,
	);
	expect(persisted.permissions).toEqual([]);
	await captureReviewCheckpoint(page, testInfo, "empty-role-after-reload");
});

// Covers: PRODUCT_ROLES, PRODUCT_ROLE_DETAIL
test("access.role-lifecycle: create role wizard, change grants, cancel edit, reload and delete", async ({
	page,
	world,
}) => {
	const grants = await Promise.all([
		permission(world, "document:read"),
		permission(world, "document:write"),
	]);
	const name = world.name("Document reader");
	await selectProduct(page, world.product);
	await page.goto(paths.roles);
	await page.getByRole("button", { name: "Create Role", exact: true }).click();
	const wizard = page.getByRole("dialog", { name: "Create Role", exact: true });
	const basicStep = wizard.getByRole("tabpanel", {
		name: "1. Basic Info",
		exact: true,
	});
	const grantStep = wizard.getByRole("tabpanel", {
		name: "2. Permissions",
		exact: true,
	});
	const reviewStep = wizard.getByRole("tabpanel", {
		name: "3. Review",
		exact: true,
	});
	await expect(
		basicStep.getByRole("button", { name: "Continue", exact: true }),
	).toBeDisabled();
	await wizard.getByLabel("Role Name").fill(name);
	await wizard.getByLabel("Description").fill("Reads documents");
	await basicStep
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await expect(
		grantStep.getByRole("button", { name: "Continue", exact: true }),
	).toBeDisabled();
	await grantStep.getByRole("textbox").fill(grants[0].name);
	await grantStep.getByText(grants[0].name, { exact: true }).click();
	await grantStep
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await expect(reviewStep).toContainText(name);
	await expect(reviewStep).toContainText(grants[0].name);
	await reviewStep
		.getByRole("button", { name: "Create Role", exact: true })
		.click();
	await expect(wizard).toHaveCount(0);
	await expect(row(page, name)).toBeVisible();
	const created = await world.api.post<ProductRoleListResponse>(
		`${world.productPath}/roles/search`,
		{ filter: { names: [name] } },
		200,
	);
	expect(created.items).toHaveLength(1);
	const savedRole = created.items[0];
	expect(savedRole.permissions.map((grant) => grant.permission_name)).toEqual([
		grants[0].name,
	]);
	await row(page, name)
		.getByRole("link", { name: `View ${name}`, exact: true })
		.click();
	await expect(
		page.getByRole("region", { name: "Role details" }),
	).toContainText(grants[0].name);
	await page.getByRole("link", { name: "Edit role", exact: true }).click();
	await page.getByLabel("Role Name").fill("Unsaved name");
	await page
		.getByRole("button", { name: "Cancel editing", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	await page.getByRole("link", { name: "Edit role", exact: true }).click();
	const nextName = world.name("Document editor");
	await page.getByLabel("Role Name").fill(nextName);
	await page.getByLabel("Description").fill("Writes documents");
	await page
		.getByRole("tabpanel", { name: "1. Basic Info", exact: true })
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	const permissionsStep = page.getByRole("tabpanel", {
		name: "2. Permissions",
		exact: true,
	});
	await permissionsStep
		.getByText(grants[0].name, { exact: true })
		.last()
		.click();
	await permissionsStep.getByText(grants[1].name, { exact: true }).click();
	await permissionsStep
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await page
		.getByRole("tabpanel", { name: "3. Review", exact: true })
		.getByRole("button", { name: "Save Changes", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: nextName, exact: true }),
	).toBeVisible();
	const persisted = await world.api.get<ProductRoleResponse>(
		`${world.productPath}/roles/${savedRole.id}`,
	);
	expect(persisted.name).toBe(nextName);
	expect(persisted.permissions.map((grant) => grant.permission_name)).toEqual([
		grants[1].name,
	]);
	await page.reload();
	await expect(
		page.getByRole("region", { name: "Role details" }),
	).toContainText(grants[1].name);
	await expect(
		page.getByRole("region", { name: "Role details" }),
	).not.toContainText(grants[0].name);
	await page.getByRole("link", { name: "All roles", exact: true }).click();
	await row(page, nextName)
		.getByRole("button", { name: "Delete role", exact: true })
		.click();
	const deletion = page.waitForResponse(
		(response) =>
			response.request().method() === "DELETE" &&
			response.url().endsWith(`/roles/${savedRole.id}`),
	);
	await page
		.getByRole("dialog", { name: "Delete Product Role", exact: true })
		.getByRole("button", { name: "Delete Product Role", exact: true })
		.click();
	expect((await deletion).status()).toBe(204);
	await expect(row(page, nextName)).toHaveCount(0);
	await expect
		.poll(async () =>
			(
				await world.api.context.get(
					`${world.productPath}/roles/${savedRole.id}`,
				)
			).status(),
		)
		.toBe(404);
});

// Covers: PRODUCT_ROLES
test("access.role-list: sorting, pagination, name facets and empty search recovery", async ({
	page,
	world,
}) => {
	const roles = await Promise.all(
		Array.from({ length: 12 }, (_, index) =>
			role(world, [], `Catalog role ${String(index).padStart(2, "0")}`),
		),
	);
	await selectProduct(page, world.product);
	await page.goto(paths.roles);
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	await page
		.getByRole("columnheader", { name: "Name", exact: true })
		.getByRole("button")
		.click();
	await expect(row(page, roles[0].name)).toBeVisible();
	await page.getByRole("button", { name: "Next", exact: true }).click();
	await expect(row(page, roles[11].name)).toBeVisible();
	await page.getByRole("textbox").fill(roles[0].name);
	await expect(row(page, roles[0].name)).toBeVisible();
	await expect(page.getByText("1 total", { exact: true })).toBeVisible();
	await facet(page, "Name", roles[0].name);
	await expect(row(page, roles[0].name)).toBeVisible();
	await page.getByRole("textbox").fill(world.name("missing-role"));
	await expect(
		page.getByText("No roles match your search and filters", { exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Clear search and filters", exact: true })
		.click();
	await expect(page.getByText("12 total", { exact: true })).toBeVisible();
});

// Covers: PRODUCT_ROLES
test("access.role-bulk: search and bulk delete keep the unrelated role", async ({
	page,
	world,
}) => {
	const roles = await Promise.all([
		role(world, [], "Delete first"),
		role(world, [], "Delete second"),
		role(world, [], "Keep role"),
	]);
	await selectProduct(page, world.product);
	await page.goto(paths.roles);
	await page.getByRole("textbox").fill("Delete");
	await expect(row(page, roles[0].name)).toBeVisible();
	await expect(row(page, roles[1].name)).toBeVisible();
	await expect(row(page, roles[2].name)).toHaveCount(0);
	await bulkDelete(
		page,
		roles.slice(0, 2).map((item) => item.name),
	);
	await page.getByRole("button", { name: "Clear search", exact: true }).click();
	await expect(row(page, roles[2].name)).toBeVisible();
	for (const removed of roles.slice(0, 2))
		expect(
			(
				await world.api.context.get(`${world.productPath}/roles/${removed.id}`)
			).status(),
		).toBe(404);
	await world.api.get(`${world.productPath}/roles/${roles[2].id}`);
});

// Covers: PRODUCT_ROLES
test("access.role-in-use: assigned roles show a deletion error and bulk deletion keeps failed rows", async ({
	page,
	world,
}) => {
	const [assigned, deletable] = await Promise.all([
		role(world, [], "Assigned role"),
		role(world, [], "Disposable role"),
	]);
	const [tenant, member] = await Promise.all([
		organization(world),
		user(world),
	]);
	await (await world.productAPI()).post(
		`${world.productPath}/organizations/${tenant.id}/members`,
		{
			product_user_id: member.id,
			role_id: assigned.id,
		},
	);
	await selectProduct(page, world.product);
	await page.goto(paths.roles);
	await row(page, assigned.name)
		.getByRole("button", { name: "Delete role", exact: true })
		.click();
	const confirmation = page.getByRole("dialog", {
		name: "Delete Product Role",
		exact: true,
	});
	await expect(confirmation).toContainText(
		"This will fail if the role is currently assigned to any users.",
	);
	const deletion = page.waitForResponse(
		(response) =>
			response.request().method() === "DELETE" &&
			response.url().endsWith(`/roles/${assigned.id}`),
	);
	await confirmation
		.getByRole("button", { name: "Delete Product Role", exact: true })
		.click();
	expect((await deletion).status()).toBe(409);
	await expect(confirmation).toContainText(
		`You cannot delete product role ${assigned.id}.`,
	);
	await confirmation
		.getByRole("button", { name: "Cancel", exact: true })
		.click();
	await expect(row(page, assigned.name)).toBeVisible();
	for (const item of [assigned, deletable])
		await row(page, item.name)
			.getByRole("checkbox", { name: "Select row" })
			.check();
	await page.getByRole("button", { name: "Bulk actions", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Delete selected", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete selected", exact: true })
		.click();
	await expect(
		page.getByText("1 succeeded. 1 failed.", { exact: true }),
	).toBeVisible();
	await expect(row(page, assigned.name)).toBeVisible();
	await expect(row(page, deletable.name)).toHaveCount(0);
	await world.api.get(`${world.productPath}/roles/${assigned.id}`);
	expect(
		(
			await world.api.context.get(`${world.productPath}/roles/${deletable.id}`)
		).status(),
	).toBe(404);
});

// Covers: PRODUCT_ROLE_DETAIL, PRODUCT_RESOURCE_PERMISSION_DETAIL
test("access.missing-details: missing role and permission show recovery links", async ({
	page,
	world,
}) => {
	const removedRole = await role(world);
	const removedPermission = await permission(world);
	await world.api.remove(`${world.productPath}/roles/${removedRole.id}`);
	await world.api.remove(
		`${world.productPath}/resource-permissions/${encodeURIComponent(removedPermission.name)}`,
	);
	const reads = new Map<string, number>();
	page.on("response", (response) => {
		if (response.request().method() === "GET" && response.status() === 404) {
			const route = new URL(response.url()).pathname;
			reads.set(route, (reads.get(route) ?? 0) + 1);
		}
	});
	await selectProduct(page, world.product);
	await page.goto(`${paths.roles}/${removedRole.id}`);
	await expect(
		page.getByRole("heading", { name: "Could not load role", exact: true }),
	).toBeVisible();
	expect(reads.get(`${world.productPath}/roles/${removedRole.id}`)).toBe(1);
	await expect(
		page.getByRole("link", { name: "All roles", exact: true }),
	).toBeVisible();
	await page.goto(
		`${paths.permissions}/${encodeURIComponent(removedPermission.name)}`,
	);
	await expect(
		page.getByRole("heading", {
			name: "Could not load resource permission",
			exact: true,
		}),
	).toBeVisible();
	expect(
		reads.get(
			`${world.productPath}/resource-permissions/${encodeURIComponent(removedPermission.name)}`,
		),
	).toBe(1);
	await expect(
		page.getByRole("link", { name: "All resource permissions", exact: true }),
	).toBeVisible();
});
