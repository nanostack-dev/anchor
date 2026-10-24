# Authorization with OpenFGA

Anchor decides access with [OpenFGA](https://openfga.dev), running inside the Anchor
process. The decision and its alternatives are in
[ADR-0021](../adr/0021-access-decisions-come-from-openfga.md); this page describes how it
works. Code: `apps/anchor/internal/authz`.

## Pieces

| Package | Role |
| --- | --- |
| `authz/engine` | Runs OpenFGA's migrations into the `openfga` schema, then serves `openfgav1.OpenFGAServiceServer` in-process on a small pgx pool. The check cache is off. |
| `authz/model` | Pure code: builds a product's model from its catalog, turns an organization's records into tuples, diffs stored and desired tuples. |
| `authz/repository` | Reads the catalog and an organization's records; keeps `authz_stores` (one OpenFGA store per product) and `authz_sync_marks`. |
| `authz/service` | `Marker` (event observer), `Syncer`, `Sweeper` and the fluent `Authorizer` the access evaluator calls. |

## Model

Built per product from `product_roles`, `product_role_resource_permissions` and
`product_resource_permissions`. Relation names keep a readable part and a hash of the
original, because permission names hold characters OpenFGA refuses (`flows:read` becomes
`can_flows_read_<hash>`, a role `role_<id>_<hash>`). For a catalog with a Viewer role
granting `flows:read`:

```text
type organization
  relations
    define product: [product]
    define member: [user]
    define role_viewer_…: [user]
    # flows:read
    define can_flows_read_…: [api_key] or role_viewer_…
type workspace
  relations
    define organization: [organization]
    define role_viewer_…: [user]
    # flows:read
    define can_flows_read_…: role_viewer_… or can_flows_read_… from organization
```

## Tuples

| Record | Tuple |
| --- | --- |
| Organization | `organization:<id>#product@product:<id>` |
| Membership | `organization:<id>#member@user:<product_user_id>` and `organization:<id>#<role>@user:<product_user_id>` |
| Workspace | `workspace:<id>#organization@organization:<id>` |
| Workspace membership | `workspace:<id>#<role>@user:<product_user_id>`, only while the user is a member of the organization |
| Active organization API key permission | `organization:<id>#<permission>@api_key:<key_id>` |

A role or permission the catalog no longer defines produces no tuple.

## Freshness

1. A write emits its product event inside its transaction (an existing rule). The
   `Marker` observer upserts `authz_sync_marks (product_id, organization_id)` and bumps
   its `revision`. The scope is the organization the event names, or one of two product
   scopes:
   - `#model` for a role or permission change: only the model is rebuilt, because tuples
     name roles, not permissions.
   - `#product` for a deleted role, permission or product user: tuples may linger in
     organizations no event named. They grant nothing (the model or the cascade already
     removed what they meant), so only the sweeper replays this scope.
2. `Authorizer.Resolve` asks `Syncer` for a view. With no mark for the organization or
   the model, and the store's latest model equal to the one built from the current
   catalog, it reads the store directly.
3. Otherwise it tries `pg_try_advisory_xact_lock('authz-sync:<product>')`. When another
   sync holds it, the decision waits outside any transaction (10 to 200 ms backoff) and
   looks again, so waiters hold no pooled connection. Under the lock it re-reads the
   marks, rebuilds the model if the catalog changed (models are written only here, so an
   older catalog can never become the latest), diffs each named organization, applies
   deletes then writes (idempotent: `on_missing`/`on_duplicate` ignore), and deletes each
   mark only if its revision is unchanged and no write holds it (`FOR UPDATE SKIP LOCKED`).
4. A sync runs detached from the request (`context.WithoutCancel`, 30 s bound), so a
   client that hangs up does not roll back work the next caller would redo. A new store
   is recorded in `authz_stores` in its own transaction; a store left by a sync that
   failed before recording it is found by name and adopted.
5. The `Sweeper`, every 5 seconds on each replica: fills the store of products that have
   none, replays the oldest marks (product scope included), and rewrites in full the one
   product whose `reconciled_at` is older than 15 minutes. The full rewrite repairs drift
   from writes that left no mark, such as a SQL data fix or an API key that expired.

A decision checks every catalog permission in batches of 50, OpenFGA's limit. Role names
in the decision come from Postgres, so a rename shows on the next call on every replica.

## Inspecting a store

The tables live in the `openfga` schema. To see a product's tuples:

```sql
SELECT object_type, object_id, relation, _user
FROM openfga.tuple
WHERE store = (SELECT store_id FROM authz_stores WHERE product_id = '<product_id>');
```

## Limits and follow-ups

- **Model size grows as roles × permissions.** Each permission relation lists the roles
  that grant it, on two types. With every role granting every permission, about 60 roles
  × 35 permissions reaches OpenFGA's 256 KB model limit, and every role edit writes a new
  model version that is never deleted. The alternative is to store role grants as tuples
  (OpenFGA's custom-roles pattern), which keeps the model constant but needs a per-scope
  permission object, so a grant change writes tuples in every organization using the role.
- **Decision cost.** A decision checks every catalog permission so it can report them all;
  with tens of permissions and roles that is hundreds of datastore reads. Checking only the
  requested actions, and computing the reported list another way, would cut it.
- Deleting a product leaves its OpenFGA store behind (`authz_stores` cascades, the store does not).
- Resource-level sharing, folders and teams need model types the catalog does not describe yet.
