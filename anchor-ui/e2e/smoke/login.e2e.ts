import { type Page, type Response, expect, test } from "playwright/test";
import type {
	ProductListResponse,
	ProductSearchRequest,
} from "../../src/client";

function waitForProducts(page: Page, fullTextSearch?: string) {
	return page.waitForResponse((response) => {
		const request = response.request();
		if (
			request.method() !== "POST" ||
			new URL(request.url()).pathname !== "/v1/products/search"
		) {
			return false;
		}
		const body: ProductSearchRequest = request.postDataJSON();
		return (
			body.sort_by === "created_at" && body.full_text_search === fullTextSearch
		);
	});
}

async function expectProductsLoaded(page: Page, response: Response) {
	expect(response.status()).toBe(200);
	const products: ProductListResponse = await response.json();
	const first = products.items[0];
	if (first) {
		await expect(
			page
				.getByRole("table")
				.getByRole("cell", { name: first.name, exact: true })
				.first(),
		).toBeVisible();
	} else {
		await expect(
			page.getByText("No products yet", { exact: true }),
		).toBeVisible();
	}
}

test("guest is sent to login when opening Products", async ({ page }) => {
	await page.goto("/products");
	await expect(page).toHaveURL(/\/login\?redirect=%2Fproducts$/);
	await expect(
		page.getByRole("heading", { name: "Login to your account" }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Login", exact: true }),
	).toBeDisabled();
});

test("admin can log in, browse Products, keep the session after reload, and log out", async ({
	page,
}) => {
	const email = process.env.E2E_EMAIL;
	const password = process.env.E2E_PASSWORD;
	if (!email || !password) {
		throw new Error(
			"Set E2E_EMAIL and E2E_PASSWORD in the environment or .env.e2e.local to run the authenticated smoke test.",
		);
	}

	await page.goto("/login");
	await page.getByRole("textbox", { name: "Email", exact: true }).fill(email);
	await page.getByLabel("Password", { exact: true }).fill(password);
	await page.getByRole("button", { name: "Login", exact: true }).click();
	await expect(
		page.getByRole("link", { name: "Products", exact: true }),
	).toBeVisible();
	const initialProducts = waitForProducts(page);
	await page.getByRole("link", { name: "Products", exact: true }).click();
	await expectProductsLoaded(page, await initialProducts);
	await expect(page).toHaveURL(/\/products$/);
	await expect(
		page.getByRole("heading", { name: "Products", exact: true }),
	).toBeVisible();
	await expect(page.getByRole("table")).toBeVisible();
	await expect(
		page.getByRole("columnheader", { name: "Name", exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Create Product", exact: true }),
	).toBeEnabled();

	const search = page.getByRole("textbox");
	const searchText = "e2e-no-matching-product-000000000000000000000000";
	const searchedProducts = waitForProducts(page, searchText);
	await search.fill(searchText);
	const searchResponse = await searchedProducts;
	expect(searchResponse.status()).toBe(200);
	const searchResults: ProductListResponse = await searchResponse.json();
	expect(searchResults.items).toHaveLength(0);
	await expect(
		page.getByText("No products match your search", { exact: true }),
	).toBeVisible();
	const clearedProducts = waitForProducts(page);
	await page.getByRole("button", { name: "Clear search", exact: true }).click();
	await expectProductsLoaded(page, await clearedProducts);
	await expect(search).toHaveValue("");
	await expect(
		page.getByText("No products match your search", { exact: true }),
	).toHaveCount(0);

	const reloadedProducts = waitForProducts(page);
	await page.reload();
	await expectProductsLoaded(page, await reloadedProducts);
	await expect(page).toHaveURL(/\/products$/);
	await expect(
		page.getByRole("heading", { name: "Products", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", {
			name: new RegExp(email.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")),
		})
		.click();
	await page.getByRole("menuitem", { name: "Log out", exact: true }).click();
	await expect(page).toHaveURL(/\/login$/);
	await page.goto("/products");
	await expect(page).toHaveURL(/\/login\?redirect=%2Fproducts$/);
	await expect(
		page.getByRole("heading", { name: "Login to your account" }),
	).toBeVisible();
});
