# ADR-0002: Anchor owns license state, not billing

**Status:** Extended by [ADR-0019](0019-stripe-is-an-optional-product-integration.md)

## Context

A licensing subsystem invites an obvious extension: if Anchor knows an organization is on "Pro", why not have it know what Pro costs, whether the invoice was paid, and when the trial ends?

Anchor is source-available under FSL and self-hostable. It has no money concepts anywhere in its current model.

## Decision

Anchor stores the *license*: which fields an organization is granted and at what values. The licensing domain has no prices, invoices, payment state, subscription lifecycle or trial periods. Optional provider integrations may store and display their own billing state and translate it into licenses; [ADR-0019](0019-stripe-is-an-optional-product-integration.md) defines the Stripe sandbox prototype.

A billing integration, an upstream API client, or a human writes the license through Anchor's existing license services and API. Anchor is the authority on **what is granted**. It is never the authority on **what was paid**.

Consequently, "plan" is not a word this project uses internally. A plan is a billing concept; the Anchor equivalent is a *license template*. See [`CONTEXT.md`](../../CONTEXT.md).

## Consequences

**Good.** Self-hosters do not inherit our commercial model. Licensing is universal; billing is ours.

**Good.** Subscription lifecycle — proration, dunning, trials, refunds, tax — is a genuinely large domain that would roughly double this repository. It stays out.

**Good.** The reverse direction stays cheap. A billing integration is additive: it becomes one more API client writing licenses. Removing billing once embedded would not have been.

**Cost.** Something must translate "customer bought Pro" into "write these license field values". That translation belongs to an optional integration, outside the license domain. Without one, licenses are written by hand or by Terraform.

**Cost.** Core licensing cannot answer "which organizations are past due". An enabled provider integration can display that provider's billing state.
