import type {
	CreatedProductApiKeyResponse,
	ProductApiKeyResponse,
} from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { bulkDelete, searchTable, selectProduct } from "../../support/ui";

// Covers: PRODUCT_API_KEYS, PRODUCT_API_KEY_NEW, PRODUCT_API_KEY_EDIT
test(
	"API key wizard validates, reviews, reveals once, copies and edits mutable permissions",
	{ tag: "@products" },
	async ({ page, world }) => {
		await selectProduct(page, world.product);
		await page.goto("/products/product-api-keys/new");
		await expect(
			page.getByRole("button", { name: "Next", exact: true }),
		).toBeDisabled();
		const name = world.name("browser-key");
		await page.getByRole("textbox", { name: "Name *", exact: true }).fill(name);
		await page
			.getByLabel("Description", { exact: true })
			.fill("Browser-generated key");
		await page.getByRole("switch", { name: "Mutable permissions" }).check();
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await page
			.getByPlaceholder("Search permissions…", { exact: true })
			.fill("organization:read");
		await page.getByRole("button", { name: "Select all visible" }).click();
		await page.getByRole("button", { name: "Selected", exact: true }).click();
		await expect(page.getByText("1 selected", { exact: true })).toBeVisible();
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await expect(page.getByText(name, { exact: true })).toBeVisible();
		await page.getByRole("button", { name: "Back", exact: true }).click();
		await expect(page.getByText("1 selected", { exact: true })).toBeVisible();
		await page.getByRole("button", { name: "Next", exact: true }).click();
		const creation = page.waitForResponse(
			(r) =>
				new URL(r.url()).pathname === `${world.productPath}/api-keys` &&
				r.request().method() === "POST",
		);
		await page
			.getByRole("button", { name: "Create API Key", exact: true })
			.click();
		const response = await creation;
		expect(response.status()).toBe(201);
		const key = (await response.json()) as CreatedProductApiKeyResponse;
		await expect(
			page.getByRole("heading", { name: "API Key Created", exact: true }),
		).toBeVisible();
		await expect(page.getByText(key.value, { exact: true })).toHaveCount(0);
		await page.getByRole("button", { name: "Show value", exact: true }).click();
		await expect(page.getByText(key.value, { exact: true })).toBeVisible();
		await page
			.context()
			.grantPermissions(["clipboard-read", "clipboard-write"]);
		await page.getByRole("button", { name: "Copy value", exact: true }).click();
		expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
			key.value,
		);
		await page.getByRole("button", { name: "Done", exact: true }).click();
		await page
			.getByRole("row")
			.filter({ hasText: name })
			.getByRole("link", { name: "Edit API key" })
			.click();
		await page
			.getByRole("textbox", { name: "Name *", exact: true })
			.fill(`${name}-updated`);
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await page
			.getByPlaceholder("Search permissions…", { exact: true })
			.fill("organization:create");
		await page.getByRole("button", { name: "Select all visible" }).click();
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await page
			.getByRole("button", { name: "Update API Key", exact: true })
			.click();
		await expect(page).toHaveURL(/\/products\/product-api-keys$/);
		await expect(
			page.getByRole("row").filter({ hasText: `${name}-updated` }),
		).toBeVisible();
		const saved = await world.api.get<ProductApiKeyResponse>(
			`${world.productPath}/api-keys/${key.id}`,
		);
		expect(
			saved.permissions.map((permission) => permission.permission_name).sort(),
		).toEqual(["organization:create", "organization:read"]);
		await page.reload();
		await expect(page.getByText(key.value, { exact: true })).toHaveCount(0);
	},
);

test(
	"immutable API keys keep grants locked and key search, single and bulk deletion persist",
	{ tag: "@products" },
	async ({ page, world }) => {
		const keys: CreatedProductApiKeyResponse[] = [];
		const prefix = world.name("immutable-key");
		for (let i = 0; i < 3; i++)
			keys.push(
				await world.api.post<CreatedProductApiKeyResponse>(
					`${world.productPath}/api-keys`,
					{
						name: `${prefix}-${i}`,
						mutable: false,
						permissions: ["organization:read"],
					},
				),
			);
		await selectProduct(page, world.product);
		await page.goto(`/products/product-api-keys/${keys[0].id}/edit`);
		await expect(page.getByText(/created as immutable/)).toBeVisible();
		await page
			.getByLabel("Description", { exact: true })
			.fill("Immutable metadata edit");
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await expect(
			page.getByText("Permissions are immutable and cannot be changed.", {
				exact: true,
			}),
		).toBeVisible();
		await expect(page.getByPlaceholder("Search permissions…")).toHaveCount(0);
		await page
			.getByRole("button", { name: "Update API Key", exact: true })
			.click();
		await expect(page).toHaveURL(/\/products\/product-api-keys$/);
		await searchTable(
			page,
			"Search API keys",
			`${world.productPath}/api-keys/search`,
			prefix,
		);
		await expect(page.getByRole("row").filter({ hasText: prefix })).toHaveCount(
			3,
		);
		await page
			.getByRole("row")
			.filter({ hasText: keys[0].name })
			.getByRole("button", { name: "Delete API Key", exact: true })
			.click();
		await page
			.getByRole("dialog")
			.getByRole("button", { name: "Delete API Key", exact: true })
			.click();
		await expect(
			page.getByRole("row").filter({ hasText: keys[0].name }),
		).toHaveCount(0);
		for (const key of keys.slice(1))
			await page
				.getByRole("row")
				.filter({ hasText: key.name })
				.getByRole("checkbox", { name: "Select row" })
				.check();
		await bulkDelete(page, 2);
		await page.reload();
		await expect(
			page.getByText("No API keys yet", { exact: true }),
		).toBeVisible();
	},
);

test(
	"missing API key explains the failure and provides a return link",
	{ tag: "@products" },
	async ({ page, world }) => {
		const key = await world.api.post<CreatedProductApiKeyResponse>(
			`${world.productPath}/api-keys`,
			{ name: world.name("deleted-key"), permissions: ["organization:read"] },
		);
		await world.api.remove(`${world.productPath}/api-keys/${key.id}`);
		await selectProduct(page, world.product);
		let missingReads = 0;
		page.on("response", (response) => {
			if (
				new URL(response.url()).pathname ===
					`${world.productPath}/api-keys/${key.id}` &&
				response.request().method() === "GET"
			)
				missingReads++;
		});
		await page.goto(`/products/product-api-keys/${key.id}/edit`);
		await expect(
			page.getByText("This API key could not be found.", { exact: true }),
		).toBeVisible();
		expect(missingReads).toBe(1);
		await page
			.getByRole("button", { name: "Back to API Keys", exact: true })
			.click();
		await expect(page).toHaveURL(/\/products\/product-api-keys$/);
	},
);

test(
	"API key list paginates, sorts and applies Name and Status filters",
	{ tag: "@products" },
	async ({ page, world }) => {
		const prefix = world.name("paged-key");
		const keys: CreatedProductApiKeyResponse[] = [];
		for (let i = 0; i < 12; i++)
			keys.push(
				await world.api.post<CreatedProductApiKeyResponse>(
					`${world.productPath}/api-keys`,
					{ name: `${prefix}-${String(i).padStart(2, "0")}`, permissions: [] },
				),
			);
		for (const key of keys) expect(key.permissions).toEqual([]);
		await selectProduct(page, world.product);
		await page.goto("/products/product-api-keys");
		await searchTable(
			page,
			"Search API keys",
			`${world.productPath}/api-keys/search`,
			prefix,
		);
		const rows = page.getByRole("row").filter({ hasText: prefix });
		await expect(rows).toHaveCount(10);
		await page.getByRole("button", { name: "Next", exact: true }).click();
		await expect(rows).toHaveCount(2);
		await page.getByRole("button", { name: "Previous", exact: true }).click();
		await expect(rows).toHaveCount(10);
		await page
			.getByRole("columnheader", { name: "Name", exact: true })
			.getByRole("button")
			.click();
		await expect(rows.first()).toContainText(keys[0].name);
		await page
			.getByRole("button", { name: "Name", exact: true })
			.filter({ hasNot: page.getByRole("columnheader") })
			.first()
			.click();
		await page.getByRole("option", { name: keys[0].name, exact: true }).click();
		await page.keyboard.press("Escape");
		await expect(rows).toHaveCount(1);
		await expect(page.getByText("1 total", { exact: true })).toBeVisible();
		await page.getByRole("button", { name: "Clear all", exact: true }).click();
		await expect(rows).toHaveCount(10);
		await page
			.getByRole("button", { name: "Status", exact: true })
			.first()
			.click();
		await page.getByRole("option", { name: "Inactive", exact: true }).click();
		await page.keyboard.press("Escape");
		await expect(rows).toHaveCount(0);
		await expect(
			page.getByText("No API keys match your search and filters", {
				exact: true,
			}),
		).toBeVisible();
		await page.getByRole("button", { name: "Clear all", exact: true }).click();
		await expect(rows).toHaveCount(10);
		await page
			.getByRole("button", { name: "Status", exact: true })
			.first()
			.click();
		await page.getByRole("option", { name: "Active", exact: true }).click();
		await page.keyboard.press("Escape");
		await expect(page.getByText("12 total", { exact: true })).toBeVisible();
	},
);
