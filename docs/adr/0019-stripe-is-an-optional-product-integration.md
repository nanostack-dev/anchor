# ADR-0019: Stripe is an optional product integration

**Status:** Accepted for the sandbox prototype

## Context

Organizations already receive licenses through Anchor's schema, templates, values and migration history. Administrators need to manage a product's commercial prices and reconcile purchased subscriptions without switching to a separate application. Stripe remains the authority for payment and subscription state.

## Decision

Register Stripe in the existing product Integration Hub. Configuration, encrypted credentials, connection verification, enable/disable, deletion and tenant access use the same lifecycle as other integrations. The native API supports a test API key; the local prototype may use the already authenticated Stripe CLI without exporting its OAuth credentials. Both transports refuse live mode and pin the configured account.

Product Pricing links recurring Stripe prices to existing license templates. An organization's Billing tab starts hosted Checkout, opens the customer portal, changes prices and schedules or resumes cancellation. These pages use Anchor's normal authenticated layout and generated API client. Payment card entry stays in Stripe's hosted pages.

Stripe billing stores its own product/account identity, customer links, mutation intents and event inbox in PostgreSQL. Signed webhook receipt and queue insertion commit together. Durable queue workers retry reconciliation and periodically repair missed events. Per-instance advisory locks serialize external operations while mutation intents commit before Stripe calls, preserving recovery after a crash or upstream failure.

The integration calls the existing license services. Paid or trialing subscriptions grant their mapped template; terminal or unpaid states use a configured fallback. Incomplete payment preserves the current grant. Migrations carry forward organization adjustments, and repeated reconciliation of the same template writes no extra history. Licensing itself has no currency, invoice, payment or provider fields.

Billing management requires platform bearer authentication and product ownership. The only public billing operation is a signed webhook addressed to a product. Webhook data cannot select another tenant, account or customer link. The native prototype is sandbox-only; production rollout and commercial policies need a separate decision.

## Consequences

Stripe setup and billing behave as part of Anchor, while core licenses retain the provider-independent contract. Disconnecting Stripe leaves existing licenses and subscription objects intact; operators must resolve ongoing Stripe subscriptions before removing an integration if they expect future automatic license updates.

This extends ADR-0002's integration boundary. Anchor can display billing information through a provider adapter, while Stripe continues to determine what was paid and Anchor determines what is granted. Self-hosters can leave the integration unconfigured.
