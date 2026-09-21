Tracker: https://github.com/nanostack-dev/anchor/issues/128 (`ready-for-agent`)

## Problem Statement

A Product built on Anchor (EchoPoint today) only learns about Organization, membership, and license changes by calling Anchor on demand. Caches go stale. Clerk can write Product Users into Anchor without the Product hearing about it. There is no push path from Anchor to the Product backend.

## Solution

Anchor emits **events** when Product-scoped resources change. A Product registers an **endpoint**. Anchor makes a **delivery**: an HTTPS POST signed with Standard Webhooks (Svix), the same dialect Clerk already uses inbound. The body is thin. The Product fetches current state from the API if it needs it.

The first consumer is EchoPoint. The first event types are organization and membership and license. Other features add a type and call `Emit` in the same transaction as the write. The delivery worker does not change.

## User Stories

1. As a Product backend, I want Anchor to POST an event when an Organization is created, so that I do not poll for new customers.
2. As a Product backend, I want Anchor to POST an event when an Organization is updated, so that my copy of name and metadata stays current.
3. As a Product backend, I want Anchor to POST an event when a membership is added, so that I can grant access without waiting for the next request.
4. As a Product backend, I want Anchor to POST an event when a membership role changes, so that I can refresh permissions.
5. As a Product backend, I want Anchor to POST an event when a membership is removed, so that I can revoke access.
6. As a Product backend, I want Anchor to POST an event when an Organization license is instantiated, adjusted, or migrated, so that my limit cache is not 30 seconds late.
7. As a Product backend, I want `data` to carry identifiers only, so that a retried delivery cannot overwrite newer state.
8. As a Product backend, I want to fetch the live Organization, membership, or license from the API after an event, so that I act on current data.
9. As a Product backend, I want every delivery signed with Standard Webhooks HMAC, so that I can reject a forged POST.
10. As a Product backend, I want `webhook-id` to stay the same across retries, so that I can ignore duplicates.
11. As a Product backend, I want `webhook-timestamp` in the signature, so that I can reject a replay outside the tolerance window.
12. As a Product backend, I want `type` in `resource.action` form, so that I can switch on event types the same way I read Clerk.
13. As a Product backend, I want a delivery that returns 2xx to stop retries, so that a successful handler is not hit again for that attempt.
14. As a Product backend, I want a failed delivery to be retried with backoff, so that a brief outage does not drop the event.
15. As a Product backend, I want HTTPS-only endpoints, so that the thin payload and signing secret are not sent in the clear.
16. As a Product backend, I want a deleted subject's identifiers still present on `*.deleted` events, so that I can remove local rows after the API starts returning 404.
17. As a Product backend, I want events for writes that originated in Clerk ingest as well as REST, so that I do not care which path mutated Anchor.
18. As a Product backend, I want events when a product role or resource permission is created, updated, or deleted, so that I can refresh local RBAC without polling. Assign and unassign on a role emit `product.role.updated` only when the assignment changes.
19. As a Product backend, I do not want platform invitation events, so that Platform User onboarding stays out of my Product bus.
20. As EchoPoint, I want one endpoint URL and one signing secret in Product configuration for the tracer, so that I can ship without a CRUD API.
21. As EchoPoint, I want to verify deliveries with the same Svix/Standard Webhooks library used for Clerk, so that I do not write a custom verifier.
22. As an Anchor feature author, I want to call `Emit` inside `transactor.InTx` after my domain write, so that a rollback drops the event.
23. As an Anchor feature author, I want `Emit` to refuse a context with no transaction, so that I cannot accidentally send after commit.
24. As an Anchor feature author, I want to add a new event type by registering it and calling `Emit`, so that I do not fork signing or retry code.
25. As an Anchor feature author, I want an unknown event type to fail at emit time, so that a typo never reaches a Product.
26. As an operator, I want a durable queue job, so that an Anchor restart does not lose an event that already committed.
27. As an operator, I want delivery attempts to be visible, so that I can see why EchoPoint did not update.
28. As an operator, I want a 410 from the endpoint to stop further deliveries to that URL, so that a retired handler does not keep failing.
29. As an operator, I want a 429 or 503 with Retry-After to delay the next attempt, so that we do not hammer a throttled Product.
30. As an operator, I want 3xx not followed, so that a redirect cannot send events to an unexpected host.
31. As an operator, I want the signing secret unique per endpoint and stored encrypted, so that a leak of one Product does not forge another.
32. As a later Product, I want to register more than one endpoint and filter by event type, so that billing and the app do not share a handler. (Not in the tracer. The catalog and worker must not block this.)
33. As a later Product, I want an API to add, list, and remove endpoints, so that I do not file an ops ticket for a URL change. (Not in the tracer.)
34. As a Product backend, I want at-least-once delivery and no exactly-once promise, so that my handler is idempotent on `webhook-id`.
35. As a Product backend, I want events for workspace and Product User and Organization API key changes in the catalog even if the tracer does not emit them yet, so that adding those types is a registration, not a redesign.
36. As a security reviewer, I want endpoint URLs checked against internal addresses, so that a bad config cannot SSRF Anchor into its own network.
37. As a test author, I want to assert a signed POST against a WireMock Product URL, so that delivery is proven without EchoPoint running.
38. As a test author, I want to roll back a domain write and see no queue job, so that the outbox invariant stays locked.

## Implementation Decisions

- New subsystem `events` as its own package tree with an fx module, matching email and license (ADR-0007). HTTP handlers stay on the generated API struct if a later ticket adds endpoint CRUD. The tracer has no public endpoint-management API.
- Glossary: **event**, **event type**, **endpoint**, **delivery**, **thin payload**. Do not name this feature "webhooks". Inbound Clerk remains **integration webhook**.
- Wire format: Standard Webhooks (ADR-0017). Headers `webhook-id`, `webhook-timestamp`, `webhook-signature`. Body `{ "type", "timestamp", "data" }`. `type` is `resource.action`. `timestamp` is ISO-8601 UTC. `data` is identifiers only. No CloudEvents envelope. No Bearer token. `webhook-id` is a KSUID and is stable across retries of the same event.
- Tracer event types: `organization.created`, `organization.updated`, `organization.membership.created`, `organization.membership.updated`, `organization.membership.deleted`, `organization.license.updated`. License instantiate, adjust, and migrate all use `organization.license.updated`.
- Catalog boundary: Product SDK surface (organizations, members, workspaces, organization API keys, product users, licenses, product roles, product resource permissions). Platform invitations do not emit. Built-in Anchor product permissions (`product:settings:read` and similar) do not emit.
- Emit path: pgkit queue `EnqueueTx` on `transactor.CurrentTx`. Not `workflow.Start` (it opens its own transaction).

```
transactor.InTx(ctx, func(txCtx context.Context) error {
    // domain write
    return events.Emit(txCtx, Event{Type: "organization.created", Data: thinIDs})
})
```

`Emit` without a transaction returns an error.
- After commit, a worker fans out one delivery job per matching endpoint. Tracer: one endpoint, one delivery job. HTTP is not inside the domain transaction.
- Tracer configuration: one HTTPS URL and one signing secret per Product, set by operators, not a Product API. Later tickets add many endpoints with event-type filters on the Product API. The worker already filters by type so that ticket does not rewrite delivery.
- Signing secret is unique per endpoint, `whsec_` format, encrypted at rest like integration webhook secrets. Svix libraries already in the Clerk path verify and sign.
- Retry: reuse the existing pgkit worker shape (bounded attempts, exponential backoff, reap stuck jobs). Treat 2xx as success. Do not follow 3xx. Honour Retry-After on 429/503. Stop the endpoint after 410. At-least-once. Products dedupe on `webhook-id`.
- HTTPS only. Reject endpoint URLs that resolve to non-public addresses.
- Clerk ingest that writes a catalog resource emits the matching event types in the same transaction as the domain write. Tracer types fire only when those resources actually change.
- Stacked PRs if the diff would mix schema, worker, and the first emitters.

## Testing Decisions

- Test observable behaviour: a signed POST happens, a rollback drops the job, an unknown type is refused, a 2xx stops retry. Do not assert worker internals.
- Highest seam: component tests of the full app. WireMock stands in for the Product endpoint. Assert method, path, Standard Webhooks headers, JSON body shape, and that a second delivery of the same event keeps `webhook-id`. Verify the body with the Svix/Standard Webhooks library in the test. Prior art: Clerk ingest CTs (signature, idempotency, WireMock).
- Service integration tests for the outbox: real Postgres, `InTx` commit leaves a job, rollback leaves none, `Emit` without a transaction fails. Prior art: organization API key event worker IT; integration ingest `EnqueueTx` in the same transaction as the event row.
- Unit tests for thin `data` mapping and event-type catalog membership (pure, no HTTP, no DB).
- Third parties stay WireMock. Do not invent a fake dispatcher port only to unit-test delivery.
- EchoPoint as a running app is out of this spec's test suite. A later EchoPoint ticket owns the consumer.

## Out of Scope

- Organization-facing endpoints (a customer of EchoPoint registering a URL in Anchor)
- CloudEvents envelope or Bearer authentication
- Hosted or self-hosted Svix as the dispatcher
- pgkit workflow for emit or fan-out
- Pull Events API / cursor replay
- CloudEvents `OPTIONS` abuse handshake
- Endpoint CRUD API and multi-endpoint fan-out UI (worker must allow filters later; no API in the tracer)
- Emitting every catalog type in the first slice (workspaces, Product Users, Organization API keys wait)
- EchoPoint handler, cache invalidation, and SDK subscribe helpers
- Signing-secret self-service rotation API (operators can rotate config)
- Multi-day Svix-style retry calendars (use existing pgkit backoff first)

## Further Notes

Glossary and ADR live on branch `feat/product-events`: `CONTEXT.md` Product events section; ADR-0017. Research dump: `docs/research/outbound-webhooks-2026.md` (CloudEvents recommendation in that file is superseded by ADR-0017).

Next after this spec: `/to-tickets` into tracer-bullet issues with blocking edges, then implement from the unblocked frontier, one ticket per session.
