# Access evaluation

A product backend asks Anchor whether one caller may act on an organization or
one of its own resources, instead of rebuilding memberships, roles,
resource chains and API-key checks itself.

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
- **Resource:** `organization`, or any resource type slug the product declares, with the product's own ID ([resources](#resources)). A user needs the
  organization: the organization ID, or `properties.organization_id` for anything
  else. An API key defaults to its own organization when none is named.
- **Action:** a resource permission name such as `flows:read`. A batch with no
  items and no top-level action asks only whether the subject reaches the resource
  (an Anchor extension of AuthZEN).
- **Caller credential:** a product API key with `product_user:read` and
  `organization_api_key:read`, or the platform session of the Product's tenant, as
  the admin UI's access explainer uses.

A denial is HTTP 200 with `decision: false` and `context.reason`:
`SUBJECT_NOT_SYNCED`, `API_KEY_INVALID`, `API_KEY_INACTIVE`, `ORGANIZATION_NOT_FOUND`,
`NOT_A_MEMBER`, `RESOURCE_TYPE_NOT_FOUND`,
`RESOURCE_NOT_FOUND`, `RESOURCE_NOT_IN_ORGANIZATION`, `PARENT_TYPE_NOT_ALLOWED`
([resources](#resources)), `MISSING_PERMISSION` (with `missing_permissions`) or
`NOT_EVALUATED` for items skipped by `options.evaluations_semantic`. A
subject or resource that is not reached denies every item of a batch with the same
reason. When the subject reaches the resource, `context` carries the organization,
the organization role, the product user or API key ID and every permission the role
or key holds in the organization, allowed or not.

Errors are only for the call itself: 400 for an invalid request, 401 and 403 for the
caller's credential, 404 `PRODUCT_NOT_FOUND` for a platform session of another tenant,
500 for a failure.

## Resources

A Product's own resource types follow ADR-0022 B1 and E2. Postgres resolves the
resource after the subject, in this order; the first rule that fails is the reason:

1. **The type.** A slug the Product does not declare is `RESOURCE_TYPE_NOT_FOUND`.
   `organization` keeps its own path, and `properties.parent` on it is a 400.
2. **A registered resource** of that type and ID in the asked Organization is checked
   on its own object, under its stored chain. A `properties.parent` that differs is
   ignored and logged at warn level as `"event": "access.parent_mismatch"`, sampled at
   20 lines a minute: naming a parent never widens access to a registered resource.
3. **An unregistered resource with no parent** is `RESOURCE_NOT_FOUND`, even when
   another Organization of the Product registered that type and ID: IDs are unique
   per Organization and type (ADR-0022 B2), so that is a different resource, and
   naming it would tell a caller that the ID exists elsewhere.
4. **An unregistered resource with a parent.** The parent's type must be one of the
   resource type's parent types, or its own when it nests in itself, otherwise
   `PARENT_TYPE_NOT_ALLOWED` (a parent type the Product does not declare included).
   The parent is the asked Organization, `{ "type": "organization", "id": … }`, or a
   resource registered in it. Another Organization as the parent, or a parent
   registered only in another Organization, is `RESOURCE_NOT_IN_ORGANIZATION`, which
   reveals that the parent exists in another Organization of the same Product; a
   parent no Organization registered is `RESOURCE_NOT_FOUND`. The check then runs on an
   object of the resource's type that exists for that check only, with the parent
   given to OpenFGA as a contextual tuple
   ([authorization engine](authorization-engine.md#decisions)).

An ID registered in another Organization stops neither step 3 nor step 4: only the
parent's own Organization counts.

A role held on the resource, or on any resource above it, grants what it holds,
and so does the Organization role, the root of every chain. An Organization API key
holds its permissions on every resource of its Organization.

A permission scoped to a type above the resource's type, such as an
Organization-scoped `flows:read` asked on a `workspace`, or a `workspace`-scoped
permission asked on a project inside it, is answered through the resource's chain:
a role on the ancestor of that scope, or above it, grants it
([authorization engine](authorization-engine.md#the-model)). A role held on the
resource itself never holds it, since a role holds only permissions scoped to its
own type or below. A product can therefore ask every check on the resource it acts
on. A permission scoped to a type off the resource's chain, such as `invoice:pay` on
a document, is `MISSING_PERMISSION` and is not asked.

### Explaining a decision

`options.explain: true`, on the single or the batch route, adds `granted_by` to the
`context` of each allowed decision: on the single route in the response's
`context`, on the batch route in each allowed item's `context`. It is an Anchor
extension inside AuthZEN's free-form `context`, so an AuthZEN client ignores it.

```json
"granted_by": [
  { "role": { "id": "role_…", "name": "document-viewer" }, "resource": { "type": "document", "id": "e8a2f6c1…" } },
  { "role": { "id": "role_…", "name": "folder-viewer" }, "resource": { "type": "folder", "id": "3b7e91d2…" } },
  { "role": { "id": "role_…", "name": "member" }, "resource": { "type": "organization", "id": "org_…" } }
]
```

The list is read from Anchor's tables after OpenFGA allowed the action, not from the
store that decided it: every role
the user holds on the resource and its registered ancestors whose permissions include the
action, nearest first, then the Organization role when it holds the action. An API
key's own grant is `{ "api_key": { "id", "name" }, "resource": { "type":
"organization", "id": … } }`. A denied item carries none. Right after a write, while
it reaches the store, the list can briefly be empty or differ from the grant the
store used. When the tables can't be read, the decision is answered without
`granted_by` and logged at warn level as `"event": "access.explain_failed"`; an
explanation never turns a decision into an error.

Without the option nothing is read. With it, a call reads the resource's chain
(at most 15 resources, one statement) and the user's assignments on it (one
statement), whatever the number of actions.

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
A refusal for a missing membership adds one
organization read to tell `ORGANIZATION_NOT_FOUND` from `NOT_A_MEMBER`. A resource
of the Product's own types adds the type tree, the resource, and, for an
unregistered one, its parent: three reads at most, plus one when the named parent is
missing, to tell `RESOURCE_NOT_IN_ORGANIZATION` from `RESOURCE_NOT_FOUND`. What the subject holds
then comes from OpenFGA, in one in-process `BatchCheck` per 50 actions
([authorization engine](authorization-engine.md#decisions)). Nothing is cached.

A write that changes access, a role assignment or a resource move included,
usually reaches OpenFGA before its response, so it applies to the next call. A write made outside an API request, such as a Clerk
ingest, or one whose replay couldn't run, applies when the sweeper replays its
mark: within seconds, or a minute when the Product's sync is failing
([freshness](authorization-engine.md#freshness)).

Evaluating an API key goes through introspection, so it flips an expired key to
inactive and updates `last_used_at` at most hourly, as introspection does.

When OpenFGA can't answer, the call fails with `503 ACCESS_DECISIONS_UNAVAILABLE`
instead of guessing.

## Tests

`apps/anchor/cmd/it/ct/access_resource_evaluation_test.go` covers a role on a folder
allowing documents registered under it and under a sub-folder, and an unregistered
one named by its folder, but not the parent project or the Organization; the
Organization role and an API key on resources; each resource reason; a stored parent
winning over a named one; `granted_by`; batch semantics; revocations on the next
call; and an Organization-scoped permission asked on a document answered by the
Organization. `apps/anchor/cmd/it/ct/access_evaluation_test.go` asks every check on a
resource of a declared `workspace` type with Organization-scoped permissions, the way
a product that keeps its workspaces in Anchor does. `TestAnchorSDKWorkspaceResources`
in `apps/anchor/cmd/it/ct/anchorsdk_resources_test.go` covers the same through the SDK:
an Organization role reaching a workspace-scoped permission through the parent, a
role assigned on the workspace, and an Organization-scoped permission decided on the
Organization.
`TestAccessDecisionsOnResources` in `apps/anchor/cmd/it/service` checks a decision at
the bottom of a 15-level chain and on an unregistered resource one level below it.

## Not covered

A product user's
`INACTIVE` status is not checked, matching what membership listings return today.
