import { randomUUID } from "node:crypto";
import { mkdirSync, rmdirSync } from "node:fs";
import path from "node:path";
import { type Page, test as base, expect } from "playwright/test";
import { request } from "playwright/test";
import type {
	AuthTokenResponse,
	CreatedProductApiKeyResponse,
	PlatformInvitationResponse,
	ProductPermissionListResponse,
	ProductResponse,
	UserResponse,
} from "../../src/client";
import { AnchorAPI } from "./api";
import { createAPI } from "./api";
import { frontendFingerprint } from "./fingerprint";
import { captureReviewCheckpoint } from "./review";
import { owner, runtime } from "./runtime";
import { login } from "./ui";

type StorageState = Awaited<
	ReturnType<import("playwright/test").BrowserContext["storageState"]>
>;
interface Session {
	api: AnchorAPI;
	ownerAPI: AnchorAPI;
	account: { email: string; password: string };
	storageState: StorageState;
}
export interface World {
	api: AnchorAPI;
	product: ProductResponse;
	productPath: string;
	mailpitURL: string;
	smtpPort: number;
	name(prefix: string): string;
	productAPI(): Promise<AnchorAPI>;
}
export const unique = (prefix: string) =>
	`${prefix}-${randomUUID().slice(0, 8)}`;
export const test = base.extend<
	{ world: World; guestPage: Page },
	{ session: Session }
>({
	session: [
		async ({ browser }, use, workerInfo) => {
			const local = runtime();
			const publicAPI = await createAPI(local.apiURL);
			let ownerAPI: AnchorAPI | undefined;
			let api: AnchorAPI | undefined;
			let context: import("playwright/test").BrowserContext | undefined;
			let platformUserId: string | undefined;
			try {
				const frontend = await publicAPI.get<{
					runId: string;
					fingerprint: string;
				}>(`${workerInfo.project.use.baseURL}/__e2e/ready`);
				expect(
					frontend.runId,
					"frontend must belong to this disposable backend",
				).toBe(local.runId);
				expect(
					frontend.fingerprint,
					"frontend source changed: restart the managed server before reusing it",
				).toBe(frontendFingerprint());
				const ownerAccount = owner();
				const health = await publicAPI.get<{ tenant_initialized: boolean }>(
					"/health",
				);
				if (!health.tenant_initialized) {
					const lock = path.join(local.directory, "bootstrap.lock");
					let acquired = false;
					try {
						mkdirSync(lock);
						acquired = true;
					} catch (error) {
						if (
							!(
								error instanceof Error &&
								"code" in error &&
								error.code === "EEXIST"
							)
						)
							throw error;
					}
					if (acquired) {
						try {
							const latest = await publicAPI.get<{
								tenant_initialized: boolean;
							}>("/health");
							if (!latest.tenant_initialized)
								await publicAPI.post(
									"/v1/auth/register",
									{ ...ownerAccount, tenant_name: "E2E Platform" },
									200,
								);
						} finally {
							rmdirSync(lock);
						}
					} else {
						await expect
							.poll(
								async () =>
									(
										await publicAPI.get<{ tenant_initialized: boolean }>(
											"/health",
										)
									).tenant_initialized,
								{
									message:
										"another UI worker must initialize this owned runtime",
									timeout: 15_000,
								},
							)
							.toBe(true);
					}
				}
				const ownerAuth = await publicAPI.post<AuthTokenResponse>(
					"/v1/auth/login",
					ownerAccount,
					200,
				);
				ownerAPI = await createAPI(local.apiURL, ownerAuth.accessToken);
				const account = {
					email: `${unique(`worker-${workerInfo.parallelIndex}`)}@example.test`,
					password: `LocalA1!${randomUUID()}`,
				};
				const invitation = await ownerAPI.post<PlatformInvitationResponse>(
					"/v1/platform-invitations",
					{ email: account.email },
				);
				const auth = await publicAPI.post<AuthTokenResponse>(
					"/v1/auth/register",
					{ ...account, invitation_code: invitation.code },
					200,
				);
				api = await createAPI(local.apiURL, auth.accessToken);
				platformUserId = (await api.get<UserResponse>("/v1/me")).id;
				context = await browser.newContext({
					baseURL: workerInfo.project.use.baseURL,
				});
				const page = await context.newPage();
				await login(page, account);
				const storageState = await context.storageState();
				await context.close();
				await use({ api, ownerAPI, account, storageState });
			} finally {
				try {
					if (platformUserId && ownerAPI) {
						const deleted = await ownerAPI.context.delete(
							`/v1/platform-users/${platformUserId}`,
						);
						expect(
							deleted.status(),
							"worker account cleanup must delete a real platform user",
						).toBe(204);
					}
				} finally {
					await Promise.all([
						context?.close(),
						api?.context.dispose(),
						ownerAPI?.context.dispose(),
						publicAPI.context.dispose(),
					]);
				}
			}
		},
		{ scope: "worker", timeout: 60_000 },
	],
	storageState: async ({ session }, use) => use(session.storageState),
	guestPage: async (
		{
			browser,
			contextOptions,
			acceptDownloads,
			bypassCSP,
			clientCertificates,
			colorScheme,
			deviceScaleFactor,
			extraHTTPHeaders,
			geolocation,
			hasTouch,
			httpCredentials,
			ignoreHTTPSErrors,
			isMobile,
			javaScriptEnabled,
			locale,
			offline,
			permissions,
			proxy,
			viewport,
			timezoneId,
			userAgent,
			baseURL,
			serviceWorkers,
			actionTimeout,
			navigationTimeout,
		},
		use,
		testInfo,
	) => {
		const review = testInfo.project.metadata.reviewEvidence === true;
		if (review && !viewport)
			throw new Error("Guest review recording requires a configured viewport");
		const context = await browser.newContext({
			...contextOptions,
			acceptDownloads,
			bypassCSP,
			clientCertificates,
			colorScheme,
			deviceScaleFactor,
			extraHTTPHeaders,
			geolocation,
			hasTouch,
			httpCredentials,
			ignoreHTTPSErrors,
			isMobile,
			javaScriptEnabled,
			locale,
			offline,
			permissions,
			proxy,
			viewport,
			timezoneId,
			userAgent,
			baseURL,
			serviceWorkers,
			storageState: { cookies: [], origins: [] },
			recordVideo:
				review && viewport
					? { dir: testInfo.outputPath("guest-video"), size: viewport }
					: undefined,
		});
		context.setDefaultTimeout(actionTimeout);
		context.setDefaultNavigationTimeout(navigationTimeout);
		let video: ReturnType<Page["video"]> = null;
		try {
			const page = await context.newPage();
			video = page.video();
			const errors: string[] = [];
			page.on("pageerror", (error) => errors.push(error.message));
			await use(page);
			expect(errors, "uncaught guest application errors").toEqual([]);
		} finally {
			await context.close();
			if (video)
				await testInfo.attach("guest-video", {
					path: await video.path(),
					contentType: "video/webm",
				});
		}
	},
	world: async ({ session }, use) => {
		const product = await session.api.post<ProductResponse>("/v1/products", {
			name: unique("e2e-product"),
			description: "Disposable browser fixture",
		});
		const local = runtime();
		const productPath = `/v1/products/${product.id}`;
		let tenantAPI: Promise<AnchorAPI> | undefined;
		try {
			await use({
				api: session.api,
				product,
				productPath,
				mailpitURL: local.mailpitURL,
				smtpPort: local.smtpPort,
				name: unique,
				productAPI: () => {
					if (!tenantAPI)
						tenantAPI = (async () => {
							const permissions =
								await session.api.post<ProductPermissionListResponse>(
									`${productPath}/permissions/search`,
									{ pagination: { limit: 1000, offset: 0 } },
									200,
								);
							const key = await session.api.post<CreatedProductApiKeyResponse>(
								`${productPath}/api-keys`,
								{
									name: unique("fixture-key"),
									permissions: permissions.items.map(
										(permission) => permission.name,
									),
								},
							);
							return new AnchorAPI(
								await request.newContext({
									baseURL: local.apiURL,
									extraHTTPHeaders: { "X-Product-API-Key": key.value },
								}),
							);
						})();
					return tenantAPI;
				},
			});
		} finally {
			try {
				const resolved = await tenantAPI?.catch(() => undefined);
				await resolved?.context.dispose();
			} finally {
				const response = await session.api.context.get(
					`/v1/products/${product.id}`,
				);
				if (response.status() === 200) {
					const current = (await response.json()) as ProductResponse;
					if (current.config.protected)
						await session.api.put(`/v1/products/${product.id}`, {
							name: current.name,
							description: current.description,
							config: { protected: false },
						});
				}
				await session.api.remove(`/v1/products/${product.id}`);
			}
		}
	},
	page: async ({ page }, use, testInfo) => {
		const errors: string[] = [];
		page.on("pageerror", (error) => errors.push(error.message));
		await use(page);
		expect(errors, "uncaught application errors").toEqual([]);
		if (testInfo.status === "passed")
			await captureReviewCheckpoint(page, testInfo, "final-state");
	},
});
export { expect };
