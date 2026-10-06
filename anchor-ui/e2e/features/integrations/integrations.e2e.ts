import type { ProductResponse } from "../../../src/client";
import { expect, test } from "../../support/fixtures";
import { selectProduct } from "../../support/ui";
import {
	eventReceiver,
	fillSMTP,
	integrations,
	signature,
	webhookSecret,
} from "./helpers";

// Covers: PLATFORM_INTEGRATIONS, INTEGRATION_CLERK, PRODUCT_INTEGRATIONS
test("legacy integration routes redirect to the selected product", async ({
	page,
	world,
}) => {
	await selectProduct(page, world.product);
	await page.goto("/platform/integrations");
	await expect(page).toHaveURL(`/platform/${world.product.id}/integrations`);
	await expect(page.getByText("SMTP Email", { exact: true })).toBeVisible();
	await page.goto("/platform/integration-clerk");
	await expect(page).toHaveURL(
		`/platform/${world.product.id}/integration-clerk`,
	);
	await expect(
		page.getByRole("button", { name: "Create Clerk Instance", exact: true }),
	).toBeVisible();
});

// Covers: PRODUCT_INTEGRATION_CLERK
test("Clerk setup preserves its secret through pause, reset, resume and signed ingestion", async ({
	page,
	world,
}) => {
	await selectProduct(page, world.product);
	await page.goto(`/platform/${world.product.id}/integration-clerk`);
	await page
		.getByRole("button", { name: "Create Clerk Instance", exact: true })
		.click();
	const secret = webhookSecret();
	await page.getByLabel("Webhook Secret", { exact: true }).fill(secret);
	await page
		.getByRole("button", { name: "Update Configuration", exact: true })
		.click();
	await expect(
		page.getByRole("button", { name: "Update Configuration", exact: true }),
	).toBeDisabled();
	await expect(page.getByLabel("Clerk API Key", { exact: true })).toHaveValue(
		"",
	);
	await page.getByRole("switch", { name: "Enabled", exact: true }).uncheck();
	await page.getByRole("button", { name: "Reset", exact: true }).click();
	await expect(
		page.getByRole("switch", { name: "Enabled", exact: true }),
	).toBeChecked();
	await page.getByRole("switch", { name: "Enabled", exact: true }).uncheck();
	await page
		.getByRole("button", { name: "Update Configuration", exact: true })
		.click();
	await expect
		.poll(async () => (await integrations(world))[0]?.is_enabled)
		.toBe(false);
	await page.reload();
	await expect(
		page.getByRole("switch", { name: "Enabled", exact: true }),
	).not.toBeChecked();
	await page.getByRole("switch", { name: "Enabled", exact: true }).check();
	await page
		.getByRole("button", { name: "Update Configuration", exact: true })
		.click();
	await expect
		.poll(async () => (await integrations(world))[0]?.status)
		.toBe("ACTIVE");
	const email = `${world.name("clerk-user")}@example.test`;
	const productClient = await world.productAPI();
	const body = JSON.stringify({
		type: "user.created",
		object: "event",
		data: {
			id: world.name("user"),
			object: "user",
			first_name: "Clerk",
			last_name: "Browser",
			primary_email_address_id: "email_1",
			email_addresses: [{ id: "email_1", email_address: email }],
		},
	});
	const id = world.name("msg");
	const timestamp = String(Math.floor(Date.now() / 1000));
	const invalid = await world.api.context.post(
		`${world.productPath}/integrations/webhooks/CLERK`,
		{
			data: body,
			headers: {
				"Content-Type": "application/json",
				"svix-id": world.name("invalid-msg"),
				"svix-timestamp": timestamp,
				"svix-signature": "v1,invalid",
			},
		},
	);
	expect(invalid.status()).toBe(401);
	const response = await world.api.context.post(
		`${world.productPath}/integrations/webhooks/CLERK`,
		{
			data: body,
			headers: {
				"Content-Type": "application/json",
				"svix-id": id,
				"svix-timestamp": timestamp,
				"svix-signature": signature(secret, id, timestamp, body),
			},
		},
	);
	expect(response.status(), await response.text()).toBe(200);
	await expect
		.poll(
			async () => {
				const result = await productClient.post<{ items: { email: string }[] }>(
					`${world.productPath}/product-users/search`,
					{ pagination: { limit: 20, offset: 0 } },
					200,
				);
				return result.items.some((user) => user.email === email);
			},
			{ timeout: 10_000 },
		)
		.toBe(true);
	await page.goto("/products/users");
	await expect(
		page.getByRole("row", { name: new RegExp(email) }),
	).toBeVisible();
	await page.goto(`/platform/${world.product.id}/integration-clerk`);
	await expect(
		page.getByText(/user created|webhook received|event processed/i).first(),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Delete Instance", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete instance", exact: true })
		.click();
	await expect(
		page.getByText("No Clerk instance configured", { exact: true }),
	).toBeVisible();
	await expect.poll(async () => (await integrations(world)).length).toBe(0);
});

// Covers: PRODUCT_INTEGRATION_SMTP
test("SMTP settings validate, connect to Mailpit, preserve blank passwords and persist pause", async ({
	page,
	world,
}) => {
	await page.goto(`/platform/${world.product.id}/integration-smtp`);
	await page
		.getByRole("button", { name: "Create SMTP Integration", exact: true })
		.click();
	await expect(
		page.getByText("Host is required", { exact: true }),
	).toBeVisible();
	await fillSMTP(page, world);
	await page
		.getByRole("button", { name: "Create SMTP Integration", exact: true })
		.click();
	await expect(
		page.getByRole("button", { name: "Delete Integration", exact: true }),
	).toBeVisible();
	await expect
		.poll(async () => (await integrations(world))[0]?.status)
		.toBe("ACTIVE");
	await expect(page.getByLabel("Password", { exact: true })).toHaveValue("");
	await page
		.getByLabel("From Name", { exact: true })
		.fill("Updated E2E Sender");
	await page
		.getByRole("button", { name: "Update Configuration", exact: true })
		.click();
	await expect(
		page.getByText("Configuration updated.", { exact: true }),
	).toBeVisible();
	await page.reload();
	await expect(page.getByLabel("From Name", { exact: true })).toHaveValue(
		"Updated E2E Sender",
	);
	await expect(page.getByLabel("Password", { exact: true })).toHaveValue("");
	await page.getByRole("switch", { name: "Enabled", exact: true }).uncheck();
	await page
		.getByRole("button", { name: "Update Configuration", exact: true })
		.click();
	await expect
		.poll(async () => (await integrations(world))[0]?.is_enabled)
		.toBe(false);
	await page.reload();
	await expect(
		page.getByRole("switch", { name: "Enabled", exact: true }),
	).not.toBeChecked();
	await page.getByRole("switch", { name: "Enabled", exact: true }).check();
	await page
		.getByRole("button", { name: "Update Configuration", exact: true })
		.click();
	await expect
		.poll(async () => (await integrations(world))[0]?.is_enabled)
		.toBe(true);
	await page
		.getByRole("button", { name: "Delete Integration", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Cancel", exact: true })
		.click();
	await expect(
		page.getByRole("button", { name: "Delete Integration", exact: true }),
	).toBeVisible();
	await page
		.getByRole("button", { name: "Delete Integration", exact: true })
		.click();
	await page
		.getByRole("alertdialog")
		.getByRole("button", { name: "Delete", exact: true })
		.click();
	await expect(
		page.getByRole("button", { name: "Create SMTP Integration", exact: true }),
	).toBeVisible();
});

// Covers: PRODUCT_EVENTS
test("event subscriptions filter, persist and deliver a verifiable signed webhook", async ({
	page,
	world,
}) => {
	const productClient = await world.productAPI();
	const receiver = await eventReceiver();
	try {
		await selectProduct(page, world.product);
		await page.goto("/products/events");
		const filter = page.getByPlaceholder(
			"Filter events by name, code, or description...",
		);
		await expect(
			page.getByRole("checkbox", { name: /organization\.created/ }),
		).toBeVisible();
		await page.getByRole("button", { name: /^Integrations \(/ }).click();
		await expect(
			page.getByRole("checkbox", { name: /organization\.created/ }),
		).toHaveCount(0);
		await page.getByRole("button", { name: /^Internal \(/ }).click();
		await filter.fill("does-not-exist");
		await expect(
			page.getByText("No events found", { exact: true }),
		).toBeVisible();
		await filter.fill("organization.created");
		await page
			.getByRole("button", { name: "Deselect all", exact: true })
			.click();
		await page.getByRole("checkbox", { name: /organization\.created/ }).check();
		await page
			.getByLabel("Event endpoint URL", { exact: true })
			.fill(receiver.url);
		const updateResponse = page.waitForResponse(
			(response) =>
				response.request().method() === "PUT" &&
				new URL(response.url()).pathname === world.productPath,
		);
		await page
			.getByRole("button", { name: "Save endpoint", exact: true })
			.click();
		const updated = (await (await updateResponse).json()) as ProductResponse;
		const secret = updated.config.events?.signing_secret;
		expect(secret).toMatch(/^whsec_/);
		await expect(
			page.getByText("New Signing Secret Minted", { exact: true }),
		).toBeVisible();
		await page
			.getByLabel("Event endpoint URL", { exact: true })
			.fill(`${receiver.url}/discard`);
		await page.getByRole("button", { name: "Discard", exact: true }).click();
		await expect(
			page.getByLabel("Event endpoint URL", { exact: true }),
		).toHaveValue(receiver.url);
		await page.reload();
		await expect(
			page.getByLabel("Event endpoint URL", { exact: true }),
		).toHaveValue(receiver.url);
		await expect(
			page.getByRole("checkbox", { name: /organization\.created/ }),
		).toBeChecked();
		await expect(
			page.getByText("New Signing Secret Minted", { exact: true }),
		).toHaveCount(0);
		await productClient.post(`${world.productPath}/organizations`, {
			name: world.name("event-org"),
		});
		await expect
			.poll(() => receiver.deliveries.length, { timeout: 10_000 })
			.toBe(1);
		const delivery = receiver.deliveries[0];
		expect(JSON.parse(delivery.body).type).toBe("organization.created");
		expect(delivery.headers["webhook-signature"]).toBe(
			signature(
				secret ?? "",
				String(delivery.headers["webhook-id"]),
				String(delivery.headers["webhook-timestamp"]),
				delivery.body,
			),
		);
		await page.getByLabel("Event endpoint URL", { exact: true }).fill("");
		await page
			.getByRole("button", { name: "Save endpoint", exact: true })
			.click();
		await expect
			.poll(
				async () =>
					(await world.api.get<ProductResponse>(world.productPath)).config
						.events?.endpoint_url ?? "",
			)
			.toBe("");
		await page.reload();
		await expect(
			page.getByLabel("Event endpoint URL", { exact: true }),
		).toHaveValue("");
	} finally {
		await receiver.close();
	}
});
