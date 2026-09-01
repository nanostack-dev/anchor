# ADR-0017: Product events use Standard Webhooks

**Status:** Accepted

## Context

Anchor will push Product-scoped events to Product backends (EchoPoint first). Two envelopes were on the table: CNCF CloudEvents 1.0 (JSON `specversion` / `source` / `type` / `data`, webhook delivery authenticated with a Bearer token) and Standard Webhooks / Svix (HMAC of `id.timestamp.body`, headers `webhook-id` / `webhook-timestamp` / `webhook-signature`, body `{type, timestamp, data}`). Clerk already delivers inbound events to Anchor with Svix. A Bearer token does not bind the secret to the body.

## Decision

Outbound Product events speak Standard Webhooks, the same dialect Clerk uses. The body is `{type, timestamp, data}` with a thin `data` (identifiers only). `type` is `resource.action`. Headers are `webhook-id`, `webhook-timestamp`, `webhook-signature`. There is no Bearer token and no CloudEvents envelope.

## Considered Options

- **CloudEvents body + Standard Webhooks headers.** Two specs on one POST. The body would not match what Clerk sends or what the Svix libraries treat as the payload convention.
- **CloudEvents + Bearer.** Matches Auth0 and the CloudEvents HTTP webhook spec. Does not prove the body was unchanged. Rejected.

## Consequences

EchoPoint verifies with the same Standard Webhooks / Svix library already used for Clerk ingest. A later CloudEvents content type can be added; it is not the default.