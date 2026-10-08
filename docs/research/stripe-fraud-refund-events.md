# Stripe events for proactive fraud refunds

Research date: 2026-10-08. Primary sources only. This is the pre-implementation research snapshot; Anchor source was inspected at `f48f8aa`. The recommendations and gaps below describe that baseline, rather than the implemented feature. See the [current Stripe runbook](../prototypes/stripe-billing.md#refund-early-fraud-warnings) for shipped behavior and operator steps.

## Recommendation

Use **`radar.early_fraud_warning.created` as the primary trigger**, then retrieve current warning and charge state before deciding. Require `actionable == true`: Stripe defines this as a warning whose charge has neither received a dispute nor been fully refunded. This is a chance to prevent a future dispute, not a guarantee. [Event catalog](https://docs.stripe.com/api/events/types), [warning object](https://docs.stripe.com/api/radar/early_fraud_warnings/object), [retrieve warning](https://docs.stripe.com/api/radar/early_fraud_warnings/retrieve).

Stripe's first-party [July 10, 2025 workflow article](https://stripe.dev/blog/workflows-creating-early-fraud-alerts-for-streamlined-refunds) uses this exact sequence: warning created → retrieve charge → refund when the charge is below USD15 → otherwise notify a reviewer. USD15 is the article's example, not a universal fee or recommended threshold for every account. Current documentation supplies the additional eligibility and timing constraints below.

## Which event means what?

| Event | Current documented meaning | Recommended handling |
| --- | --- | --- |
| `radar.early_fraud_warning.created` | An issuer fraud warning was created. Its object references a charge and optionally a PaymentIntent. [Warning object](https://docs.stripe.com/api/radar/early_fraud_warnings/object). | Evaluate the opt-in refund policy using fresh state. |
| `radar.early_fraud_warning.updated` | The warning changed. [Event catalog](https://docs.stripe.com/api/events/types). | Reconcile the same decision/action; never treat it as permission for another refund. |
| `review.opened` | Radar added a transaction to its review queue, which can be controlled by review rules. [Review guide](https://docs.stripe.com/radar/transaction-reviews). | Separate manual-review or explicit risk-rule policy. It is not an issuer early fraud warning. |
| `charge.dispute.created` with `warning_needs_response` | An inquiry requires a response, not yet a formal chargeback. Other inquiry statuses are `warning_under_review` and `warning_closed`. [Dispute object](https://docs.stripe.com/api/disputes/object). | Separate opt-in inquiry policy. A full refund can resolve an inquiry without a dispute fee; check `is_charge_refundable`. [Dispute lifecycle](https://docs.stripe.com/disputes/how-disputes-work#inquiries), [dispute object](https://docs.stripe.com/api/disputes/object). |
| `charge.dispute.created` with `needs_response` or `under_review` | A formal chargeback is awaiting response or under review. [Dispute object](https://docs.stripe.com/api/disputes/object). | Route to dispute handling. The received-dispute fee has already been incurred, and a refund outside the dispute process is unavailable while it remains open. [Dispute lifecycle](https://docs.stripe.com/disputes/how-disputes-work#receive-a-dispute). |

The event name alone is insufficient to classify a dispute. Also consume `charge.dispute.updated` for subsequent state changes. [Event catalog](https://docs.stripe.com/api/events/types).

## Narrow safety predicates — proposed Anchor policy

Default the policy to **off**, scope it to one product/account, and set explicit currency/amount ceilings. Refund only when:

1. The signed event and retrieved resources belong to the configured account and mode; charge ownership is established through the Anchor subscription/product/organization association.
2. The warning is currently actionable and the card charge is captured, successful, and has a refundable remainder. Inspect capture/refund fields and existing refunds; do not rely on the original webhook snapshot. [Charge object](https://docs.stripe.com/api/charges/object), [refund API](https://docs.stripe.com/api/refunds/create).
3. No refund action for this account/charge is already pending or completed. Persist one decision and stable idempotency key before the API call, then reuse the same request on uncertain retries. Webhooks can arrive twice or out of order; Stripe supports idempotent retries. [Webhook guidance](https://docs.stripe.com/webhooks#handle-duplicate-events), [idempotent requests](https://docs.stripe.com/api/idempotent_requests).
4. Refund the eligible remainder rather than leaving a partial outstanding amount; partial refunds can still be disputed. If choosing `reason=fraudulent`, disclose that Stripe also adds the associated card and email to block lists. [Dispute lifecycle](https://docs.stripe.com/disputes/how-disputes-work#disputed-amount), [refund API](https://docs.stripe.com/api/refunds/create).

Track `refund.created`, `refund.updated`, and `refund.failed`. A Refund can be `pending`, `requires_action`, `succeeded`, `failed`, or `canceled`; an API response is not proof of completed reimbursement. Card refunds can remain pending for insufficient balance. A pending refund that is disputed can fail with `charge_for_pending_refund_disputed`; switch to dispute handling rather than attempt duplicate reimbursement. [Refund object](https://docs.stripe.com/api/refunds/object), [refund guide](https://docs.stripe.com/refunds#refund-events).

## Limits and economics

Warnings can arrive after chargebacks. Stripe advises against refunding every warning indiscriminately and suggests considering the charge amount relative to the account's dispute fee. Refunding generally does not erase the fraud warning/report. Refunds cannot guarantee prevention of a chargeback that arrives before reimbursement completes. [Dispute lifecycle](https://docs.stripe.com/disputes/how-disputes-work#early-fraud-warnings), [pending-refund failure](https://docs.stripe.com/refunds#handle-failed-refunds).

## Pre-implementation Anchor gap at f48f8aa

The baseline [handler](https://github.com/nanostack-dev/anchor/blob/f48f8aae248d2472125b95f8dc5d321585367924/apps/anchor/internal/stripebilling/billing/service.go#L638) extracts only top-level `customer`; warnings and disputes instead reference charges, so they become ignored. The [worker](https://github.com/nanostack-dev/anchor/blob/f48f8aae248d2472125b95f8dc5d321585367924/apps/anchor/internal/stripebilling/billing/service.go#L688) reconciles subscriptions only. There is no refund policy or financial-action receipt in [stored state](https://github.com/nanostack-dev/anchor/blob/f48f8aae248d2472125b95f8dc5d321585367924/apps/anchor/internal/stripebilling/billing/store.go#L25), although [state and queued work commit together](https://github.com/nanostack-dev/anchor/blob/f48f8aae248d2472125b95f8dc5d321585367924/apps/anchor/internal/stripebilling/store.go#L111).

Baseline [license reconciliation](https://github.com/nanostack-dev/anchor/blob/f48f8aae248d2472125b95f8dc5d321585367924/apps/anchor/internal/stripebilling/billing/service.go#L438) follows subscription status, not refund status. Specify refund-only versus cancellation/fallback separately; do not infer license loss from a refund. The [gateway accepts only test keys](https://github.com/nanostack-dev/anchor/blob/f48f8aae248d2472125b95f8dc5d321585367924/apps/anchor/internal/stripebilling/billing/sdk_gateway.go#L28), so production activation requires additional work.

## Practical verification — proposed, not executed

Stripe documents **`4000 0000 0000 5423`** for interactive sandbox payments that succeed and then receive an early fraud warning; use a future expiry and any three-digit CVC. For API tests, use `pm_card_createIssuerFraudRecord`. Inquiry tests use `pm_card_createDisputeInquiry`. [Official test values](https://docs.stripe.com/testing#disputes).

Test duplicate/updated warnings, a dispute arriving before the worker, inquiry versus formal statuses, foreign-product charges, partial/prior refunds, pending/failed refunds, and a crash after Stripe accepts a refund but before Anchor saves its receipt. These are recommended acceptance cases, not claims of completed testing.
