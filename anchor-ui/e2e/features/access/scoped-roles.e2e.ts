import type { Page } from "playwright/test";
import type {
	ProductResourceTypeListResponse,
	ProductRoleListResponse,
	ProductRoleResponse,
} from "../../../src/client";
import { type World, expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import { selectProduct } from "../../support/ui";
import {
	facet,
	organization,
	paths,
	resourceTypeTree,
	row,
	scopedPermission,
	scopedRole,
} from "./helpers";

const documentTree = [
	{ slug: "project", name: "Project", parent_types: ["organization"] },
	{
		slug: "folder",
		name: "Folder",
		parent_types: ["project"],
		nests_in_itself: true,
	},
	{ slug: "document", name: "Document", parent_types: ["folder", "project"] },
];

async function scopedCatalog(world: World) {
	await resourceTypeTree(world, documentTree);
	const [billing, manage, view, read] = await Promise.all([
		scopedPermission(world, "organization", "billing:read"),
		scopedPermission(world, "project", "project:manage"),
		scopedPermission(world, "folder", "folder:view"),
		scopedPermission(world, "document", "document:view"),
	]);
	return { billing, manage, view, read };
}

async function savedRole(world: World, name: string) {
	const found = await world.api.post<ProductRoleListResponse>(
		`${world.productPath}/roles/search`,
		{ filter: { names: [name] } },
		200,
	);
	expect(found.items).toHaveLength(1);
	return found.items[0];
}

async function chooseScope(page: Page, name: string) {
	await page
		.getByRole("combobox", { name: "Resource type", exact: true })
		.click();
	await page.getByRole("option", { name, exact: true }).click();
}

const step = (page: Page, name: string) =>
	page.getByRole("tabpanel", { name, exact: true });

// Covers: PRODUCT_ROLE_NEW, PRODUCT_ROLE_DETAIL
test("access.scoped-role-create: a role names its scope first and holds only permissions at or below it", async ({
	page,
	world,
}, testInfo) => {
	const grants = await scopedCatalog(world);
	const name = world.name("Project lead");
	await selectProduct(page, world.product);
	await page.goto(`${paths.roles}/new`);

	const scope = step(page, "1. Scope");
	await expect(
		scope.getByRole("button", { name: "Continue", exact: true }),
	).toBeDisabled();
	await expect(scope).toContainText("Choose a resource type to continue.");
	await captureReviewCheckpoint(page, testInfo, "scope-required");
	await chooseScope(page, "Project");
	await expect(scope).toContainText("Not offered: Organization.");
	await captureReviewCheckpoint(page, testInfo, "scope-project");
	await scope.getByRole("button", { name: "Continue", exact: true }).click();

	const details = step(page, "2. Details");
	await details.getByLabel("Name", { exact: true }).fill(name);
	await details.getByRole("button", { name: "Continue", exact: true }).click();

	const picker = step(page, "3. Permissions");
	for (const type of ["Project", "Folder", "Document"])
		await expect(
			picker.getByRole("heading", { name: type, exact: true }),
		).toBeVisible();
	await expect(
		picker.getByRole("heading", { name: "Organization", exact: true }),
	).toHaveCount(0);
	await expect(
		picker.getByRole("checkbox", { name: grants.billing.name }),
	).toHaveCount(0);
	await expect(picker).toContainText("Organization isn't offered");
	await picker
		.getByRole("button", { name: "Select all Project permissions" })
		.click();
	await picker
		.getByRole("checkbox", { name: grants.read.name, exact: true })
		.check();
	await expect(picker).toContainText("2 of 3 selected");
	await captureReviewCheckpoint(page, testInfo, "grouped-permissions");
	await picker.getByRole("button", { name: "Continue", exact: true }).click();

	const review = step(page, "4. Review");
	await expect(review).toContainText("Project");
	await expect(review).toContainText(grants.manage.name);
	await expect(review).toContainText(grants.read.name);
	await review
		.getByRole("button", { name: "Create role", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const created = await savedRole(world, name);
	expect(created.resource_type).toBe("project");
	expect(
		created.permissions.map((grant) => grant.permission_name).sort(),
	).toEqual([grants.manage.name, grants.read.name].sort());
	await expect(
		page.getByRole("region", { name: "Role details" }),
	).toContainText("Project");
	await captureReviewCheckpoint(page, testInfo, "created-role");

	await page.getByRole("link", { name: "Edit role", exact: true }).click();
	const editDetails = step(page, "1. Details");
	await expect(editDetails.getByLabel("Resource type")).toHaveValue("Project");
	await editDetails
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	const editPicker = step(page, "2. Permissions");
	await expect(
		editPicker.getByRole("checkbox", { name: grants.billing.name }),
	).toHaveCount(0);
	await editPicker
		.getByRole("checkbox", { name: grants.view.name, exact: true })
		.check();
	await editPicker
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await step(page, "3. Review")
		.getByRole("button", { name: "Save changes", exact: true })
		.click();
	await expect(
		page.getByRole("region", { name: "Role details" }),
	).toContainText(grants.view.name);
	const edited = await world.api.get<ProductRoleResponse>(
		`${world.productPath}/roles/${created.id}`,
	);
	expect(edited.resource_type).toBe("project");
	expect(
		edited.permissions.map((grant) => grant.permission_name).sort(),
	).toEqual([grants.manage.name, grants.view.name, grants.read.name].sort());
});

// Covers: PRODUCT_ROLE_NEW, PRODUCT_RESOURCE_PERMISSION_NEW
test("access.scoped-role-empty: a scope with nothing to add explains why and links to a scoped permission", async ({
	page,
	world,
}, testInfo) => {
	await resourceTypeTree(world, documentTree);
	await scopedPermission(world, "organization", "members:update");
	const name = world.name("Document reader");
	await selectProduct(page, world.product);
	await page.goto(`${paths.roles}/new`);
	await chooseScope(page, "Document");
	await step(page, "1. Scope")
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await step(page, "2. Details").getByLabel("Name", { exact: true }).fill(name);
	await step(page, "2. Details")
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	const picker = step(page, "3. Permissions");
	await expect(picker).toContainText("No permissions to add");
	await expect(picker).toContainText(
		"Organization, Project, Folder aren't offered",
	);
	await captureReviewCheckpoint(page, testInfo, "empty-scope");
	await picker.getByRole("button", { name: "Continue", exact: true }).click();
	await expect(step(page, "4. Review")).toContainText("None.");
	await step(page, "4. Review")
		.getByRole("button", { name: "Create role", exact: true })
		.click();
	await expect(page.getByRole("heading", { name, exact: true })).toBeVisible();
	const created = await savedRole(world, name);
	expect(created.resource_type).toBe("document");
	expect(created.permissions).toEqual([]);

	await page.goto(`${paths.roles}/new`);
	await chooseScope(page, "Document");
	await step(page, "1. Scope")
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await step(page, "2. Details")
		.getByLabel("Name", { exact: true })
		.fill(world.name("Unsaved"));
	await step(page, "2. Details")
		.getByRole("button", { name: "Continue", exact: true })
		.click();
	await step(page, "3. Permissions")
		.getByRole("link", { name: "Create a permission", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: "New permission", exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("combobox", { name: "Resource type", exact: true }),
	).toContainText("Document");
	await expect(page.getByLabel("Name", { exact: true })).toHaveValue(
		"document:",
	);
});

// Covers: PRODUCT_ROLES, PRODUCT_RESOURCES_PERMISSIONS, PRODUCT_RESOURCE_TYPES
test("access.scoped-tables: roles and permissions show their scope, counts and holding roles, filtered from the canvas", async ({
	page,
	world,
}, testInfo) => {
	const grants = await scopedCatalog(world);
	const [member, lead, viewer] = await Promise.all([
		world.api.post<ProductRoleResponse>(`${world.productPath}/roles`, {
			name: world.name("member"),
			resource_type: "organization",
			permissions: [],
		}),
		scopedRole(
			world,
			"project",
			[grants.manage.name, grants.read.name],
			"project-admin",
		),
		scopedRole(
			world,
			"folder",
			[grants.view.name, grants.read.name],
			"folder-viewer",
		),
	]);
	await selectProduct(page, world.product);
	await page.goto(paths.roles);
	await expect(row(page, member.name)).toContainText("Organization");
	await expect(row(page, lead.name)).toContainText("Project");
	await expect(
		row(page, lead.name).getByRole("cell", { name: "2", exact: true }),
	).toBeVisible();
	await expect(row(page, viewer.name)).toContainText("Folder");
	await captureReviewCheckpoint(page, testInfo, "roles-scope-column");
	await facet(page, "Resource type", "Folder");
	await expect(page).toHaveURL(/resource_type=folder/);
	await expect(row(page, viewer.name)).toBeVisible();
	await expect(row(page, lead.name)).toHaveCount(0);

	await page.goto(paths.permissions);
	await expect(row(page, grants.read.name)).toContainText("Document");
	const holding = page.getByLabel(`Roles holding ${grants.read.name}`);
	await expect(holding).toContainText(lead.name);
	await expect(holding).toContainText(viewer.name);
	await expect(row(page, grants.billing.name)).toContainText("No roles");
	await captureReviewCheckpoint(page, testInfo, "permissions-roles-column");

	await page.goto(paths.resourceTypes);
	await page
		.getByRole("region", { name: "Resource type tree" })
		.getByRole("button", { name: "Folder", exact: true })
		.click();
	const details = page.getByRole("complementary", {
		name: "Resource type details",
	});
	await expect(details).toContainText("1 role · 1 permission");
	await details.getByRole("link", { name: "View roles", exact: true }).click();
	await expect(page).toHaveURL(
		/\/products\/resources\/roles\?resource_type=folder$/,
	);
	await expect(row(page, viewer.name)).toBeVisible();
	await expect(row(page, lead.name)).toHaveCount(0);
	await expect(row(page, member.name)).toHaveCount(0);
	await page.goBack();
	await page
		.getByRole("region", { name: "Resource type tree" })
		.getByRole("button", { name: "Folder", exact: true })
		.click();
	await details
		.getByRole("link", { name: "View permissions", exact: true })
		.click();
	await expect(page).toHaveURL(
		/\/products\/resources\/permissions\?resource_type=folder$/,
	);
	await expect(row(page, grants.view.name)).toBeVisible();
	await expect(row(page, grants.read.name)).toHaveCount(0);
	await captureReviewCheckpoint(page, testInfo, "permissions-from-canvas");
});

// Covers: PRODUCT_RESOURCE_TYPES
test("access.custom-roles-switch: Organization's custom roles switch publishes with the draft; other types can't turn it on", async ({
	page,
	world,
}, testInfo) => {
	await resourceTypeTree(world, documentTree);
	await selectProduct(page, world.product);
	await page.goto(paths.resourceTypes);
	const tree = page.getByRole("region", { name: "Resource type tree" });
	const details = page.getByRole("complementary", {
		name: "Resource type details",
	});
	await tree.getByRole("button", { name: "Organization", exact: true }).click();
	await expect(details).toContainText("Custom roles off");
	await page
		.getByRole("button", { name: "Edit resource types", exact: true })
		.click();
	await tree.getByRole("button", { name: "Folder", exact: true }).click();
	await expect(
		details.getByRole("switch", { name: "Custom roles", exact: true }),
	).toBeDisabled();
	await expect(details).toContainText(
		"Anchor accepts custom roles on Organization only, for now.",
	);
	await tree.getByRole("button", { name: "Organization", exact: true }).click();
	const toggle = details.getByRole("switch", {
		name: "Custom roles",
		exact: true,
	});
	await expect(toggle).toBeEnabled();
	await toggle.click();
	await expect(
		page.getByText("Draft: custom roles switched on 1 type"),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "switch-draft");
	await page.getByRole("button", { name: "Publish", exact: true }).click();
	await expect(
		page.getByRole("button", { name: "Edit resource types", exact: true }),
	).toBeVisible();
	await expect(details).toContainText("Custom roles on");
	const saved = await world.api.post<ProductResourceTypeListResponse>(
		`${world.productPath}/resource-types/search`,
		{ pagination: { limit: 100 } },
		200,
	);
	expect(
		saved.items.find((type) => type.slug === "organization")
			?.allows_custom_roles,
	).toBe(true);
	expect(
		saved.items.find((type) => type.slug === "folder")?.allows_custom_roles,
	).toBe(false);
	await page.reload();
	await tree.getByRole("button", { name: "Organization", exact: true }).click();
	await expect(details).toContainText("Custom roles on");
	await captureReviewCheckpoint(page, testInfo, "switch-published");
});

// Covers: ORGANIZATION_LICENSE_DETAIL, ORGANIZATION_ROLES
test("access.organization-roles: an organization's roles are read-only and marked managed or custom", async ({
	page,
	world,
}, testInfo) => {
	const grants = await scopedCatalog(world);
	const [admin, member] = await Promise.all([
		scopedRole(world, "organization", [grants.billing.name], "admin"),
		world.api.post<ProductRoleResponse>(`${world.productPath}/roles`, {
			name: world.name("member"),
			resource_type: "organization",
			permissions: [],
		}),
	]);
	const lead = await scopedRole(world, "project", [grants.manage.name], "lead");
	const tenant = await organization(world);
	const types = await world.api.post<ProductResourceTypeListResponse>(
		`${world.productPath}/resource-types/search`,
		{ pagination: { limit: 100 } },
		200,
	);
	const root = types.items.find((type) => type.slug === "organization");
	expect(root).toBeDefined();
	expect(
		(
			await world.api.context.patch(
				`${world.productPath}/resource-types/${root?.id}`,
				{ data: { allows_custom_roles: true } },
			)
		).status(),
	).toBe(200);
	const custom = await world.api.post<ProductRoleResponse>(
		`${world.productPath}/organizations/${tenant.id}/roles`,
		{
			name: world.name("Billing auditor"),
			resource_type: "organization",
			permissions: [grants.billing.name, grants.read.name],
		},
	);

	await selectProduct(page, world.product);
	await page.goto(`/organizations/license/${tenant.id}`);
	await expect(
		page.getByRole("heading", { name: tenant.name, exact: true }),
	).toBeVisible();
	await page
		.getByRole("navigation", { name: "Organization sections", exact: true })
		.getByRole("link", { name: "Roles", exact: true })
		.click();
	await expect(page).toHaveURL(
		new RegExp(`/organizations/license/${tenant.id}/roles$`),
	);
	await expect(row(page, admin.name)).toContainText("Managed");
	await expect(row(page, member.name)).toContainText("Managed");
	await expect(row(page, lead.name)).toContainText("Managed");
	await expect(row(page, lead.name)).toContainText("Project");
	await expect(row(page, custom.name)).toContainText("Custom");
	await expect(
		row(page, custom.name).getByRole("cell", { name: "2", exact: true }),
	).toBeVisible();
	await expect(page.getByText("Read only.")).toBeVisible();
	await expect(page.getByRole("checkbox", { name: "Select row" })).toHaveCount(
		0,
	);
	await expect(
		page.getByRole("button", { name: /^(Create|Delete|Edit)/ }),
	).toHaveCount(0);
	await captureReviewCheckpoint(page, testInfo, "organization-roles");

	await facet(page, "Resource type", "Project");
	await expect(row(page, lead.name)).toBeVisible();
	await expect(row(page, admin.name)).toHaveCount(0);
	await page
		.getByRole("navigation", { name: "Organization sections", exact: true })
		.getByRole("link", { name: "License", exact: true })
		.click();
	await expect(
		page.getByText(
			"This organization has not been instantiated onto a license template.",
			{ exact: false },
		),
	).toBeVisible();
});
