import { randomUUID } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { createServer } from "node:http";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

export const fixtureAccount = "acct_AnchorE2EFixture";
export const fixtureWebhookSecret = "whsec_anchor_e2e_fixture";
const fixtureVersion = "2026-09-30.endive";

export function fixtureOrigin(value) {
	const url = new URL(value);
	if (
		url.protocol !== "http:" ||
		url.hostname !== "127.0.0.1" ||
		!url.port ||
		url.username ||
		url.password ||
		url.pathname !== "/" ||
		url.search ||
		url.hash
	)
		throw new Error("Stripe fixture requires its explicit loopback origin.");
	return url.origin;
}

const id = (prefix) => `${prefix}_${randomUUID().replaceAll("-", "")}`;
const metadata = (params, prefix = "metadata") =>
	Object.fromEntries(
		Object.entries(params)
			.filter(([key]) => key.startsWith(`${prefix}[`))
			.map(([key, value]) => [key.slice(prefix.length + 1, -1), value]),
	);
const list = (values, url) => ({
	object: "list",
	data: values,
	has_more: false,
	url,
});

export function createStripeFixtureState() {
	const products = new Map();
	const records = new Map();
	const replay = new Map();
	let origin;
	function productFor(record) {
		const product = products.get(record?.metadata?.anchor_product_id);
		if (!product) throw new Error("Unregistered Stripe fixture product.");
		return product;
	}
	function put(object, product) {
		records.set(object.id, object);
		product.resources.add(object.id);
		return object;
	}
	function get(objectID) {
		const value = records.get(objectID);
		if (!value) throw new Error(`Unknown Stripe fixture resource: ${objectID}`);
		return value;
	}
	function control({ productId, action, organizationId }) {
		if (!/^[a-zA-Z0-9_-]{1,128}$/.test(productId))
			throw new Error("A scoped fixture product is required.");
		if (action === "register") {
			if (products.has(productId))
				throw new Error("Fixture product already registered.");
			products.set(productId, {
				resources: new Set(),
				calls: [],
				loseCheckout: false,
				checkoutRetrieveOverrides: new Map(),
			});
			return { accountId: fixtureAccount };
		}
		const product = products.get(productId);
		if (!product) throw new Error("Unregistered Stripe fixture product.");
		if (action === "dispose") {
			for (const resource of product.resources) records.delete(resource);
			for (const [key, value] of replay)
				if (value.productId === productId) replay.delete(key);
			products.delete(productId);
			return {};
		}
		if (action === "lose_checkout_response") {
			product.loseCheckout = true;
			return {};
		}
		if (action === "snapshot")
			return {
				calls: product.calls,
				sessions: [...product.resources]
					.map((key) => get(key))
					.filter((value) => value.object === "checkout.session"),
			};
		const session = [...product.resources]
			.map((key) => get(key))
			.find(
				(value) =>
					value.object === "checkout.session" &&
					value.client_reference_id === organizationId,
			);
		if (!session)
			throw new Error("Create Checkout through Anchor before completing it.");
		const overrides = {
			checkout_foreign_customer: { customer: "cus_foreign_fixture" },
			checkout_unknown_status: { status: "unrecognized_fixture" },
			checkout_expired_status: { status: "expired" },
		};
		if (Object.hasOwn(overrides, action)) {
			product.checkoutRetrieveOverrides.set(session.id, overrides[action]);
			return {};
		}
		if (action === "checkout_response_reset") {
			product.checkoutRetrieveOverrides.delete(session.id);
			return {};
		}
		if (action === "complete_checkout") {
			if (!session.subscription) {
				const price = get(session.fixturePrice);
				const subscription = put(
					{
						id: id("sub"),
						object: "subscription",
						livemode: false,
						customer: session.customer,
						status: "active",
						created: Math.floor(Date.now() / 1000),
						cancel_at_period_end: false,
						metadata: session.fixtureMetadata,
						items: list(
							[
								{
									id: id("si"),
									object: "subscription_item",
									price,
									current_period_end:
										Math.floor(Date.now() / 1000) + 86400 * 30,
								},
							],
							"/v1/subscription_items",
						),
					},
					product,
				);
				session.subscription = subscription.id;
				session.status = "complete";
			}
			return {
				subscriptionId: session.subscription,
				customerId: session.customer,
			};
		}
		if (!session.subscription)
			throw new Error("Complete the fixture Checkout first.");
		if (action === "invoice_paid") {
			const subscription = get(session.subscription);
			const price = get(session.fixturePrice);
			const invoice = put(
				{
					id: id("in"),
					object: "invoice",
					livemode: false,
					customer: session.customer,
					status: "paid",
					currency: price.currency,
					amount_paid: price.unit_amount,
					parent: {
						type: "subscription_details",
						subscription_details: {
							subscription: subscription.id,
							metadata: subscription.metadata,
						},
					},
				},
				product,
			);
			return { type: "invoice.paid", object: invoice };
		}
		if (action === "disputed_warning") {
			const price = get(session.fixturePrice);
			const charge = put(
				{
					id: id("ch"),
					object: "charge",
					livemode: false,
					customer: session.customer,
					payment_intent: id("pi"),
					metadata: session.fixtureMetadata,
					amount: price.unit_amount,
					amount_captured: price.unit_amount,
					amount_refunded: 0,
					currency: price.currency,
					paid: true,
					captured: true,
					status: "succeeded",
					disputed: true,
					payment_method_details: { type: "card", card: {} },
				},
				product,
			);
			const warning = put(
				{
					id: id("issfr"),
					object: "radar.early_fraud_warning",
					livemode: false,
					actionable: true,
					charge: charge.id,
					payment_intent: charge.payment_intent,
				},
				product,
			);
			return { type: "radar.early_fraud_warning.created", object: warning };
		}
		throw new Error(`Unsupported Stripe fixture control: ${action}`);
	}
	function command(args) {
		if (args.join(" ") === "whoami --format json --color off")
			return {
				body: {
					account_id: fixtureAccount,
					display_name: "Anchor E2E fixture",
					mode: "test",
				},
			};
		const [method, path, ...flags] = args;
		if (
			!["get", "post", "delete"].includes(method) ||
			!path?.startsWith("/v1/")
		)
			throw new Error("Unsupported fake Stripe CLI command.");
		const params = {};
		const options = {};
		for (let index = 0; index < flags.length; index++) {
			const flag = flags[index];
			if (flag === "--confirm") continue;
			const value = flags[++index];
			if (value === undefined)
				throw new Error("Incomplete Stripe CLI fixture flag.");
			if (flag === "-d") {
				const separator = value.indexOf("=");
				if (separator < 1) throw new Error("Malformed fixture form parameter.");
				params[value.slice(0, separator)] = value.slice(separator + 1);
			} else if (
				[
					"--color",
					"--stripe-version",
					"--stripe-account",
					"--idempotency",
				].includes(flag)
			)
				options[flag] = value;
			else throw new Error(`Unsupported Stripe CLI fixture flag: ${flag}`);
		}
		if (
			options["--stripe-account"] !== fixtureAccount ||
			options["--stripe-version"] !== fixtureVersion
		)
			throw new Error(
				"Fixture calls must pin the sandbox account and SDK version.",
			);
		let product;
		if (path === "/v1/products" || path === "/v1/customers")
			product = productFor({ metadata: metadata(params) });
		else if (path === "/v1/prices") product = productFor(get(params.product));
		else if (path === "/v1/checkout/sessions" || path === "/v1/subscriptions")
			product = productFor(get(params.customer));
		else if (path === "/v1/refunds") product = productFor(get(params.charge));
		else {
			const target = get(
				path.split("/").at(-1) === "expire"
					? path.split("/").at(-2)
					: path.split("/").at(-1),
			);
			product = [...products.values()].find((value) =>
				value.resources.has(target.id),
			);
		}
		const productId = [...products].find(([, value]) => value === product)[0];
		product.calls.push({
			method,
			path,
			params,
			idempotencyKey: options["--idempotency"] ?? "",
		});
		const replayKey = options["--idempotency"];
		if (replayKey && replay.has(replayKey)) {
			const existing = replay.get(replayKey);
			if (
				existing.productId !== productId ||
				existing.method !== method ||
				existing.path !== path ||
				JSON.stringify(existing.params) !== JSON.stringify(params)
			)
				throw new Error(
					"Fixture idempotency key was reused with a different payload.",
				);
			return { body: existing.body };
		}
		let body;
		if (method === "get" && path === "/v1/subscriptions")
			body = list(
				[...product.resources]
					.map((key) => get(key))
					.filter(
						(value) =>
							value.object === "subscription" &&
							value.customer === params.customer,
					),
				path,
			);
		else if (method === "get") {
			body = get(path.split("/").at(-1));
			if (body.object === "checkout.session")
				body = { ...body, ...product.checkoutRetrieveOverrides.get(body.id) };
		} else if (method === "post" && path === "/v1/products")
			body = put(
				{
					id: id("prod"),
					object: "product",
					livemode: false,
					active: true,
					name: params.name,
					metadata: metadata(params),
				},
				product,
			);
		else if (method === "post" && path === "/v1/prices")
			body = put(
				{
					id: id("price"),
					object: "price",
					livemode: false,
					active: true,
					product: params.product,
					unit_amount: Number(params.unit_amount),
					currency: params.currency,
					recurring: { interval: params["recurring[interval]"] },
					nickname: params.nickname,
					metadata: metadata(params),
				},
				product,
			);
		else if (method === "post" && path === "/v1/customers")
			body = put(
				{
					id: id("cus"),
					object: "customer",
					livemode: false,
					name: params.name,
					metadata: metadata(params),
				},
				product,
			);
		else if (method === "post" && path === "/v1/checkout/sessions")
			body = put(
				{
					id: id("cs"),
					object: "checkout.session",
					livemode: false,
					status: "open",
					customer: params.customer,
					client_reference_id: params.client_reference_id,
					fixturePrice: params["line_items[0][price]"],
					fixtureMetadata: metadata(params, "subscription_data[metadata]"),
					url: `${origin}/hosted/checkout`,
				},
				product,
			);
		else if (method === "post" && path.endsWith("/expire")) {
			body = get(path.split("/").at(-2));
			body.status = "expired";
		} else if (method === "post" && path.startsWith("/v1/prices/")) {
			body = get(path.split("/").at(-1));
			body.active = params.active !== "false";
		} else
			throw new Error(
				`Unsupported fixture Stripe operation: ${method} ${path}`,
			);
		if (replayKey)
			replay.set(replayKey, { productId, method, path, params, body });
		if (
			path === "/v1/checkout/sessions" &&
			method === "post" &&
			product.loseCheckout
		) {
			product.loseCheckout = false;
			return { failure: "Fixture Checkout response lost after creation." };
		}
		return { body };
	}
	return {
		control,
		command,
		setOrigin(value) {
			origin = fixtureOrigin(value);
		},
	};
}

export async function startStripeFixture({ port = 0 } = {}) {
	if (!Number.isInteger(port) || port < 0 || port > 65535)
		throw new Error("Fixture port must be an integer from 0 through 65535.");
	const state = createStripeFixtureState();
	const server = createServer(async (request, response) => {
		try {
			if (request.method === "GET" && request.url === "/health") {
				response.writeHead(200, { "Content-Type": "application/json" });
				response.end(
					JSON.stringify({ fixture: "anchor-stripe", pid: process.pid }),
				);
				return;
			}
			if (
				request.method !== "POST" ||
				!["/control", "/cli"].includes(request.url)
			)
				throw new Error("Unknown fixture route.");
			let raw = "";
			for await (const chunk of request) {
				raw += chunk;
				if (raw.length > 65536)
					throw new Error("Fixture request exceeds its size limit.");
			}
			const input = JSON.parse(raw);
			const body =
				request.url === "/control"
					? state.control(input)
					: state.command(input.args);
			response.writeHead(200, { "Content-Type": "application/json" });
			response.end(JSON.stringify(body));
		} catch (error) {
			response.writeHead(400, { "Content-Type": "application/json" });
			response.end(JSON.stringify({ error: error.message }));
		}
	});
	await new Promise((resolve, reject) => {
		server.once("error", reject);
		server.listen(port, "127.0.0.1", resolve);
	});
	const url = `http://127.0.0.1:${server.address().port}`;
	state.setOrigin(url);
	return {
		url,
		server,
		close: () =>
			new Promise((resolve, reject) =>
				server.close((error) => (error ? reject(error) : resolve())),
			),
	};
}

export async function runFixtureCLI(origin, args) {
	const response = await fetch(`${fixtureOrigin(origin)}/cli`, {
		method: "POST",
		headers: { "Content-Type": "application/json" },
		body: JSON.stringify({ args }),
		redirect: "error",
		signal: AbortSignal.timeout(10000),
	});
	const result = await response.json();
	if (!response.ok || result.failure)
		throw new Error(result.failure ?? result.error ?? "Fixture CLI failed.");
	return result.body;
}

export async function writeStripeFixtureWrapper(directory, origin) {
	const url = fixtureOrigin(origin);
	await mkdir(directory, { recursive: true, mode: 0o700 });
	const wrapper = join(directory, "stripe");
	await writeFile(
		wrapper,
		`#!${process.execPath}\nimport { runFixtureCLI } from ${JSON.stringify(import.meta.url)};\ntry { console.log(JSON.stringify(await runFixtureCLI(${JSON.stringify(url)}, process.argv.slice(2)))); } catch { console.error("Local Stripe fixture command failed."); process.exitCode = 1; }\n`,
		{ mode: 0o700 },
	);
	return wrapper;
}

if (
	process.argv[1] &&
	import.meta.url === pathToFileURL(process.argv[1]).href
) {
	const args = process.argv.slice(2);
	if (args.length !== 2 || args[0] !== "--port")
		throw new Error("Start the fixture with --port <owned loopback port>.");
	const fixture = await startStripeFixture({ port: Number(args[1]) });
	console.log(JSON.stringify({ url: fixture.url }));
	const stop = async () => {
		await fixture.close();
		process.exitCode = 0;
	};
	process.once("SIGINT", stop);
	process.once("SIGTERM", stop);
}
