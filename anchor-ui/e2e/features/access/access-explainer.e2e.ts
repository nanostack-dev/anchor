import type { Page } from "playwright/test";
import type {
	ProductOrganizationResponse,
	ProductUserResponse,
	ResourceResponse,
} from "../../../src/client";
import { type World, expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import { selectProduct } from "../../support/ui";
import {
	organization,
	paths,
	resourceTypeTree,
	role,
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

async function register(
	world: World,
	tenant: ProductOrganizationResponse,
	type: string,
	name: string,
	parent: { type: string; id: string },
) {
	const id = world.name(type);
	return (await world.productAPI()).put<ResourceResponse>(
		`${world.productPath}/organizations/${tenant.id}/resources/${type}/${id}`,
		{ name, parent },
	);
}

async function folderViewerOnDesign(world: World) {
	await resourceTypeTree(world, documentTree);
	const view = await scopedPermission(world, "document", "document:view");
	const [folderViewer, memberRole, tenant] = await Promise.all([
		scopedRole(world, "folder", [view.name], "folder-viewer"),
		role(world, [], "member"),
		organization(world, "Acme Corp"),
	]);
	const tenantAPI = await world.productAPI();
	const dan = await tenantAPI.post<ProductUserResponse>(
		`${world.productPath}/product-users`,
		{
			email: `${world.name("dan")}@example.test`,
			name: world.name("Dan Okafor"),
			status: "active",
			external_id: world.name("user_dan"),
		},
	);
	await tenantAPI.post(
		`${world.productPath}/organizations/${tenant.id}/members`,
		{ product_user_id: dan.id, role_id: memberRole.id },
	);
	const website = await register(world, tenant, "project", "Website redesign", {
		type: "organization",
		id: tenant.id,
	});
	const design = await register(world, tenant, "folder", "Design", {
		type: "project",
		id: website.id,
	});
	const specs = await register(world, tenant, "folder", "Specs", {
		type: "folder",
		id: design.id,
	});
	const pricing = await register(world, tenant, "document", "Pricing page", {
		type: "folder",
		id: specs.id,
	});
	await tenantAPI.put(
		`${world.productPath}/organizations/${tenant.id}/resources/folder/${design.id}/members/${dan.id}`,
		{ role_ids: [folderViewer.id] },
	);
	return { view, folderViewer, tenant, dan, website, pricing };
}

const panel = (page: Page) =>
	page.getByRole("region", { name: "Check access" });

const typeNode = (page: Page, name: string) =>
	page
		.getByRole("region", { name: "Resource type tree" })
		.getByRole("button", { name, exact: true });

async function pick(page: Page, label: string, term: string, option: string) {
	const input = panel(page).getByRole("combobox", { name: label, exact: true });
	await expect(input).toBeEnabled();
	await input.click();
	await input.fill(term);
	await page
		.getByRole("listbox", { name: `${label}s` })
		.getByRole("option")
		.filter({ hasText: option })
		.click();
	await expect(input).toHaveValue(option);
}

async function chooseType(page: Page, name: string) {
	await panel(page)
		.getByRole("combobox", { name: "Resource type", exact: true })
		.click();
	await page
		.getByRole("listbox", { name: "Resource types" })
		.getByRole("option", { name, exact: true })
		.click();
}

async function openResourceTypes(page: Page, world: World) {
	await selectProduct(page, world.product);
	await page.goto(paths.resourceTypes);
	await expect(typeNode(page, "Document")).toBeVisible();
}

async function check(page: Page, world: World) {
	const evaluation = page.waitForResponse(
		(response) =>
			response.request().method() === "POST" &&
			response.url().endsWith(`${world.productPath}/access/v1/evaluation`),
	);
	await panel(page).getByRole("button", { name: "Check", exact: true }).click();
	const response = await evaluation;
	expect(response.status()).toBe(200);
	expect(response.request().postDataJSON()).toMatchObject({
		options: { explain: true },
	});
}

// Covers: PRODUCT_RESOURCE_TYPES
test("access.explainer-allowed: allowed via folder-viewer on folder Design for a document under it", async ({
	page,
	world,
}, testInfo) => {
	const { view, folderViewer, tenant, dan } = await folderViewerOnDesign(world);
	await openResourceTypes(page, world);

	const checkButton = panel(page).getByRole("button", {
		name: "Check",
		exact: true,
	});
	await expect(checkButton).toBeDisabled();
	await pick(page, "Organization", "Acme", tenant.name);
	await pick(page, "Member", "Dan", dan.name ?? dan.email);
	await pick(page, "Permission", "document:view", view.name);
	await chooseType(page, "Document");
	await expect(checkButton).toBeDisabled();
	await pick(page, "Resource", "Pricing", "Pricing page");
	await captureReviewCheckpoint(page, testInfo, "all-four-chosen");
	await check(page, world);

	const decision = panel(page).getByRole("region", { name: "Access decision" });
	await expect(decision).toContainText("Allowed");
	await expect(decision).toContainText(
		`${dan.name} has ${view.name} on Pricing page.`,
	);
	await expect(
		decision.getByRole("list", { name: "Granted by" }).getByRole("listitem"),
	).toHaveText(`${folderViewer.name} role on Folder Design`);
	await expect(
		panel(page)
			.getByRole("list", { name: "Path from the resource to the Organization" })
			.getByRole("listitem"),
	).toHaveText([
		"Pricing page (Document)",
		"→ Specs (Folder)",
		`→ Design (Folder)◀granted by${folderViewer.name}`,
		"→ Website redesign (Project)",
		`→ ${tenant.name} (Organization)`,
	]);
	await expect(typeNode(page, "Folder")).toHaveAccessibleDescription(
		`Slug folder. Nests in itself. On the checked path. Grants access: ${folderViewer.name}.`,
	);
	for (const name of ["Document", "Project", "Organization"])
		await expect(typeNode(page, name)).toHaveAccessibleDescription(
			/On the checked path\.$/,
		);
	await panel(page)
		.getByRole("button", { name: "Show on the canvas", exact: true })
		.click();
	await expect(typeNode(page, "Folder")).toBeInViewport();
	await captureReviewCheckpoint(page, testInfo, "allowed-path-on-canvas");
	await panel(page)
		.getByRole("list", { name: "Path from the resource to the Organization" })
		.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "allowed-explained");
});

// Covers: PRODUCT_RESOURCE_TYPES
test("access.explainer-denied: denied with the reason for a project above the folder role", async ({
	page,
	world,
}, testInfo) => {
	const { view, tenant, dan } = await folderViewerOnDesign(world);
	await openResourceTypes(page, world);

	await pick(page, "Organization", "Acme", tenant.name);
	await pick(page, "Member", "Dan", dan.name ?? dan.email);
	await pick(page, "Permission", "document:view", view.name);
	await chooseType(page, "Project");
	await pick(page, "Resource", "Website", "Website redesign");
	await check(page, world);

	const decision = panel(page).getByRole("region", { name: "Access decision" });
	await expect(decision).toContainText("Denied");
	await expect(decision).toContainText(
		`No role ${dan.name} holds on Website redesign or above it grants ${view.name}.`,
	);
	await expect(decision).toContainText("Roles reach down the tree, never up");
	await expect(decision).toContainText("MISSING_PERMISSION");
	await expect(
		panel(page)
			.getByRole("list", { name: "Path from the resource to the Organization" })
			.getByRole("listitem"),
	).toHaveText([
		"Website redesign (Project)",
		`→ ${tenant.name} (Organization)`,
	]);
	await expect(typeNode(page, "Project")).toHaveAccessibleDescription(
		/On the checked path\.$/,
	);
	await expect(typeNode(page, "Folder")).toHaveAccessibleDescription(
		"Slug folder. Nests in itself.",
	);
	await panel(page)
		.getByRole("list", { name: "Path from the resource to the Organization" })
		.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "denied-explained");

	await chooseType(page, "Document");
	await expect(decision).toHaveCount(0);
	await expect(typeNode(page, "Project")).toHaveAccessibleDescription(
		"Slug project.",
	);
});
