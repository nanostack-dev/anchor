import { createHmac, randomUUID } from "node:crypto";
import { expect } from "playwright/test";
import type { IntegrationInstanceResponse } from "../../src/client";
import type { World } from "./fixtures";
import { runtime } from "./runtime";

const accountId = "acct_AnchorE2EFixture";
const webhookSecret = "whsec_anchor_e2e_fixture";

export interface StripeFixtureSnapshot {
	calls: {
		method: string;
		path: string;
		params: Record<string, string>;
		idempotencyKey: string;
	}[];
	sessions: {
		id: string;
		url: string;
		status: string;
		customer: string;
		client_reference_id: string;
		subscription?: string;
	}[];
}

interface FixtureEvent {
	type: string;
	object: { id: string; [field: string]: unknown };
}

export async function createNativeStripeFixture(world: World) {
	let instanceId: string | undefined;
	const value = runtime().stripeFixtureURL;
	if (!value)
		throw new Error(
			"Start the owned loopback Stripe fixture before this native journey.",
		);
	const origin = new URL(value);
	if (
		origin.protocol !== "http:" ||
		origin.hostname !== "127.0.0.1" ||
		!origin.port ||
		origin.username ||
		origin.password ||
		origin.pathname !== "/" ||
		origin.search ||
		origin.hash
	)
		throw new Error("The Stripe fixture must use its owned loopback origin.");
	async function control<T>(
		action: string,
		organizationId?: string,
	): Promise<T> {
		const response = await fetch(`${origin.origin}/control`, {
			method: "POST",
			headers: { "Content-Type": "application/json" },
			body: JSON.stringify({
				productId: world.product.id,
				action,
				organizationId,
			}),
			redirect: "error",
			signal: AbortSignal.timeout(10000),
		});
		if (!response.ok)
			throw new Error(
				`Scoped Stripe fixture control failed (${response.status}).`,
			);
		return response.json() as Promise<T>;
	}
	async function deliver(action: string, organizationId: string) {
		const event = await control<FixtureEvent>(action, organizationId);
		const eventId = `evt_${randomUUID().replaceAll("-", "")}`;
		const timestamp = Math.floor(Date.now() / 1000);
		const body = JSON.stringify({
			id: eventId,
			object: "event",
			type: event.type,
			account: accountId,
			livemode: false,
			created: timestamp,
			api_version: "2026-09-30.endive",
			data: { object: event.object },
		});
		const signature = createHmac("sha256", webhookSecret)
			.update(`${timestamp}.${body}`)
			.digest("hex");
		const response = await world.api.context.post(
			`${world.productPath}/billing/stripe/webhook`,
			{
				data: body,
				headers: {
					"Content-Type": "application/json",
					"Stripe-Signature": `t=${timestamp},v1=${signature}`,
				},
			},
		);
		expect(
			response.status(),
			"the native public webhook must accept the signed fixture bytes",
		).toBe(200);
		return {
			eventId,
			resourceId: event.object.id,
			chargeId:
				typeof event.object.charge === "string" ? event.object.charge : "",
		};
	}
	await control("register");
	return {
		async connect() {
			const instance = await world.api.post<IntegrationInstanceResponse>(
				`${world.productPath}/integrations`,
				{
					provider_type: "STRIPE",
					config: {
						auth_method: "LOCAL_CLI",
						account_id: accountId,
						webhook_secret: webhookSecret,
						return_url: runtime().frontendURL,
					},
				},
			);
			await expect
				.poll(
					async () =>
						(
							await world.api.get<IntegrationInstanceResponse>(
								`${world.productPath}/integrations/${instance.id}`,
							)
						).status,
					{ timeout: 15000 },
				)
				.toBe("ACTIVE");
			instanceId = instance.id;
			return instance;
		},
		loseNextCheckoutResponse: () => control("lose_checkout_response"),
		foreignCheckoutCustomer: (organizationId: string) =>
			control("checkout_foreign_customer", organizationId),
		unknownCheckoutStatus: (organizationId: string) =>
			control("checkout_unknown_status", organizationId),
		expiredCheckoutStatus: (organizationId: string) =>
			control("checkout_expired_status", organizationId),
		resetCheckoutResponse: (organizationId: string) =>
			control("checkout_response_reset", organizationId),
		completeCheckout: (organizationId: string) =>
			control<{ subscriptionId: string; customerId: string }>(
				"complete_checkout",
				organizationId,
			),
		async sendInvoicePaid(organizationId: string) {
			const { eventId, resourceId } = await deliver(
				"invoice_paid",
				organizationId,
			);
			return { eventId, resourceId, invoiceId: resourceId };
		},
		async sendDisputedWarning(organizationId: string) {
			const { eventId, resourceId, chargeId } = await deliver(
				"disputed_warning",
				organizationId,
			);
			return { eventId, resourceId, warningId: resourceId, chargeId };
		},
		snapshot: () => control<StripeFixtureSnapshot>("snapshot"),
		async dispose() {
			if (instanceId)
				await world.api.put(`${world.productPath}/integrations/${instanceId}`, {
					is_enabled: false,
				});
			return control("dispose");
		},
	};
}
