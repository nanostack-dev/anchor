import { type Page, expect } from "playwright/test";
import type { ProductResponse } from "../../src/client";
import { runtime } from "./runtime";

export async function login(
	page: Page,
	account: { email: string; password: string },
) {
	await page.goto("/login");
	await page.getByLabel("Email", { exact: true }).fill(account.email);
	await page.getByLabel("Password", { exact: true }).fill(account.password);
	await page.getByRole("button", { name: "Login", exact: true }).click();
	await expect(page).toHaveURL(`${runtime().frontendURL}/`);
	await expect(
		page.getByRole("heading", { name: "Dashboard", exact: true }),
	).toBeVisible();
}

export async function revealAccountButton(page: Page, email: string) {
	const account = page.getByRole("button").filter({ hasText: email });
	if (!(await account.isVisible()))
		await page
			.getByRole("button", { name: "Toggle Sidebar", exact: true })
			.click();
	await expect(account).toBeVisible();
	return account;
}

export async function selectProduct(page: Page, product: ProductResponse) {
	await page.goto("/products");
	await page.getByRole("button", { name: /Working on:/ }).click();
	await page.getByRole("menuitem").filter({ hasText: product.name }).click();
	await expect(page.getByRole("button", { name: /Working on:/ })).toContainText(
		product.name,
	);
}

export async function searchTable(
	page: Page,
	placeholder: string,
	endpoint: string,
	term: string,
) {
	const response = page.waitForResponse((response) => {
		const request = response.request();
		return (
			request.method() === "POST" &&
			new URL(request.url()).pathname === endpoint &&
			request.postDataJSON()?.full_text_search === (term || undefined)
		);
	});
	await page.getByPlaceholder(placeholder, { exact: true }).fill(term);
	expect((await response).status()).toBe(200);
}

export async function bulkDelete(page: Page, count: number) {
	await page.getByRole("button", { name: "Bulk actions", exact: true }).click();
	await page
		.getByRole("menuitem", { name: "Delete selected", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete selected", exact: true })
		.click();
	await expect(
		page.getByText(`${count} succeeded. 0 failed.`, { exact: true }),
	).toBeVisible();
}
