import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { promisify } from "node:util";
import {
	fixtureBackendPath,
	ownsStripeFixture,
	startStripeFixtureProcess,
	stopStripeFixture,
} from "./e2e-runtime.mjs";
import {
	createStripeFixtureState,
	fixtureAccount,
	fixtureOrigin,
	startStripeFixture,
	writeStripeFixtureWrapper,
} from "./e2e-stripe-fixture.mjs";

const flags = [
	"--color",
	"off",
	"--stripe-version",
	"2026-09-30.endive",
	"--stripe-account",
	fixtureAccount,
];
const params = (values) =>
	Object.entries(values).flatMap(([key, value]) => ["-d", `${key}=${value}`]);

function seeded() {
	const state = createStripeFixtureState();
	state.setOrigin("http://127.0.0.1:13099");
	state.control({ productId: "prd_first", action: "register" });
	const metadata = {
		"metadata[anchor_product_id]": "prd_first",
		"metadata[anchor_prototype_id]": "installation_first",
	};
	const request = (method, path, values = {}, key = "") =>
		state.command([
			method,
			path,
			...flags,
			...(method === "post" ? ["--confirm"] : []),
			...(key ? ["--idempotency", key] : []),
			...params(values),
		]);
	const product = request(
		"post",
		"/v1/products",
		{ ...metadata, name: "Fixture" },
		"product_key",
	).body;
	const price = request(
		"post",
		"/v1/prices",
		{
			product: product.id,
			currency: "usd",
			unit_amount: "500",
			"recurring[interval]": "month",
		},
		"price_key",
	).body;
	const customer = request(
		"post",
		"/v1/customers",
		{
			...metadata,
			"metadata[anchor_organization_id]": "org_first",
			name: "Fixture organization",
		},
		"customer_key",
	).body;
	const checkout = {
		customer: customer.id,
		"line_items[0][price]": price.id,
		client_reference_id: "org_first",
		"subscription_data[metadata][anchor_product_id]": "prd_first",
		"subscription_data[metadata][anchor_prototype_id]": "installation_first",
		"subscription_data[metadata][anchor_organization_id]": "org_first",
	};
	return { state, request, checkout };
}

test("lost Checkout result keeps one remote session and requires the original idempotent payload", () => {
	const { state, request, checkout } = seeded();
	state.control({ productId: "prd_first", action: "lose_checkout_response" });
	assert.match(
		request("post", "/v1/checkout/sessions", checkout, "checkout_key").failure,
		/lost after creation/,
	);
	const recovered = request(
		"post",
		"/v1/checkout/sessions",
		checkout,
		"checkout_key",
	).body;
	assert.equal(recovered.status, "open");
	assert.throws(
		() =>
			request(
				"post",
				"/v1/checkout/sessions",
				{ ...checkout, client_reference_id: "org_changed" },
				"checkout_key",
			),
		/different payload/,
	);
	const snapshot = state.control({
		productId: "prd_first",
		action: "snapshot",
	});
	assert.equal(snapshot.sessions.length, 1);
	assert.equal(snapshot.sessions[0].id, recovered.id);
});

test("completed Checkout derives exact ownership and disputed warnings never create a refund", () => {
	const { state, request, checkout } = seeded();
	request("post", "/v1/checkout/sessions", checkout, "checkout_key");
	const complete = state.control({
		productId: "prd_first",
		action: "complete_checkout",
		organizationId: "org_first",
	});
	const subscription = request(
		"get",
		`/v1/subscriptions/${complete.subscriptionId}`,
	).body;
	assert.deepEqual(subscription.metadata, {
		anchor_product_id: "prd_first",
		anchor_prototype_id: "installation_first",
		anchor_organization_id: "org_first",
	});
	const warning = state.control({
		productId: "prd_first",
		action: "disputed_warning",
		organizationId: "org_first",
	});
	assert.equal(
		request("get", `/v1/charges/${warning.object.charge}`).body.disputed,
		true,
	);
	const invoice = state.control({
		productId: "prd_first",
		action: "invoice_paid",
		organizationId: "org_first",
	});
	assert.equal(invoice.object.customer, complete.customerId);
	assert.equal(
		invoice.object.parent.subscription_details.subscription,
		complete.subscriptionId,
	);
	assert.throws(
		() =>
			state.control({
				productId: "prd_other",
				action: "complete_checkout",
				organizationId: "org_first",
			}),
		/Unregistered/,
	);
	assert.equal(
		state
			.control({ productId: "prd_first", action: "snapshot" })
			.calls.filter((call) => call.path === "/v1/refunds").length,
		0,
	);
});

test("Checkout retrieval overrides are scoped and never replace session or idempotency state", () => {
	const { state, request, checkout } = seeded();
	const original = structuredClone(
		request("post", "/v1/checkout/sessions", checkout, "checkout_key").body,
	);
	state.control({ productId: "prd_second", action: "register" });
	assert.throws(
		() =>
			state.control({
				productId: "prd_second",
				action: "checkout_foreign_customer",
				organizationId: "org_first",
			}),
		/Create Checkout through Anchor/,
	);
	for (const [action, expected] of [
		["checkout_foreign_customer", { customer: "cus_foreign_fixture" }],
		["checkout_unknown_status", { status: "unrecognized_fixture" }],
		["checkout_expired_status", { status: "expired" }],
	]) {
		state.control({
			productId: "prd_first",
			action,
			organizationId: "org_first",
		});
		assert.deepEqual(
			request("get", `/v1/checkout/sessions/${original.id}`).body,
			{ ...original, ...expected },
		);
		assert.deepEqual(
			request("post", "/v1/checkout/sessions", checkout, "checkout_key").body,
			original,
		);
		assert.deepEqual(
			state.control({ productId: "prd_first", action: "snapshot" }).sessions,
			[original],
		);
		state.control({
			productId: "prd_first",
			action: "checkout_response_reset",
			organizationId: "org_first",
		});
		assert.deepEqual(
			request("get", `/v1/checkout/sessions/${original.id}`).body,
			original,
		);
	}
	state.control({ productId: "prd_second", action: "dispose" });
	assert.equal(
		state.control({ productId: "prd_first", action: "snapshot" }).sessions
			.length,
		1,
	);
});

test("fixture origin and CLI wrapper refuse vendor URLs and never fall through to a real CLI", async () => {
	for (const origin of [
		"https://api.stripe.com",
		"http://localhost:13099",
		"http://127.0.0.1:13099/path",
		"http://user@127.0.0.1:13099",
	])
		assert.throws(() => fixtureOrigin(origin), /explicit loopback/);
	const fixture = await startStripeFixture();
	const directory = await mkdtemp(join(tmpdir(), "anchor-stripe-cli-fixture-"));
	try {
		const wrapper = await writeStripeFixtureWrapper(directory, fixture.url);
		const execute = promisify(execFile);
		const { stdout } = await execute(wrapper, [
			"whoami",
			"--format",
			"json",
			"--color",
			"off",
		]);
		assert.equal(JSON.parse(stdout).account_id, fixtureAccount);
		await assert.rejects(
			execute(wrapper, ["login"]),
			/Local Stripe fixture command failed/,
		);
	} finally {
		await fixture.close();
		await rm(directory, { recursive: true, force: true });
	}
});

test("managed fixture preserves process ownership and changes only the backend PATH", async () => {
	const directory = await mkdtemp(
		join(tmpdir(), "anchor-stripe-runtime-fixture-"),
	);
	const metadata = { directory };
	const originalPath = process.env.PATH;
	try {
		await startStripeFixtureProcess(metadata);
		assert.equal(await ownsStripeFixture(metadata), true);
		assert.equal(
			fixtureBackendPath(metadata, "/fixture/original"),
			`${join(directory, "stripe-cli")}:/fixture/original`,
		);
		assert.equal(process.env.PATH, originalPath);
		await assert.rejects(
			stopStripeFixture({
				...metadata,
				stripeFixtureStartedAt: "foreign process timestamp",
			}),
			/ownership changed/,
		);
		assert.equal(await ownsStripeFixture(metadata), true);
		assert.throws(
			() =>
				fixtureBackendPath(
					{ ...metadata, stripeFixtureURL: "https://api.stripe.com" },
					"/fixture/original",
				),
			/explicit loopback/,
		);
		assert.throws(
			() =>
				fixtureBackendPath(
					{ ...metadata, stripeFixtureWrapperDirectory: "/foreign/wrapper" },
					"/fixture/original",
				),
			/belong to this runtime/,
		);
	} finally {
		await stopStripeFixture(metadata);
		assert.equal(await ownsStripeFixture(metadata), false);
		await rm(directory, { recursive: true, force: true });
	}
});

test("interrupted fixture startup cleans up its already recorded child", async () => {
	const directory = await mkdtemp(
		join(tmpdir(), "anchor-stripe-runtime-aborted-"),
	);
	const metadata = { directory };
	const controller = new AbortController();
	try {
		await assert.rejects(
			startStripeFixtureProcess(metadata, {
				signal: controller.signal,
				persist: async () =>
					controller.abort(new Error("fixture startup interrupted")),
			}),
			/fixture startup interrupted/,
		);
		assert.equal(await ownsStripeFixture(metadata), false);
		assert.throws(() => process.kill(metadata.stripeFixturePID, 0), {
			code: "ESRCH",
		});
	} finally {
		await stopStripeFixture(metadata);
		await rm(directory, { recursive: true, force: true });
	}
});
