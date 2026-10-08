import type { BillingAPI } from "./billing-api";
import {
	FraudRefundCurrency,
	FraudRefundStatus,
	type Organization,
	type State,
} from "./billing-types";

const timestamp = "2026-10-07T14:00:00Z";

export const fixtureState: State = {
	account: { id: "acct_sandbox", name: "Anchor sandbox", mode: "sandbox" },
	product: { id: "prd_example", name: "Echopoint" },
	templates: [
		{
			id: "tpl_free",
			name: "Free",
			values: { seats: 1, max_flows: 5 },
			archived: false,
		},
		{
			id: "tpl_pro",
			name: "Pro",
			values: { seats: 10, max_flows: 100 },
			archived: false,
		},
		{
			id: "tpl_enterprise",
			name: "Enterprise",
			values: { seats: 100, max_flows: 1000 },
			archived: false,
		},
	],
	prices: [
		{
			id: "price_pro",
			name: "Pro monthly",
			template_id: "tpl_pro",
			amount: 2900,
			currency: "usd",
			interval: "month",
			stripe_product_id: "prod_pro",
			stripe_price_id: "price_stripe_pro",
			active: true,
		},
		{
			id: "price_enterprise",
			name: "Enterprise yearly",
			template_id: "tpl_enterprise",
			amount: 99900,
			currency: "usd",
			interval: "year",
			stripe_product_id: "prod_enterprise",
			stripe_price_id: "price_stripe_enterprise",
			active: true,
		},
	],
	organizations: [
		{
			id: "org_acme",
			name: "Acme",
			template_id: "tpl_pro",
			license_values: { seats: 10, max_flows: 100 },
			customer_id: "cus_acme",
			subscription_id: "sub_acme",
			price_id: "price_pro",
			status: "active",
			cancel_at_period_end: false,
			current_period_end: "2026-11-07T14:00:00Z",
			sync_state: "synced",
			last_synced_at: timestamp,
			sync_error: "",
			pending_update: false,
		},
		{
			id: "org_new",
			name: "New organization",
			template_id: "",
			license_values: {},
			customer_id: "",
			subscription_id: "",
			price_id: "",
			status: "none",
			cancel_at_period_end: false,
			current_period_end: null,
			sync_state: "pending",
			last_synced_at: null,
			sync_error: "",
			pending_update: false,
		},
	],
	settings: {
		fallback_template_id: "tpl_free",
		fraud_refund_policy: {
			enabled: false,
			currency: FraudRefundCurrency.USD,
			max_amount: 0,
		},
	},
	fraud_refunds: [],
	events: [
		{
			id: "evt_checkout",
			type: "checkout.session.completed",
			organization_id: "org_acme",
			status: "processed",
			received_at: timestamp,
			last_error: "",
		},
	],
};

export const fraudEnabledFixtureState: State = {
	...structuredClone(fixtureState),
	settings: {
		fallback_template_id: "tpl_free",
		fraud_refund_policy: {
			enabled: true,
			currency: FraudRefundCurrency.CAD,
			max_amount: 1500,
		},
	},
	fraud_refunds: [
		{
			id: "fraud_refunded",
			warning_id: "issfr_warning_refunded",
			charge_id: "ch_payment_refunded",
			invoice_id: "in_acme",
			subscription_id: "sub_acme",
			organization_id: "org_acme",
			refund_id: "re_completed",
			currency: "cad",
			amount: 1200,
			status: FraudRefundStatus.FRAUD_REFUND_SUCCEEDED,
			reason: "refund_status_updated",
			last_error: "",
			created_at: timestamp,
			updated_at: timestamp,
		},
		{
			id: "fraud_skipped",
			warning_id: "issfr_warning_skipped",
			charge_id: "",
			invoice_id: "",
			subscription_id: "",
			organization_id: "",
			refund_id: "",
			currency: "",
			amount: 0,
			status: FraudRefundStatus.FRAUD_REFUND_SKIPPED,
			reason: "ownership_unverified",
			last_error: "",
			created_at: timestamp,
			updated_at: timestamp,
		},
		{
			id: "fraud_pending",
			warning_id: "issfr_warning_pending",
			charge_id: "ch_payment_pending",
			invoice_id: "in_acme",
			subscription_id: "sub_acme",
			organization_id: "org_acme",
			refund_id: "re_pending",
			currency: "cad",
			amount: 1499,
			status: FraudRefundStatus.FRAUD_REFUND_PENDING,
			reason: "refund_requested",
			last_error: "",
			created_at: timestamp,
			updated_at: timestamp,
		},
		{
			id: "fraud_failed",
			warning_id: "issfr_warning_failed",
			charge_id: "ch_payment_failed",
			invoice_id: "in_acme",
			subscription_id: "sub_acme",
			organization_id: "org_acme",
			refund_id: "re_failed",
			currency: "cad",
			amount: 100,
			status: FraudRefundStatus.FRAUD_REFUND_FAILED,
			reason: "refund_failed",
			last_error:
				"Stripe could not complete this refund. Review the payment in Stripe before taking another action.",
			created_at: timestamp,
			updated_at: timestamp,
		},
	],
};

export const fraudWorstCaseFixtureState: State = (() => {
	const state = structuredClone(fraudEnabledFixtureState);
	state.organizations[0].name =
		"Aleksandra Wiśniewska-Kowalczyk — Northwind International Holdings 日本 🌍";
	state.settings.fraud_refund_policy.max_amount = 99999999;
	state.fraud_refunds[0].amount = 99999999;
	state.fraud_refunds[0].currency = "eur";
	state.fraud_refunds[3].last_error =
		"The connected account’s available balance is insufficient to complete this refund. The refund remains failed until an operator reviews the payment and account balance in Stripe. No subscription was canceled and the organization’s adjusted license values remain unchanged.";
	for (const status of [
		FraudRefundStatus.FRAUD_REFUND_PROCESSING,
		FraudRefundStatus.FRAUD_REFUND_REQUIRES_ACTION,
		FraudRefundStatus.FRAUD_REFUND_CANCELED,
		FraudRefundStatus.FRAUD_REFUND_REVIEW_REQUIRED,
	]) {
		state.fraud_refunds.push({
			...state.fraud_refunds[2],
			id: `fraud_${status}`,
			status,
			reason:
				status === FraudRefundStatus.FRAUD_REFUND_REVIEW_REQUIRED
					? "idempotency_window_expired"
					: "refund_status_updated",
		});
	}
	return state;
})();

export const emptyFixtureState: State = {
	...structuredClone(fixtureState),
	prices: [],
	organizations: [],
	events: [],
};

export const oneFixtureState: State = {
	...structuredClone(fixtureState),
	prices: [structuredClone(fixtureState.prices[0])],
	organizations: [structuredClone(fixtureState.organizations[0])],
};

export const worstCaseFixtureState: State = (() => {
	const state = structuredClone(fixtureState);
	state.account.name =
		"Northwind International Holdings — Europe, Amérique du Nord & 日本 🌍";
	state.product.name =
		"Shared organizational platform for international teams and partners";
	state.templates[1].name =
		"International professional license with advanced collaboration and governance";
	state.prices[0].name =
		"ProfessionalSubscriptionForInternationalOrganizationsWithoutSpaces"
			.repeat(2)
			.slice(0, 120);
	state.prices[0].amount = 99999999;
	state.prices[0].currency = "cad";
	state.prices[1].active = false;
	state.organizations[0].name =
		"Aleksandra Wiśniewska-Kowalczyk — Northwind International Holdings 日本 🚀";
	state.organizations[0].license_values = {
		seats: 1284,
		webhook_url: `https://northwind-industries-holdings.example.com/${"international-organizations/".repeat(12)}`,
		sso: true,
	};
	state.organizations[0].sync_error =
		"Anchor is temporarily unavailable. Stripe billing is unchanged; use Reconcile after the API recovers.";
	state.organizations[0].sync_state = "error";
	state.organizations[0].pending_update = true;
	state.organizations[0].cancel_at_period_end = true;
	state.organizations[1].name = "Jo";
	state.events.push({
		id: "evt_reconciliation",
		type: "customer.subscription.updated",
		organization_id: "org_acme",
		status: "error",
		received_at: timestamp,
		last_error: state.organizations[0].sync_error,
	});
	return state;
})();

export function createFixtureAPI(initial: State = fixtureState): BillingAPI {
	const state = structuredClone(initial);
	let nextPrice = 1;
	const organization = (id: string): Organization => {
		const found = state.organizations.find((item) => item.id === id);
		if (!found) throw { error: "Organization not found." };
		return found;
	};
	const synchronize = (item: Organization) => {
		const templateId = ["active", "trialing"].includes(item.status)
			? state.prices.find((price) => price.id === item.price_id)?.template_id
			: state.settings.fallback_template_id;
		const template = state.templates.find(
			(candidate) => candidate.id === templateId,
		);
		item.template_id = template?.id ?? "";
		item.license_values = structuredClone(template?.values ?? {});
		item.sync_state = "synced";
		item.last_synced_at = timestamp;
		item.sync_error = "";
		return structuredClone(item);
	};
	return {
		load: async () => structuredClone(state),
		createPrice: async (body) => {
			const id = `price_created_${nextPrice++}`;
			const price = {
				...body,
				id,
				stripe_product_id: `prod_${id}`,
				stripe_price_id: `stripe_${id}`,
				active: true,
			};
			state.prices.push(price);
			return structuredClone(price);
		},
		archivePrice: async (id) => {
			const price = state.prices.find((item) => item.id === id);
			if (!price) throw { error: "Price not found." };
			price.active = false;
			return structuredClone(price);
		},
		updateSettings: async (body) => {
			state.settings = { ...state.settings, ...body };
			return structuredClone(state.settings);
		},
		checkout: async (id) => {
			organization(id).customer_id = `cus_${id}`;
			return "https://checkout.stripe.com/c/pay/cs_test_fixture";
		},
		changeSubscription: async (id, body) => {
			const item = organization(id);
			item.price_id = body.price_id;
			return synchronize(item);
		},
		cancel: async (id) => {
			const item = organization(id);
			item.cancel_at_period_end = true;
			return synchronize(item);
		},
		resume: async (id) => {
			const item = organization(id);
			item.cancel_at_period_end = false;
			return synchronize(item);
		},
		portal: async (id) => {
			organization(id);
			return "https://billing.stripe.com/p/session/test_fixture";
		},
		sync: async (id) => synchronize(organization(id)),
	};
}
