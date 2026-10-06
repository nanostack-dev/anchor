import { expect, test } from "playwright/test";
import { owner, runtime } from "../../support/runtime";
import { login } from "../../support/ui";

// Covers: INIT
test(
	"initialize the disposable platform through the browser",
	{ tag: ["@auth", "@bootstrap"] },
	async ({ page, request }) => {
		const local = runtime();
		const account = owner();
		const health = await request.get(`${local.apiURL}/health`);
		expect(health.status()).toBe(200);
		if ((await health.json()).tenant_initialized) {
			await login(page, account);
			return;
		}
		await page.goto("/init");
		await expect(
			page.getByRole("heading", { name: "Welcome to Anchor" }),
		).toBeVisible();
		await page.getByLabel("Organization Name").fill("E2E Platform");
		await page.getByLabel("Administrator Email").fill(account.email);
		await page.getByLabel("Password", { exact: true }).fill(account.password);
		await page
			.getByLabel("Confirm Password", { exact: true })
			.fill(account.password);
		await page.getByRole("button", { name: "Launch Anchor" }).click();
		await expect(page).toHaveURL("http://127.0.0.1:3015/");
		await expect(
			page.getByRole("heading", { name: "Dashboard", exact: true }),
		).toBeVisible();
		const initialized = await request.get(`${local.apiURL}/health`);
		expect((await initialized.json()).tenant_initialized).toBe(true);
	},
);
