import type { Page } from "playwright/test";
import {
	type IntegrationInstanceResponse,
	IntegrationInstanceStatus,
	IntegrationProviderType,
	type StripeBillingErrorResponse,
	StripeBillingFraudRefundCurrency,
	StripeIntegrationAuthMethod,
	zStripeIntegrationConfigWritable,
	zStripeIntegrationPublicConfig,
} from "../../../src/client";
import {
	zCheckoutRequest,
	zCreatePriceRequest,
	zSubscriptionRequest,
	zUpdateSettingsRequest,
} from "../../../src/features/billing/billing-types";
import type { State } from "../../../src/features/billing/billing-types";
import {
	createFixtureAPI,
	fixtureState,
	fraudEnabledFixtureState,
	worstCaseFixtureState,
} from "../../../src/features/billing/fixtures";
import { expect, test } from "../../support/fixtures";
import { captureReviewCheckpoint } from "../../support/review";
import { selectProduct } from "../../support/ui";
import {
	chooseOption,
	createOrganization,
	declareSchema,
} from "../licensing/helpers";

async function fixtureBilling(
	page: Page,
	state: State,
	beforeLoad?: () => Promise<void>,
	requestProductId = state.product.id,
) {
	const api = createFixtureAPI(state);
	const prefix = `/v1/products/${requestProductId}/billing/stripe`;
	await page.route(new RegExp(`${prefix}(?:/.*)?$`), async (route) => {
		const request = route.request();
		const path = new URL(request.url()).pathname.slice(prefix.length);
		let body: unknown;
		let status = 200;
		if (!path) {
			try {
				await beforeLoad?.();
				body = await api.load();
			} catch (error) {
				status = 409;
				body = {
					error: error instanceof Error ? error.message : "Billing unavailable",
				} satisfies StripeBillingErrorResponse;
			}
		} else if (path === "/prices") {
			body = await api.createPrice(
				zCreatePriceRequest.parse(request.postDataJSON()),
			);
			status = 201;
		} else if (path.endsWith("/archive"))
			body = await api.archivePrice(path.split("/")[2]);
		else if (path === "/settings") {
			const parsed = zUpdateSettingsRequest.parse(request.postDataJSON());
			const { fraud_refund_policy: policy, ...settings } = parsed;
			body = await api.updateSettings({
				...settings,
				...(policy
					? {
							fraud_refund_policy: {
								...policy,
								currency: {
									usd: StripeBillingFraudRefundCurrency.USD,
									cad: StripeBillingFraudRefundCurrency.CAD,
									eur: StripeBillingFraudRefundCurrency.EUR,
								}[policy.currency],
							},
						}
					: {}),
			});
		} else {
			const organizationId = path.split("/")[2];
			const operation = path.split("/")[3];
			if (operation === "checkout")
				body = {
					url: await api.checkout(
						organizationId,
						zCheckoutRequest.parse(request.postDataJSON()),
					),
				};
			else if (operation === "subscription")
				body = await api.changeSubscription(
					organizationId,
					zSubscriptionRequest.parse(request.postDataJSON()),
				);
			else if (operation === "cancellation")
				body = request.postDataJSON().cancel_at_period_end
					? await api.cancel(organizationId)
					: await api.resume(organizationId);
			else if (operation === "portal")
				body = { url: await api.portal(organizationId) };
			else if (operation === "sync") body = await api.sync(organizationId);
			else throw new Error(`Unexpected billing operation: ${operation}`);
		}
		await route.fulfill({ status, json: body });
	});
	return api;
}

async function fixtureStripeConnection(page: Page, productId: string) {
	const prefix = `/v1/products/${productId}/integrations`;
	await page.route(new RegExp(`${prefix}(?:/.*)?$`), async (route) => {
		if (new URL(route.request().url()).pathname.endsWith("/audit-logs"))
			return route.fulfill({ json: { items: [], count: 0, total: 0 } });
		const timestamp = "2026-10-08T14:00:00Z";
		const instance: IntegrationInstanceResponse = {
			id: "iin_fraud_contractfixture",
			product_id: productId,
			provider_type: IntegrationProviderType.STRIPE,
			config_version: 1,
			is_enabled: true,
			status: IntegrationInstanceStatus.ACTIVE,
			created_at: timestamp,
			updated_at: timestamp,
			public_config: {
				auth_method: StripeIntegrationAuthMethod.API_KEY,
				account_id: "acct_fraud_contractfixture",
				return_url: "https://anchor.example.com",
				mode: "sandbox",
				api_key_configured: true,
				webhook_secret_configured: true,
			},
		};
		await route.fulfill({ json: { items: [instance], count: 1 } });
	});
}

// Covers: PRODUCT_INTEGRATION_STRIPE, PRODUCT_PRICING, ORGANIZATION_LICENSE_DETAIL
test("fraud refund policy validates, persists, preserves fallback and presents contract fixture outcomes", async ({
	page,
	world,
}, testInfo) => {
	const state = structuredClone(fraudEnabledFixtureState);
	state.product = { id: world.product.id, name: world.product.name };
	state.settings.fraud_refund_policy.enabled = false;
	state.settings.fraud_refund_policy.max_amount = 0;
	await fixtureStripeConnection(page, world.product.id);
	const api = await fixtureBilling(page, state);
	await selectProduct(page, world.product);
	const integrationURL = `/platform/${world.product.id}/integration-stripe`;
	await page.goto(integrationURL);
	const enabled = page.getByRole("switch", {
		name: "Automatically refund fraud warnings",
		exact: true,
	});
	const amount = page.getByLabel("Maximum payment amount (minor units)", {
		exact: true,
	});
	await expect(enabled).not.toBeChecked();
	await enabled.click();
	await amount.fill("0");
	await page
		.getByRole("button", { name: "Save refund policy", exact: true })
		.click();
	await expect(enabled).toBeChecked();
	await expect((await api.load()).settings.fraud_refund_policy.enabled).toBe(
		false,
	);
	await expect(page.getByText(/Enter a whole amount/).first()).toBeVisible();
	await amount.fill("1500");
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "Refund currency", exact: true }),
		"CAD",
	);
	await page
		.getByRole("button", { name: "Save refund policy", exact: true })
		.click();
	await expect(
		page.getByText("Refund policy saved.", { exact: true }),
	).toBeVisible();
	await page.reload();
	await expect(enabled).toBeChecked();
	await expect(amount).toHaveValue("1500");
	await expect(
		page.getByRole("combobox", { name: "Refund currency", exact: true }),
	).toContainText("CAD");
	expect((await api.load()).settings.fallback_template_id).toBe("tpl_free");
	const activity = page.getByRole("list", {
		name: "Fraud refund activity",
		exact: true,
	});
	for (const outcome of ["Refunded", "Skipped", "Pending", "Failed"])
		await expect(
			activity.getByText(outcome, { exact: true }).first(),
		).toBeVisible();
	await page
		.getByRole("heading", { name: "Fraud warning refunds", exact: true })
		.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "fraud-refund-policy-saved");
	await activity.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "fraud-refund-activity");
	await page.goto("/products/pricing");
	await chooseOption(
		page,
		page.getByRole("combobox", {
			name: "Fallback license template",
			exact: true,
		}),
		"Enterprise",
	);
	await page
		.getByRole("button", { name: "Save fallback", exact: true })
		.click();
	await expect(
		page.getByText("Fallback template saved.", { exact: true }),
	).toBeVisible();
	await page.goto(integrationURL);
	await expect(enabled).toBeChecked();
	await expect(amount).toHaveValue("1500");
	expect((await api.load()).settings.fallback_template_id).toBe(
		"tpl_enterprise",
	);
	await enabled.click();
	await page
		.getByRole("button", { name: "Save refund policy", exact: true })
		.click();
	await expect(
		page.getByText("Refund policy saved.", { exact: true }),
	).toBeVisible();
	await page.reload();
	await expect(enabled).not.toBeChecked();
	await expect(
		activity.getByText("Refunded", { exact: true }).first(),
	).toBeVisible();
	await page
		.getByRole("heading", { name: "Fraud warning refunds", exact: true })
		.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(page, testInfo, "fraud-refund-policy-disabled");
});

// Covers: PRODUCT_INTEGRATION_STRIPE
test("fraud refund draft survives held refresh and unavailable billing disables saving", async ({
	page,
	world,
}, testInfo) => {
	const state = structuredClone(fraudEnabledFixtureState);
	state.product = { id: world.product.id, name: world.product.name };
	let releaseRefresh = () => {};
	let refreshRequested = () => {};
	const refreshGate = new Promise<void>((resolve) => {
		releaseRefresh = resolve;
	});
	const refreshRequest = new Promise<void>((resolve) => {
		refreshRequested = resolve;
	});
	let loads = 0;
	let unavailable = false;
	await fixtureStripeConnection(page, world.product.id);
	await fixtureBilling(page, state, async () => {
		if (unavailable) throw new Error("Enable Stripe before managing billing.");
		loads += 1;
		if (loads === 2) {
			refreshRequested();
			await refreshGate;
		}
	});
	await selectProduct(page, world.product);
	await page.goto(`/platform/${world.product.id}/integration-stripe`);
	const amount = page.getByLabel("Maximum payment amount (minor units)", {
		exact: true,
	});
	await amount.fill("2500");
	const reload = page.getByRole("button", {
		name: "Reload refund policy",
		exact: true,
	});
	await reload.click();
	await refreshRequest;
	await expect(amount).toHaveValue("2500");
	const readBack = page.waitForResponse(
		(response) =>
			new URL(response.url()).pathname ===
			`${world.productPath}/billing/stripe`,
	);
	releaseRefresh();
	await readBack;
	await expect(amount).toHaveValue("2500");
	unavailable = true;
	await reload.click();
	await expect(
		page.getByText("Enable Stripe before managing billing.", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Save refund policy", exact: true }),
	).toBeDisabled();
	await expect(reload).toBeEnabled();
	await expect(amount).toHaveValue("2500");
	await page
		.getByRole("heading", { name: "Fraud warning refunds", exact: true })
		.scrollIntoViewIfNeeded();
	await captureReviewCheckpoint(
		page,
		testInfo,
		"fraud-refund-policy-unavailable",
	);
	unavailable = false;
	await reload.click();
	await expect(
		page.getByRole("button", { name: "Save refund policy", exact: true }),
	).toBeEnabled();
	await expect(amount).toHaveValue("2500");
});

// Covers: PRODUCT_PRICING, ORGANIZATION_LICENSE_DETAIL
test("native pricing and organization billing preserve edits and show contract fixture subscription changes", async ({
	page,
	world,
}, testInfo) => {
	await declareSchema(world);
	const organization = await createOrganization(
		world,
		undefined,
		"billing-customer",
	);
	const state = structuredClone(fixtureState);
	state.product = { id: world.product.id, name: world.product.name };
	state.organizations[0].id = organization.id;
	state.organizations[0].name = organization.name;
	state.events[0].organization_id = organization.id;
	let paused = false;
	await fixtureBilling(page, state, async () => {
		if (paused) throw new Error("Enable Stripe before managing billing.");
	});
	await selectProduct(page, world.product);
	await page.goto("/products/pricing");
	await expect(
		page.getByRole("heading", { name: "Pricing", exact: true }),
	).toBeVisible();
	await expect(page.getByRole("button", { name: /Working on:/ })).toContainText(
		world.product.name,
	);
	await page.getByRole("button", { name: "Create price", exact: true }).click();
	await expect(
		page.getByText("Enter a whole amount between 1 and 99,999,999 cents.", {
			exact: true,
		}),
	).toBeVisible();
	await page.getByLabel("Price name", { exact: true }).fill("Business monthly");
	await page.getByLabel("Amount in cents", { exact: true }).fill("7900");
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "License template", exact: true }),
		"Enterprise",
	);
	await page
		.getByRole("button", { name: "Reload billing", exact: true })
		.click();
	await expect(page.getByLabel("Price name", { exact: true })).toHaveValue(
		"Business monthly",
	);
	await expect(
		page.getByRole("combobox", { name: "License template", exact: true }),
	).toContainText("Enterprise");
	await page.getByRole("button", { name: "Create price", exact: true }).click();
	await expect(
		page.getByRole("heading", { name: "Business monthly", exact: true }),
	).toBeVisible();
	await page.reload();
	await expect(
		page.getByRole("heading", { name: "Business monthly", exact: true }),
	).toBeVisible();
	await captureReviewCheckpoint(page, testInfo, "native-pricing");
	await page.goto(`/organizations/license/${organization.id}/usage`);
	await expect(
		page.getByText("This organization has no license yet.", { exact: false }),
	).toBeVisible();
	await page.getByRole("link", { name: "Billing", exact: true }).click();
	await expect(page).toHaveURL(
		new RegExp(`/organizations/license/${organization.id}/billing$`),
	);
	await expect(
		page.getByRole("heading", { name: organization.name, exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("link", { name: "Usage", exact: true }),
	).toBeVisible();
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "Recurring price", exact: true }),
		"Business monthly · USD 79.00 / month",
	);
	await page.getByRole("button", { name: "Change price", exact: true }).click();
	await expect(
		page
			.getByText("Business monthly · USD 79.00 / month", { exact: true })
			.first(),
	).toBeVisible();
	await expect(
		page.getByRole("heading", { name: "Enterprise", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Cancel at period end", exact: true })
		.click();
	await expect(
		page.getByText("Cancellation scheduled", { exact: true }),
	).toBeVisible();
	await page.reload();
	await expect(
		page.getByRole("button", { name: "Resume subscription", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Resume subscription", exact: true })
		.click();
	await expect(
		page.getByText("Cancellation scheduled", { exact: true }),
	).toHaveCount(0);
	await page
		.getByRole("button", { name: "Customer portal", exact: true })
		.click();
	await expect(
		page.getByRole("link", {
			name: "Continue to customer portal",
			exact: false,
		}),
	).toHaveAttribute("href", /^https:\/\/billing\.stripe\.com\//);
	await captureReviewCheckpoint(page, testInfo, "native-organization-billing");
	paused = true;
	await page
		.getByRole("button", { name: "Reload billing", exact: true })
		.click();
	await expect(
		page.getByText("Enable Stripe before managing billing.", { exact: true }),
	).toBeVisible();
	for (const name of ["Reconcile", "Customer portal", "Cancel at period end"]) {
		await expect(
			page.getByRole("button", { name, exact: true }),
		).toBeDisabled();
	}
	await expect(
		page.getByRole("button", { name: "Reload billing", exact: true }),
	).toBeEnabled();
	paused = false;
	await page.getByRole("button", { name: "Try again", exact: true }).click();
	await expect(
		page.getByRole("button", { name: "Customer portal", exact: true }),
	).toBeEnabled();
	await page.goto("/products/pricing");
	await page
		.getByRole("button", { name: "Archive Business monthly", exact: true })
		.click();
	await expect(
		page.getByRole("button", { name: "Archive Business monthly", exact: true }),
	).toHaveCount(0);
	await page.reload();
	await expect(
		page.getByRole("heading", { name: "Business monthly", exact: true }),
	).toBeVisible();
});

// Covers: PRODUCT_PRICING
test("billing waits for initial state and preserves a draft through a held background refresh", async ({
	page,
	world,
}) => {
	const state = structuredClone(fixtureState);
	state.product = { id: world.product.id, name: world.product.name };
	let releaseInitial: () => void = () => {};
	let releaseRefresh: () => void = () => {};
	let initialRequested: () => void = () => {};
	let refreshRequested: () => void = () => {};
	const initialGate = new Promise<void>((resolve) => {
		releaseInitial = resolve;
	});
	const refreshGate = new Promise<void>((resolve) => {
		releaseRefresh = resolve;
	});
	const initialRequest = new Promise<void>((resolve) => {
		initialRequested = resolve;
	});
	const refreshRequest = new Promise<void>((resolve) => {
		refreshRequested = resolve;
	});
	let loads = 0;
	let paused = false;
	await fixtureBilling(page, state, async () => {
		if (paused) throw new Error("Enable Stripe before managing billing.");
		loads += 1;
		if (loads === 1) {
			initialRequested();
			await initialGate;
		}
		if (loads === 2) {
			refreshRequested();
			await refreshGate;
		}
	});
	await selectProduct(page, world.product);
	await page.goto("/products/pricing");
	await initialRequest;
	await expect(
		page.getByRole("button", { name: "Create price", exact: true }),
	).toHaveCount(0);
	releaseInitial();
	await page
		.getByLabel("Price name", { exact: true })
		.fill("An unfinished draft");
	await refreshRequest;
	await expect(page.getByLabel("Price name", { exact: true })).toBeEnabled();
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "License template", exact: true }),
		"Enterprise",
	);
	const refreshResponse = page.waitForResponse(
		(response) =>
			new URL(response.url()).pathname ===
			`${world.productPath}/billing/stripe`,
	);
	releaseRefresh();
	await refreshResponse;
	await expect(page.getByLabel("Price name", { exact: true })).toHaveValue(
		"An unfinished draft",
	);
	await expect(
		page.getByRole("combobox", { name: "License template", exact: true }),
	).toContainText("Enterprise");
	await chooseOption(
		page,
		page.getByRole("combobox", {
			name: "Fallback license template",
			exact: true,
		}),
		"Enterprise",
	);
	await expect(
		page.getByRole("button", { name: "Save fallback", exact: true }),
	).toBeEnabled();
	paused = true;
	await page
		.getByRole("button", { name: "Reload billing", exact: true })
		.click();
	await expect(
		page.getByText("Enable Stripe before managing billing.", { exact: true }),
	).toBeVisible();
	for (const name of ["Create price", "Archive Pro monthly", "Save fallback"]) {
		await expect(
			page.getByRole("button", { name, exact: true }),
		).toBeDisabled();
	}
	await expect(
		page.getByRole("button", { name: "Create price", exact: true }),
	).not.toHaveAttribute("aria-busy", "true");
	await expect(
		page.getByRole("heading", { name: "Pro monthly", exact: true }),
	).toBeVisible();
	await expect(page.getByLabel("Price name", { exact: true })).toHaveValue(
		"An unfinished draft",
	);
	await expect(
		page.getByRole("button", { name: "Reload billing", exact: true }),
	).toBeEnabled();
	paused = false;
	await page.getByRole("button", { name: "Try again", exact: true }).click();
	await expect(
		page.getByRole("button", { name: "Create price", exact: true }),
	).toBeEnabled();
	await expect(
		page.getByRole("button", { name: "Save fallback", exact: true }),
	).toBeEnabled();
	await expect(page.getByLabel("Price name", { exact: true })).toHaveValue(
		"An unfinished draft",
	);
});

// Covers: PRODUCT_INTEGRATION_STRIPE, PRODUCT_INTEGRATIONS
test("Stripe connects through the native Integration Hub with write-only contract fixture secrets and pause persistence", async ({
	page,
	world,
}, testInfo) => {
	let instance: IntegrationInstanceResponse | undefined;
	const prefix = `${world.productPath}/integrations`;
	await page.route(new RegExp(`${prefix}(?:/.*)?$`), async (route) => {
		const request = route.request();
		const path = new URL(request.url()).pathname;
		if (path.endsWith("/audit-logs"))
			return route.fulfill({ json: { items: [], count: 0, total: 0 } });
		if (request.method() === "GET")
			return route.fulfill({
				json: { items: instance ? [instance] : [], count: instance ? 1 : 0 },
			});
		if (request.method() === "DELETE") {
			instance = undefined;
			return route.fulfill({ status: 204 });
		}
		const body = request.postDataJSON();
		const config = zStripeIntegrationConfigWritable.parse(body.config);
		const previous = zStripeIntegrationPublicConfig.safeParse(
			instance?.public_config,
		);
		const timestamp = new Date().toISOString();
		instance = {
			id: "iin_contractfixture",
			product_id: world.product.id,
			provider_type: IntegrationProviderType.STRIPE,
			config_version: 1,
			is_enabled: body.is_enabled ?? true,
			status:
				body.is_enabled === false
					? IntegrationInstanceStatus.INACTIVE
					: IntegrationInstanceStatus.ACTIVE,
			created_at: timestamp,
			updated_at: timestamp,
			public_config: {
				auth_method:
					config.auth_method === "LOCAL_CLI"
						? StripeIntegrationAuthMethod.LOCAL_CLI
						: StripeIntegrationAuthMethod.API_KEY,
				account_id: config.account_id,
				return_url: config.return_url,
				mode: "sandbox",
				api_key_configured:
					!!config.api_key ||
					(previous.success && previous.data.api_key_configured),
				webhook_secret_configured:
					!!config.webhook_secret ||
					(previous.success && previous.data.webhook_secret_configured),
			},
		};
		await route.fulfill({
			status: request.method() === "POST" ? 201 : 200,
			json: instance,
		});
	});
	await selectProduct(page, world.product);
	await page.goto(`/platform/${world.product.id}/integrations`);
	await page
		.getByRole("link", { name: "Configure Stripe", exact: true })
		.click();
	await expect(
		page.getByRole("heading", { name: "Stripe", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Connect Stripe", exact: true })
		.click();
	await expect(
		page.getByText("Enter the Stripe sandbox account ID.", { exact: true }),
	).toBeVisible();
	await chooseOption(
		page,
		page.getByRole("combobox", { name: "Connection method", exact: true }),
		"Connected Stripe CLI (local sandbox)",
	);
	await expect(
		page.getByLabel("Stripe test API key", { exact: true }),
	).toHaveCount(0);
	await page
		.getByLabel("Stripe account ID", { exact: true })
		.fill("acct_contractfixture");
	await page
		.getByLabel("Webhook signing secret", { exact: true })
		.fill("whsec_contractfixture");
	await page
		.getByRole("button", { name: "Connect Stripe", exact: true })
		.click();
	await expect(
		page.getByText("Stripe connection saved.", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByLabel("Webhook signing secret", { exact: true }),
	).toHaveValue("");
	await expect(
		page.getByText("Signing secret saved", { exact: true }),
	).toBeVisible();
	await page.getByRole("switch", { name: "Enabled", exact: true }).uncheck();
	await page
		.getByRole("button", { name: "Save Stripe settings", exact: true })
		.click();
	await page.reload();
	await expect(
		page.getByRole("switch", { name: "Enabled", exact: true }),
	).not.toBeChecked();
	await expect(
		page.getByText("Signing secret saved", { exact: true }),
	).toBeVisible();
	await page.getByRole("switch", { name: "Enabled", exact: true }).check();
	await page
		.getByRole("button", { name: "Save Stripe settings", exact: true })
		.click();
	await captureReviewCheckpoint(page, testInfo, "native-stripe-integration");
	await page
		.getByRole("link", { name: "Integration Hub", exact: true })
		.click();
	await expect(
		page.getByRole("link", { name: "Open Stripe details", exact: true }),
	).toBeVisible();
	for (const name of ["Configured", "Live"]) {
		await expect(
			page
				.getByText(name, { exact: true })
				.first()
				.evaluate((element) => element.scrollWidth <= element.clientWidth),
		).resolves.toBe(true);
	}
	await captureReviewCheckpoint(page, testInfo, "native-integration-hub");
});

// Covers: PRODUCT_PRICING
test("native billing handles missing setup, wrong product scope and worst-case contract fixture data", async ({
	page,
	world,
}, testInfo) => {
	await selectProduct(page, world.product);
	await page.goto("/products/pricing");
	await expect(
		page.getByRole("alert").getByText("Couldn’t load billing", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("link", { name: "Stripe integration", exact: true }),
	).toHaveAttribute("href", `/platform/${world.product.id}/integration-stripe`);
	const state = structuredClone(worstCaseFixtureState);
	await fixtureBilling(page, state, undefined, world.product.id);
	await page.reload();
	await expect(
		page
			.getByRole("alert")
			.getByText("Billing state does not match this product", { exact: true }),
	).toBeVisible();
	await expect(
		page.getByRole("button", { name: "Create price", exact: true }),
	).toHaveCount(0);
	state.product = { id: world.product.id, name: world.product.name };
	await fixtureBilling(page, state);
	await page.reload();
	await expect(
		page.getByRole("list", { name: "Pricing catalog", exact: true }),
	).toBeVisible();
	await expect(page.getByText("Archived", { exact: true })).toBeVisible();
	const viewportWidth = page.viewportSize()?.width;
	expect(viewportWidth).toBeDefined();
	expect(
		await page.evaluate(() => document.documentElement.scrollWidth),
	).toBeLessThanOrEqual(viewportWidth ?? 0);
	await captureReviewCheckpoint(page, testInfo, "native-billing-worst-case");
});
