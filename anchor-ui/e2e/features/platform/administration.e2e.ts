import type {
	AuthTokenResponse,
	PlatformInvitationResponse,
	UserResponse,
} from "../../../src/client";
import { createAPI } from "../../support/api";
import { expect, test, unique } from "../../support/fixtures";
import { owner, runtime } from "../../support/runtime";
import { bulkDelete, searchTable } from "../../support/ui";

// Covers: PLATFORM_INVITATIONS, REGISTER
test(
	"create an invitation, copy its link, validate signup and automatically sign in",
	{ tag: "@platform" },
	async ({ page, browser, session }) => {
		const email = `${unique("browser-invite")}@example.test`;
		let invitation: PlatformInvitationResponse | undefined;
		let registeredID: string | undefined;
		const publicAPI = await createAPI(runtime().apiURL);
		const context = await browser.newContext({
			baseURL: "http://127.0.0.1:3015",
			permissions: ["clipboard-read", "clipboard-write"],
		});
		try {
			await page.goto("/platform/users/invitations");
			await page
				.getByRole("button", { name: "Add Invitation", exact: true })
				.click();
			const dialog = page.getByRole("dialog");
			await dialog.getByLabel("Email", { exact: true }).fill("invalid");
			await expect(
				dialog.getByRole("button", { name: "Send Invitation", exact: true }),
			).toBeDisabled();
			await dialog.getByLabel("Email", { exact: true }).fill(email);
			const response = page.waitForResponse(
				(r) =>
					r.request().method() === "POST" &&
					new URL(r.url()).pathname === "/v1/platform-invitations",
			);
			await dialog
				.getByRole("button", { name: "Send Invitation", exact: true })
				.click();
			const created = await response;
			expect(created.status()).toBe(201);
			invitation = (await created.json()) as PlatformInvitationResponse;
			const link = await dialog.getByRole("textbox").inputValue();
			expect(new URL(link).searchParams.get("invitationCode")).toBe(
				invitation.code,
			);
			await page
				.context()
				.grantPermissions(["clipboard-read", "clipboard-write"]);
			await dialog.getByRole("button", { name: "Copy", exact: true }).click();
			expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
				link,
			);
			const guest = await context.newPage();
			await guest.goto(link);
			await expect(guest.getByLabel("Email", { exact: true })).toHaveValue(
				email,
			);
			await expect(guest.getByLabel("Email", { exact: true })).toBeDisabled();
			await guest
				.getByLabel("Password", { exact: true })
				.fill("LocalA1!invite-password");
			await guest
				.getByLabel("Confirm Password", { exact: true })
				.fill("mismatch");
			await expect(
				guest.getByText("Passwords do not match", { exact: true }),
			).toBeVisible();
			await guest
				.getByLabel("Confirm Password", { exact: true })
				.fill("LocalA1!invite-password");
			const registration = guest.waitForResponse(
				(r) =>
					new URL(r.url()).pathname === "/v1/auth/register" &&
					r.request().method() === "POST",
			);
			await guest
				.getByRole("button", { name: "Create account", exact: true })
				.click();
			const result = await registration;
			expect(result.status()).toBe(200);
			const auth = (await result.json()) as AuthTokenResponse;
			const registeredAPI = await createAPI(runtime().apiURL, auth.accessToken);
			registeredID = (await registeredAPI.get<UserResponse>("/v1/me")).id;
			await registeredAPI.context.dispose();
			await expect(
				guest.getByRole("link", { name: "Products", exact: true }),
			).toBeVisible();
			await guest.reload();
			await expect(
				guest.getByRole("button").filter({ hasText: email }),
			).toBeVisible();
		} finally {
			if (registeredID)
				await session.ownerAPI.remove(`/v1/platform-users/${registeredID}`);
			if (invitation)
				await session.ownerAPI.remove(
					`/v1/platform-invitations/${invitation.id}`,
				);
			await context.close();
			await publicAPI.context.dispose();
		}
	},
);

test(
	"invitation search, sorting, paging, single delete and bulk delete are isolated",
	{ tag: "@platform" },
	async ({ page, session }) => {
		const prefix = unique("invite-table");
		const invitations: PlatformInvitationResponse[] = [];
		try {
			for (let i = 0; i < 12; i++)
				invitations.push(
					await session.ownerAPI.post<PlatformInvitationResponse>(
						"/v1/platform-invitations",
						{ email: `${prefix}-${String(i).padStart(2, "0")}@example.test` },
					),
				);
			await page.goto("/platform/users/invitations");
			await searchTable(
				page,
				"Search invitations",
				"/v1/platform-invitations/search",
				prefix,
			);
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
				.getByRole("columnheader", { name: "Email", exact: true })
				.getByRole("button")
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: prefix }).first(),
			).toContainText(invitations[0].email);
			await page
				.getByRole("row")
				.filter({ hasText: invitations[0].email })
				.getByRole("button", { name: "Delete invitation", exact: true })
				.click();
			await page
				.getByRole("alertdialog")
				.getByRole("button", { name: "Delete", exact: true })
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: invitations[0].email }),
			).toHaveCount(0);
			for (const invitation of invitations.slice(1, 3))
				await page
					.getByRole("row")
					.filter({ hasText: invitation.email })
					.getByRole("checkbox", { name: "Select row" })
					.check();
			await bulkDelete(page, 2);
			await expect(
				page.getByRole("row").filter({ hasText: invitations[3].email }),
			).toBeVisible();
		} finally {
			await Promise.all(
				invitations.map((invitation) =>
					session.ownerAPI.remove(`/v1/platform-invitations/${invitation.id}`),
				),
			);
		}
	},
);

// Covers: PLATFORM_USERS
test(
	"platform users paginate, protect the owner and delete only selected disposable admins",
	{ tag: "@platform" },
	async ({ page, session }) => {
		const api = await createAPI(runtime().apiURL);
		const prefix = unique("admin-table");
		const users: UserResponse[] = [];
		try {
			for (let i = 0; i < 12; i++) {
				const email = `${prefix}-${String(i).padStart(2, "0")}@example.test`;
				const invitation =
					await session.ownerAPI.post<PlatformInvitationResponse>(
						"/v1/platform-invitations",
						{ email },
					);
				const auth = await api.post<AuthTokenResponse>(
					"/v1/auth/register",
					{
						email,
						password: "LocalA1!admin-password",
						invitation_code: invitation.code,
					},
					200,
				);
				const userAPI = await createAPI(runtime().apiURL, auth.accessToken);
				users.push(await userAPI.get<UserResponse>("/v1/me"));
				await userAPI.context.dispose();
			}
			await page.goto("/platform/users");
			await searchTable(
				page,
				"Search users",
				"/v1/platform-users/search",
				owner().email,
			);
			await expect(
				page
					.getByRole("row")
					.filter({ hasText: owner().email })
					.getByRole("button", { name: "Delete Platform User", exact: true }),
			).toBeDisabled();
			await searchTable(
				page,
				"Search users",
				"/v1/platform-users/search",
				prefix,
			);
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
				.getByRole("columnheader", { name: "Email", exact: true })
				.getByRole("button")
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: prefix }).first(),
			).toContainText(users[0].email);
			await page
				.getByRole("row")
				.filter({ hasText: users[0].email })
				.getByRole("button", { name: "Delete Platform User", exact: true })
				.click();
			await page
				.getByRole("dialog")
				.getByRole("button", { name: "Delete Platform User", exact: true })
				.click();
			await expect(
				page.getByRole("row").filter({ hasText: users[0].email }),
			).toHaveCount(0);
			for (const user of users.slice(1, 3))
				await page
					.getByRole("row")
					.filter({ hasText: user.email })
					.getByRole("checkbox", { name: "Select row" })
					.check();
			await bulkDelete(page, 2);
			await expect(
				page.getByRole("row").filter({ hasText: prefix }),
			).toHaveCount(9);
			await expect(
				page.getByRole("row").filter({ hasText: users[3].email }),
			).toBeVisible();
		} finally {
			await Promise.all(
				users.map((user) =>
					session.ownerAPI.remove(`/v1/platform-users/${user.id}`),
				),
			);
			await api.context.dispose();
		}
	},
);
