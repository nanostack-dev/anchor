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
   its `revision`. `organization_id` is `''` for role, permission and product user events.
2. `Authorizer.Resolve` calls `Syncer.Fresh`, which reads the marks for that organization
   and the product. With none, it reads from the store directly.
3. Otherwise it takes `pg_advisory_xact_lock('authz-sync:<product>')`, re-reads the marks,
   rebuilds the model if the catalog changed, diffs each named organization (all of them
   for a product-wide mark or a new store), applies deletes then writes (idempotent:
   `on_missing`/`on_duplicate` ignore), and deletes each mark only if its revision is
   unchanged. A write that lands during a sync keeps its mark.
4. The `Sweeper` does the same every 5 seconds for the oldest marks, and on start creates
   and fills the store of every product without one.

Each replica caches a product's catalog against the store's latest model ID, so a model
written by another replica invalidates it.

## Inspecting a store

The tables live in the `openfga` schema. To see a product's tuples:

```sql
SELECT object_type, object_id, relation, _user
FROM openfga.tuple
WHERE store = (SELECT store_id FROM authz_stores WHERE product_id = '<product_id>');
```

## Limits and follow-ups

- A product-wide mark resyncs every organization of the product under one lock.
- Deleting a product leaves its OpenFGA store behind (`authz_stores` cascades, the store does not).
- Resource-level sharing, folders and teams need model types the catalog does not describe yet.
