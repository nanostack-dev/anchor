import { randomUUID } from "node:crypto";
import { expect, test } from "playwright/test";
import type { AuthTokenResponse, ProductResponse } from "../src/client";

const apiURL = process.env.ANCHOR_E2E_API_URL;

test("bulk delete calls the existing endpoint for each selected product", async ({
	page,
	request,
}) => {
	const health = await request.get(`${apiURL}/health`);
	expect(health.ok()).toBeTruthy();
	const initialized = (await health.json()).tenant_initialized;
	const authResponse = await request.post(
		`${apiURL}/v1/auth/${initialized ? "login" : "register"}`,
		{
			data: {
				email: "anchor-bulk-e2e@example.com",
				password: "AnchorBulkE2e!2026",
			},
		},
	);
	expect(authResponse.ok()).toBeTruthy();
	const { accessToken } = (await authResponse.json()) as AuthTokenResponse;
	const headers = { Authorization: `Bearer ${accessToken}` };
	const user = JSON.parse(
		Buffer.from(accessToken.split(".")[1], "base64url").toString(),
	);
	await page.addInitScript(
		({ token, user }) => {
			localStorage.setItem(
				"anchor_auth_state",
				JSON.stringify({ token, user, timestamp: Date.now() }),
			);
		},
		{ token: accessToken, user },
	);

	const products: ProductResponse[] = [];
	try {
		for (const name of [
			"selected-first",
			"selected-second",
			"keep-this-product",
		]) {
			const response = await request.post(`${apiURL}/v1/products`, {
				headers,
				data: { name: `${name}-${randomUUID()}` },
			});
			expect(response.status()).toBe(201);
			products.push(await response.json());
		}
		const deletes: string[] = [];
		page.on("request", (request) => {
			if (request.method() === "DELETE")
				deletes.push(new URL(request.url()).pathname);
		});
		await page.goto("/products");
		for (const product of products.slice(0, 2)) {
			const row = page.getByRole("row", { name: new RegExp(product.name) });
			await expect(row).toBeVisible();
			await row.getByRole("checkbox", { name: "Select row" }).click();
		}
		await page.getByRole("button", { name: "Bulk actions" }).click();
		await page.getByRole("menuitem", { name: "Delete selected" }).click();
		await page
			.getByRole("alertdialog")
			.getByRole("button", { name: "Delete selected" })
			.click();
		await expect(page.getByText("2 succeeded. 0 failed.")).toBeVisible();
		expect(deletes.sort()).toEqual(
			products
				.slice(0, 2)
				.map((product) => `/v1/products/${product.id}`)
				.sort(),
		);
		for (const product of products.slice(0, 2)) {
			await expect(
				page.getByRole("row", { name: new RegExp(product.name) }),
			).toHaveCount(0);
		}
		await expect(
			page.getByRole("row", { name: new RegExp(products[2].name) }),
		).toBeVisible();
	} finally {
		for (const product of products) {
			await request.delete(`${apiURL}/v1/products/${product.id}`, { headers });
		}
	}
});
