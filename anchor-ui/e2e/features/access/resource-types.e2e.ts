import type { Page } from "playwright/test";
import type { ProductResourceTypeListResponse } from "../../../src/client";
import { type World, expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import { selectProduct } from "../../support/ui";
import {
	clearResourceTypes,
	paths,
	resourceTypeTree,
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

const typeNode = (page: Page, name: string) =>
	page
		.getByRole("region", { name: "Resource type tree" })
		.getByRole("button", { name, exact: true });

const details = (page: Page) =>
	page.getByRole("complementary", { name: "Resource type details" });

async function savedTypes(world: World) {
	const result = await world.api.post<ProductResourceTypeListResponse>(
		`${world.productPath}/resource-types/search`,
		{ pagination: { limit: 100 } },
		200,
	);
	return new Map(result.items.map((type) => [type.slug, type]));
}

async function openResourceTypes(page: Page, world: World) {
	await selectProduct(page, world.product);
	await page.goto(paths.resourceTypes);
	await expect(
		page.getByRole("heading", { name: "Resource types", exact: true }),
	).toBeVisible();
}

test.afterEach(async ({ world }) => clearResourceTypes(world));

// Covers: PRODUCT_RESOURCE_TYPES
test("access.resource-types-read: the published tree reads top down and shows what uses a type", async ({
	page,
	world,
}, testInfo) => {
	await resourceTypeTree(world, documentTree);
	const grant = await scopedPermission(world, "folder", "folder:view");
	await scopedRole(world, "folder", [grant.name], "Folder viewer");
	await openResourceTypes(page, world);

	await expect(typeNode(page, "Document")).toBeVisible();
	await expect(typeNode(page, "Organization")).toHaveAccessibleDescription(
		/Built in\./,
	);
	await expect(typeNode(page, "Folder")).toHaveAccessibleDescription(
		/Nests in itself\./,
	);
	await expect(
		page.getByRole("button", { name: /^Add a type under/ }),
	).toHaveCount(0);
	await captureReviewCheckpoint(page, testInfo, "published-tree");

	await typeNode(page, "Folder").click();
	await expect(details(page)).toContainText("1 role · 1 permission");
	await expect(details(page)).toContainText("Project");
	await page
		.getByRole("button", { name: "Edit resource types", exact: true })
		.click();
	await expect(
		details(page).getByRole("button", { name: "Delete Folder", exact: true }),
	).toBeDisabled();
	await expect(details(page)).toContainText("1 type sits under Folder.");
	await expect(details(page)).toContainText("1 role is scoped to Folder.");
	await expect(details(page)).toContainText(
		"1 permission is scoped to Folder.",
	);
	await captureReviewCheckpoint(page, testInfo, "delete-blocked");

	await typeNode(page, "Organization").click();
	await expect(details(page)).toContainText("Built in.");
	await expect(
		details(page).getByRole("button", { name: /^Delete/ }),
	).toHaveCount(0);
});

// Covers: PRODUCT_RESOURCE_TYPES
test("access.resource-types-publish: a draft adds, renames and deletes types, then publishes the whole tree", async ({
	page,
	world,
}, testInfo) => {
	await resourceTypeTree(world, [
		{ slug: "project", name: "Project", parent_types: ["organization"] },
	]);
	await openResourceTypes(page, world);
	await page
		.getByRole("button", { name: "Edit resource types", exact: true })
		.click();

	await typeNode(page, "Project").click();
	await page
		.getByRole("button", { name: "Add a type under Project", exact: true })
		.click();
	const newName = page.getByRole("textbox", { name: "New type name" });
	await expect(newName).toBeFocused();
	await newName.fill("Folder");
	await newName.press("Enter");
	await expect(page.getByText("Draft: 1 type added")).toBeVisible();
	await expect(typeNode(page, "Folder")).toBeFocused();

	await page.keyboard.press("n");
	await expect(newName).toBeFocused();
	await newName.fill("Document");
	await newName.press("Enter");
	await expect(typeNode(page, "Document")).toBeFocused();
	await page.keyboard.press("n");
	await page.keyboard.press("Escape");
	await expect(newName).toHaveCount(0);
	await expect(typeNode(page, "Document")).toBeFocused();
	await expect(page.getByText("Draft: 2 types added")).toBeVisible();

	await page.keyboard.press("ArrowUp");
	await page.keyboard.press("ArrowUp");
	await expect(typeNode(page, "Project")).toBeFocused();
	await page.keyboard.press("Enter");
	const rename = page.getByRole("textbox", { name: "Rename Project" });
	await rename.fill("Workspace");
	await rename.press("Enter");
	await expect(page.getByText("Draft: 2 types added, 1 renamed")).toBeVisible();

	await typeNode(page, "Folder").click();
	await details(page)
		.getByRole("checkbox", { name: "Nests in itself", exact: true })
		.click();
	await captureReviewCheckpoint(page, testInfo, "draft-ready");

	const publish = page.waitForResponse(
		(response) =>
			response.request().method() === "PUT" &&
			response.url().endsWith(`${world.productPath}/resource-types`),
	);
	await page.getByRole("button", { name: "Publish", exact: true }).click();
	expect((await publish).status()).toBe(200);
	await expect(
		page.getByRole("button", { name: "Edit resource types", exact: true }),
	).toBeFocused();
	const saved = await savedTypes(world);
	expect(saved.get("project")?.name).toBe("Workspace");
	expect(saved.get("folder")).toMatchObject({
		parent_types: ["project"],
		nests_in_itself: true,
	});
	expect(saved.get("document")?.parent_types).toEqual(["folder"]);

	await page.reload();
	await expect(typeNode(page, "Document")).toBeVisible();
	await expect(typeNode(page, "Workspace")).toBeVisible();
	await expect(typeNode(page, "Folder")).toContainText("nests in itself");
	await captureReviewCheckpoint(page, testInfo, "published-after-reload");

	await page
		.getByRole("button", { name: "Edit resource types", exact: true })
		.click();
	await typeNode(page, "Document").click();
	await details(page)
		.getByRole("button", { name: "Delete Document", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete type", exact: true })
		.click();
	await expect(page.getByText("Draft: 1 deleted")).toBeVisible();
	await expect(typeNode(page, "Document")).toHaveCount(0);
	await page.getByRole("button", { name: "Discard", exact: true }).click();
	await expect(typeNode(page, "Document")).toBeVisible();
	expect((await savedTypes(world)).has("document")).toBe(true);

	await page
		.getByRole("button", { name: "Edit resource types", exact: true })
		.click();
	await typeNode(page, "Document").click();
	await expect(details(page)).toContainText("0 roles · 0 permissions");
	await expect(typeNode(page, "Document")).toBeFocused();
	await page.keyboard.press("Delete");
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete type", exact: true })
		.click();
	await expect(typeNode(page, "Folder")).toBeFocused();
	const deletion = page.waitForResponse(
		(response) =>
			response.request().method() === "PUT" &&
			response.url().endsWith(`${world.productPath}/resource-types`),
	);
	await page.getByRole("button", { name: "Publish", exact: true }).click();
	expect((await deletion).status()).toBe(200);
	await expect(typeNode(page, "Document")).toHaveCount(0);
	expect((await savedTypes(world)).has("document")).toBe(false);
	await captureReviewCheckpoint(page, testInfo, "deletion-published");
});

// Covers: PRODUCT_RESOURCE_TYPES
test("access.resource-types-refused: Anchor's refusal keeps the draft and names the blocking role", async ({
	page,
	world,
}, testInfo) => {
	await resourceTypeTree(world, documentTree);
	const grant = await scopedPermission(world, "document", "document:view");
	const blocking = await scopedRole(
		world,
		"project",
		[grant.name],
		"Project reader",
	);
	await openResourceTypes(page, world);
	await page
		.getByRole("button", { name: "Edit resource types", exact: true })
		.click();
	await typeNode(page, "Document").click();
	for (const parent of ["Organization", "Folder", "Project"])
		await details(page)
			.getByRole("checkbox", { name: parent, exact: true })
			.click();
	await expect(page.getByText("Draft: 1 changed")).toBeVisible();

	const publish = page.waitForResponse(
		(response) =>
			response.request().method() === "PUT" &&
			response.url().endsWith(`${world.productPath}/resource-types`),
	);
	await page.getByRole("button", { name: "Publish", exact: true }).click();
	expect((await publish).status()).toBe(409);
	const refusal = page
		.getByRole("alert")
		.filter({ hasText: "Anchor didn't publish this tree" });
	await expect(refusal).toContainText("outside their scope");
	await expect(refusal).toContainText(`Role to fix: ${blocking.name}.`);
	await expect(page.getByText("Draft: 1 changed")).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "publish-refused");
	expect((await savedTypes(world)).get("document")?.parent_types).toEqual([
		"folder",
		"project",
	]);
});
