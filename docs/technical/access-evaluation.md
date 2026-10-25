# Access evaluation

A product backend asks Anchor whether one caller may act on an organization or a
workspace, instead of rebuilding memberships, roles, workspace links and API-key
checks itself. EchoPoint is the first caller.

## Contract

`POST /v1/products/{product_id}/access/v1/evaluations` decides several actions in
one call; `POST /v1/products/{product_id}/access/v1/evaluation` decides one. Both
follow the [OpenID AuthZEN Authorization API 1.0](https://openid.net/specs/authorization-api-1_0.html)
shape. In a batch, a top-level `action` is the default for items without one and,
with no items, is evaluated alone. Per-item `subject` and `resource` overrides are
refused with 400 rather than ignored, so a request is never answered for a different
subject or resource than the caller named.

- **Subject:** `user` with the product user's external ID, or
  `organization_api_key` with the raw key. The request logger never records the
  body of these routes, introspection or key validation, even with
  `LOG_REQUEST_BODY=true`.
- **Resource:** `organization` or `workspace`. A user needs the organization: the
  organization ID, or `properties.organization_id` for a workspace. An API key
  defaults to its own organization when none is named.
- **Action:** a resource permission name such as `flows:read`. A batch with no
  items and no top-level action asks only whether the subject reaches the resource
  (an Anchor extension of AuthZEN).
- **Caller credential:** a product API key with `product_user:read` and
  `organization_api_key:read`.

A denial is HTTP 200 with `decision: false` and `context.reason`:
`SUBJECT_NOT_SYNCED`, `API_KEY_INVALID`, `API_KEY_INACTIVE`, `ORGANIZATION_NOT_FOUND`,
`NOT_A_MEMBER`, `WORKSPACE_NOT_IN_ORGANIZATION`, `MISSING_PERMISSION` (with
`missing_permissions`) or `NOT_EVALUATED` for items skipped by
`options.evaluations_semantic`. An unknown, foreign or deleted workspace gives the same
`WORKSPACE_NOT_IN_ORGANIZATION`. When the subject reaches the resource, `context`
carries the organization, the role, the product user or API key ID and every
permission held, allowed or not.

Errors are only for the call itself: 400 for an invalid request, 401 and 403 for the
caller's product API key, 500 for a failure.

## Who a user subject is

A user subject is the product user whose external ID equals `subject.id`, compared
exactly, within the calling product. A product user gets an external ID in one of
two ways ([ADR-0023](../adr/0023-a-product-can-be-its-own-identity-provider.md)):

- The product is its own identity provider. It sends `external_id` on
  `POST /v1/products/{product_id}/product-users`, usually its own account ID. In the
  Go SDK: `c.Users().Create(email).Name(name).ExternalID(accountID).Do(ctx)`. The ID
  is printable ASCII, at most 255 characters.
- A Clerk integration syncs the user. Clerk ingest writes the Clerk user ID.

A user created without an external ID cannot be named here until Clerk syncs it, and
any ID that no product user holds answers `SUBJECT_NOT_SYNCED`. No route changes an
external ID after it is set: a changed ID would give one person's access decisions to
another. The two sources share one set of IDs per product. A user created with a
Clerk user ID is the user that Clerk ingest then updates and deletes. Each Clerk event
replaces that user's email and name with Clerk's values and sets it active, whatever
the product sent on create.

## Freshness

The evaluator (`apps/anchor/internal/access`) resolves the subject in Postgres on
every call: the product user by external ID, the membership joined to its role,
and the workspace when one is named. A refusal for a missing membership adds one
organization read to tell `ORGANIZATION_NOT_FOUND` from `NOT_A_MEMBER`. What the
subject holds then comes from OpenFGA, in one in-process `BatchCheck` per 50
actions ([authorization engine](authorization-engine.md#decisions)). Nothing is
cached.

A write that changes access reaches OpenFGA before its response, so it applies to
the next call. A write made outside an API request, such as a Clerk ingest, or one
whose replay couldn't run, applies within 5 s, when the sweeper replays its mark.

Evaluating an API key goes through introspection, so it flips an expired key to
inactive and updates `last_used_at` at most hourly, as introspection does.

When OpenFGA can't answer, the call fails with `503 ACCESS_DECISIONS_UNAVAILABLE`
instead of guessing.

## Not covered

Workspace roles (`workspace_memberships`) are not read yet. A product user's
`INACTIVE` status is not checked, matching what membership listings return today.
