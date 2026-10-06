import type { ProductResponse } from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { bulkDelete, searchTable, selectProduct } from "../../support/ui";

// Covers: PRODUCTS, PRODUCT_EDIT
test(
	"product create validation, cancellation, edit, protection and persistence",
	{ tag: "@products" },
	async ({ page, world }) => {
		const name = world.name("browser-create");
		let created: ProductResponse | undefined;
		try {
			await page.goto("/products");
			await page
				.getByRole("button", { name: "Create Product", exact: true })
				.click();
			const dialog = page.getByRole("dialog");
			await expect(
				dialog.getByRole("button", { name: "Create Product", exact: true }),
			).toBeDisabled();
			await dialog.getByLabel("Product Name").fill("x");
			await expect(
				dialog.getByText("Product name must be at least 2 characters"),
			).toBeVisible();
			await dialog.getByRole("button", { name: "Cancel", exact: true }).click();
			await expect(dialog).toHaveCount(0);
			await page
				.getByRole("button", { name: "Create Product", exact: true })
				.click();
			await expect(dialog.getByLabel("Product Name")).toHaveValue("");
			await dialog.getByLabel("Product Name").fill(name);
			await dialog
				.getByLabel("Description", { exact: true })
				.fill("Created in the browser");
			const response = page.waitForResponse(
				(r) =>
					new URL(r.url()).pathname === "/v1/products" &&
					r.request().method() === "POST",
			);
			await dialog
				.getByRole("button", { name: "Create Product", exact: true })
				.click();
			const result = await response;
			expect(result.status()).toBe(201);
			created = (await result.json()) as ProductResponse;
			await expect(dialog).toHaveCount(0);
			await searchTable(page, "Search products", "/v1/products/search", name);
			await page
				.getByRole("row")
				.filter({ hasText: name })
				.getByRole("button", { name: "Edit product", exact: true })
				.click();
			await expect(
				page.getByRole("heading", { name: "Edit Product", exact: true }),
			).toBeVisible();
			await page.getByLabel("Product Name").fill(`${name}-edited`);
			await page
				.getByLabel("Description", { exact: true })
				.fill("Saved browser edit");
			await page.getByRole("tab", { name: "Config", exact: true }).click();
			await page.getByLabel("Organization API key prefix").fill("browser");
			await page
				.getByRole("switch", { name: "Protected product", exact: true })
				.check();
			await page
				.getByRole("button", { name: "Update Product", exact: true })
				.click();
			await expect(page).toHaveURL(/\/products$/);
			await searchTable(
				page,
				"Search products",
				"/v1/products/search",
				`${name}-edited`,
			);
			await expect(
				page
					.getByRole("row")
					.filter({ hasText: `${name}-edited` })
					.getByRole("button", { name: "Product protected from deletion" }),
			).toBeDisabled();
			await page.goto(`/products/${created.id}/edit`);
			await expect(page.getByLabel("Product Name")).toHaveValue(
				`${name}-edited`,
			);
			await page.getByRole("tab", { name: "Config", exact: true }).click();
			await expect(page.getByLabel("Organization API key prefix")).toHaveValue(
				"browser",
			);
			await expect(
				page.getByRole("switch", { name: "Protected product" }),
			).toBeChecked();
			await page.getByRole("switch", { name: "Protected product" }).uncheck();
			await page
				.getByRole("button", { name: "Update Product", exact: true })
				.click();
			await expect(page).toHaveURL(/\/products$/);
			await searchTable(
				page,
				"Search products",
				"/v1/products/search",
				`${name}-edited`,
			);
			await page
				.getByRole("row")
				.filter({ hasText: `${name}-edited` })
				.getByRole("button", { name: "Delete product", exact: true })
				.click();
			await page
				.getByRole("dialog")
				.getByRole("button", { name: "Cancel", exact: true })
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: `${name}-edited` }),
			).toBeVisible();
			await page
				.getByRole("row")
				.filter({ hasText: `${name}-edited` })
				.getByRole("button", { name: "Delete product", exact: true })
				.click();
			const deletion = page.waitForResponse(
				(response) =>
					new URL(response.url()).pathname === `/v1/products/${created?.id}` &&
					response.request().method() === "DELETE",
			);
			await page
				.getByRole("dialog")
				.getByRole("button", { name: "Delete Product", exact: true })
				.click();
			expect((await deletion).status()).toBe(204);
			await expect(
				page.getByRole("row").filter({ hasText: `${name}-edited` }),
			).toHaveCount(0);
			expect(
				(await world.api.context.get(`/v1/products/${created.id}`)).status(),
			).toBe(404);
		} finally {
			if (created) {
				const current = await world.api.context.get(
					`/v1/products/${created.id}`,
				);
				if (current.status() === 200)
					await world.api.put(`/v1/products/${created.id}`, {
						name: created.name,
						config: { protected: false },
					});
				await world.api.remove(`/v1/products/${created.id}`);
			}
		}
	},
);

test(
	"product selection persists and search, facets, pagination and bulk deletion stay scoped",
	{ tag: "@products" },
	async ({ page, world }) => {
		const prefix = world.name("table");
		const products: ProductResponse[] = [];
		try {
			for (let i = 0; i < 12; i++)
				products.push(
					await world.api.post<ProductResponse>("/v1/products", {
						name: `${prefix}-${String(i).padStart(2, "0")}`,
					}),
				);
			await selectProduct(page, world.product);
			await page.reload();
			await expect(
				page.getByRole("button", { name: /Working on:/ }),
			).toContainText(world.product.name);
			await searchTable(page, "Search products", "/v1/products/search", prefix);
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(10);
			await page.getByRole("button", { name: "Next", exact: true }).click();
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(2);
			await page.getByRole("button", { name: "Previous", exact: true }).click();
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(10);
			await page
				.getByRole("columnheader", { name: "Name", exact: true })
				.getByRole("button")
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: prefix }).first(),
			).toContainText(products[0].name);
			await page
				.getByRole("button", { name: "Name", exact: true })
				.filter({ has: page.getByText("Name", { exact: true }) })
				.first()
				.click();
			await page
				.getByRole("option", { name: products[0].name, exact: true })
				.click();
			await page.keyboard.press("Escape");
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(1);
			await expect(page.getByText("1 total", { exact: true })).toBeVisible();
			await page
				.getByRole("button", { name: "Clear all", exact: true })
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(10);
			for (const product of products.slice(0, 2))
				await page
					.getByRole("row")
					.filter({ hasText: product.name })
					.getByRole("checkbox", { name: "Select row" })
					.check();
			await bulkDelete(page, 2);
			for (const product of products.slice(0, 2))
				expect(
					(await world.api.context.get(`/v1/products/${product.id}`)).status(),
				).toBe(404);
			await expect(
				page.getByRole("row").filter({ hasText: products[2].name }),
			).toBeVisible();
			await searchTable(
				page,
				"Search products",
				"/v1/products/search",
				`${prefix}-missing`,
			);
			await expect(
				page.getByText("No products match your search", { exact: true }),
			).toBeVisible();
			await searchTable(page, "Search products", "/v1/products/search", prefix);
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(10);
		} finally {
			await Promise.all(
				products.map((product) =>
					world.api.remove(`/v1/products/${product.id}`),
				),
			);
		}
	},
);

// Covers: INDEX, SETTINGS_USER, SETTINGS_APP
test(
	"dashboard links reflect the selected product and settings expose their current placeholders",
	{ tag: "@shell" },
	async ({ page, world }) => {
		await selectProduct(page, world.product);
		await page.goto("/");
		await expect(
			page.getByText(`Current product · ${world.product.name}`, {
				exact: true,
			}),
		).toBeVisible();
		await page
			.getByRole("link", { name: "Products: view all", exact: true })
			.click();
		await expect(page).toHaveURL(/\/products$/);
		for (const [route, title] of [
			["/settings/user", "User Settings"],
			["/settings/app", "Application Settings"],
		]) {
			await page.goto(route);
			await expect(
				page.getByRole("heading", { name: title, exact: true }),
			).toBeVisible();
			await expect(
				page.getByText("Settings are coming soon", { exact: true }),
			).toBeVisible();
		}
	},
);
