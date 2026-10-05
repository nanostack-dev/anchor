import { randomUUID } from "node:crypto";
import type {
	AuthTokenResponse,
	PlatformInvitationResponse,
} from "../../../src/client";
import { createAPI } from "../../support/api";
import { expect, test, unique } from "../../support/fixtures";
import { runtime } from "../../support/runtime";

test.use({ storageState: { cookies: [], origins: [] } });
test.describe.configure({ mode: "parallel" });

// Covers: LOGIN
test(
	"guest protection preserves the requested route and search",
	{ tag: "@auth" },
	async ({ page }) => {
		await page.goto("/products?e2e=guard");
		await expect(page).toHaveURL(
			/\/login\?redirect=%2Fproducts%3Fe2e%3Dguard$/,
		);
		await expect(
			page.getByRole("heading", { name: "Login to your account" }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "Login", exact: true }),
		).toBeDisabled();
		await page.getByLabel("Email", { exact: true }).fill("invalid-email");
		await page.getByLabel("Password", { exact: true }).fill("short");
		await expect(
			page.getByText("Invalid email address", { exact: true }),
		).toBeVisible();
		await expect(
			page.getByRole("button", { name: "Login", exact: true }),
		).toBeDisabled();
	},
);

test(
	"invalid credentials produce a real authentication failure",
	{ tag: "@auth" },
	async ({ page }) => {
		await page.goto("/login");
		await page
			.getByLabel("Email", { exact: true })
			.fill(`${unique("missing")}@example.test`);
		await page
			.getByLabel("Password", { exact: true })
			.fill("LocalA1!wrong-password");
		const failure = page.waitForResponse(
			(r) =>
				new URL(r.url()).pathname === "/v1/auth/login" &&
				r.request().method() === "POST",
		);
		await page.getByRole("button", { name: "Login", exact: true }).click();
		expect((await failure).status()).toBe(401);
		await expect(
			page.getByRole("region", { name: "Notifications" }),
		).toContainText(/credential|password|user|login/i);
		await expect(page).toHaveURL(/\/login$/);
	},
);

test(
	"login follows an internal redirect, reloads, refreshes cookies and logs out",
	{ tag: "@auth" },
	async ({ page, session }) => {
		const api = await createAPI(runtime().apiURL);
		const account = {
			email: `${unique("session")}@example.test`,
			password: `LocalA1!${randomUUID()}`,
		};
		const invitation = await session.ownerAPI.post<PlatformInvitationResponse>(
			"/v1/platform-invitations",
			{ email: account.email },
		);
		const auth = await api.post<AuthTokenResponse>(
			"/v1/auth/register",
			{ ...account, invitation_code: invitation.code },
			200,
		);
		const userAPI = await createAPI(runtime().apiURL, auth.accessToken);
		const currentUser = await userAPI.get<{ id: string }>("/v1/me");
		try {
			await page.goto("/products");
			await expect(page).toHaveURL(/\/login\?redirect=%2Fproducts$/);
			await page.getByLabel("Email", { exact: true }).fill(account.email);
			await page.getByLabel("Password", { exact: true }).fill(account.password);
			await page.getByRole("button", { name: "Login", exact: true }).click();
			await expect(page).toHaveURL(/\/products$/);
			await page.reload();
			await expect(
				page.getByRole("heading", { name: "Products", exact: true }),
			).toBeVisible();
			await page.evaluate(() => localStorage.removeItem("anchor_auth_state"));
			const refresh = page.waitForResponse(
				(r) => new URL(r.url()).pathname === "/v1/auth/refresh",
			);
			await page.reload();
			expect((await refresh).status()).toBe(200);
			await expect(
				page.getByRole("heading", { name: "Products", exact: true }),
			).toBeVisible();
			await page.getByRole("button").filter({ hasText: account.email }).click();
			const logout = page.waitForResponse(
				(r) => new URL(r.url()).pathname === "/v1/auth/logout",
			);
			await page
				.getByRole("menuitem", { name: "Log out", exact: true })
				.click();
			expect((await logout).status()).toBe(204);
			await expect(page).toHaveURL(/\/login$/);
			await page.goto("/products");
			await expect(page).toHaveURL(/\/login\?redirect=%2Fproducts$/);
		} finally {
			await session.ownerAPI.remove(`/v1/platform-users/${currentUser.id}`);
			await userAPI.context.dispose();
			await api.context.dispose();
		}
	},
);

test(
	"external login redirect stays on the Anchor origin",
	{ tag: "@auth" },
	async ({ page, session }) => {
		await page.goto("/login?redirect=https%3A%2F%2Fexample.invalid%2Fsteal");
		await page.getByLabel("Email", { exact: true }).fill(session.account.email);
		await page
			.getByLabel("Password", { exact: true })
			.fill(session.account.password);
		await page.getByRole("button", { name: "Login", exact: true }).click();
		await expect(page).toHaveURL("http://127.0.0.1:3015/");
		await expect(
			page.getByRole("link", { name: "Products", exact: true }),
		).toBeVisible();
	},
);
