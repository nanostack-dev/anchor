import type { Page } from "playwright/test";
import type {
	ProductOrganizationResponse,
	ProductResourcePermissionResponse,
	ProductResourceTypeTreeItem,
	ProductResourceTypeTreeResponse,
	ProductRoleResponse,
	ProductUserResponse,
} from "../../../src/client";
import { type World, expect } from "../../support/fixtures";

export const paths = {
	resourceTypes: "/products/resources/types",
	permissions: "/products/resources/permissions",
	roles: "/products/resources/roles",
};

export const row = (page: Page, name: string) =>
	page
		.getByRole("row")
		.filter({ has: page.getByRole("cell", { name, exact: true }) });

export async function permission(world: World, prefix = "document:read") {
	return world.api.post<ProductResourcePermissionResponse>(
		`${world.productPath}/resource-permissions`,
		{
			name: world.name(prefix),
			description: "Read the test document",
			resource_type: "organization",
		},
	);
}

export async function role(
	world: World,
	permissions: string[] = [],
	prefix = "Test role",
) {
	return world.api.post<ProductRoleResponse>(`${world.productPath}/roles`, {
		name: world.name(prefix),
		description: "Browser fixture role",
		resource_type: "organization",
		permissions,
	});
}

export async function organization(world: World, prefix = "Test organization") {
	return (await world.productAPI()).post<ProductOrganizationResponse>(
		`${world.productPath}/organizations`,
		{ name: world.name(prefix), description: "Browser fixture organization" },
	);
}

export async function user(world: World, prefix = "member", status = "active") {
	const name = world.name("Product member");
	const created = await (await world.productAPI()).post<ProductUserResponse>(
		`${world.productPath}/product-users`,
		{
			email: `${world.name(prefix)}@example.test`,
			name,
			status,
		},
	);
	expect(created.name).toBe(name);
	return { ...created, name };
}

export async function selectOrganization(page: Page, name: string) {
	await page
		.getByRole("combobox", { name: "Organization", exact: true })
		.click();
	await page.getByRole("option", { name, exact: true }).click();
}

export async function facet(page: Page, name: string, value: string) {
	await page.getByRole("button", { name, exact: true }).first().click();
	await page.getByRole("option", { name: value, exact: true }).click();
	await page.keyboard.press("Escape");
}

export async function bulkDelete(page: Page, names: string[]) {
	for (const name of names)
		await row(page, name).getByRole("checkbox", { name: "Select row" }).check();
	await page.getByRole("button", { name: "Bulk actions", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Delete selected", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete selected", exact: true })
		.click();
	await expect(
		page.getByText(`${names.length} succeeded. 0 failed.`, { exact: true }),
	).toBeVisible();
	for (const name of names) await expect(row(page, name)).toHaveCount(0);
}

export async function resourceTypeTree(
	world: World,
	resourceTypes: ProductResourceTypeTreeItem[],
) {
	return world.api.put<ProductResourceTypeTreeResponse>(
		`${world.productPath}/resource-types`,
		{ resource_types: resourceTypes },
	);
}

export async function scopedPermission(
	world: World,
	resourceType: string,
	prefix: string,
) {
	return world.api.post<ProductResourcePermissionResponse>(
		`${world.productPath}/resource-permissions`,
		{
			name: world.name(prefix),
			description: "Browser fixture permission",
			resource_type: resourceType,
		},
	);
}

export async function scopedRole(
	world: World,
	resourceType: string,
	permissions: string[],
	prefix: string,
) {
	return world.api.post<ProductRoleResponse>(`${world.productPath}/roles`, {
		name: world.name(prefix),
		description: "Browser fixture role",
		resource_type: resourceType,
		permissions,
	});
}
