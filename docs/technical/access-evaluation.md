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

- **Subject:** `user` with the external (identity provider) user ID, or
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

## Freshness

Decisions come from the product's OpenFGA store ([authorization with OpenFGA](authorization-openfga.md)).
Postgres resolves the product user by external ID, or the API key by its credential, and the
organization name. Every other answer (the organization exists, the user is a member, the
workspace belongs to the organization, the roles and every permission held) is read from the
store. Each write that changes access leaves a mark in its own transaction, and the evaluator
replays the marks of the organization and product before it reads, so membership, role,
workspace and API-key writes apply to the next call, including Clerk ingest writes.

On a workspace, a product user holds the permissions of their organization role and of their
[workspace role](workspace-members.md) together; `context.workspace_role` names the second.

Evaluating an API key goes through introspection, so it flips an expired key to
inactive and updates `last_used_at` at most hourly, as introspection does.

## Not covered

A product user's
`INACTIVE` status is not checked, matching what membership listings return today.
