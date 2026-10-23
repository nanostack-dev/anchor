# ADR-0021: Access decisions come from OpenFGA, run inside Anchor

**Status:** Proposed, 2026-10-23 (proof of concept on `poc/openfga-authz`)

## Context

Access evaluation ([access evaluation](../technical/access-evaluation.md)) answered from Postgres: one membership row and its role's permissions, plus a check that the workspace belongs to the organization. Workspace roles (`workspace_memberships`) existed in the schema but nothing read them. Products want more than one role per organization: a role per workspace now, and later sharing a single resource with one person, nested folders and teams. Each of these is another hand-written rule in the evaluator, and every product would have to agree with Anchor's reading of it.

Zanzibar-style systems express all of them as relationship tuples and a model that says how relations combine. OpenFGA is an open-source implementation, written in Go, that runs on Postgres and can be embedded as a library (`pkg/server`, with `pkg/storage/migrate` exposed for embedding).

## Decision

Every access decision is read from an OpenFGA store, one per product, served by OpenFGA running in the Anchor process on Anchor's Postgres (schema `openfga`). Postgres stays the source of truth for organizations, members, workspaces, roles, permissions and API keys; the store mirrors them.

- **Model per product.** Anchor builds each product's model from its catalog. A role is a relation assignable on an organization and on a workspace. A permission is a computed relation: on an organization it is held through a role that grants it or directly by an API key; on a workspace through a workspace role that grants it or through the organization. A member's permissions in a workspace are therefore their organization role's and their workspace role's together.
- **Writes leave marks, not OpenFGA calls.** Every write already emits a product event inside its transaction. An in-transaction observer turns the events that change access into a mark naming the organization, the product's model (a role or permission change), or the whole product (a deletion whose leftover tuples grant nothing). The mark commits or rolls back with the write.
- **Decisions replay marks first.** Before answering, the evaluator replays the marks of the organization and of the model, under a per-product try-lock, waiting outside any transaction when another sync holds it. Models are written only under that lock. A background sweeper replays the rest, fills the store of a product that has none, and rewrites one product in full every 15 minutes to repair drift. A sync is a diff between the tuples Postgres implies and the tuples the store holds, so replaying it twice changes nothing.
- **The evaluator keeps its contract.** The AuthZEN request and response are unchanged except for a new `workspace_role` in the decision context. Postgres still resolves who asks (a product user's external ID, an API key's credential) and the display names.

## Considered Options

- **Keep evaluating in Postgres and add workspace roles to the query.** Smallest change. Rejected for this experiment because each later rule (resource sharing, folders, teams) becomes more SQL that every product depends on.
- **Run OpenFGA as its own service.** The documented deployment. Rejected for now: it adds a service to deploy, monitor and upgrade in every environment, including PR previews. The evaluator depends only on `openfgav1.OpenFGAServiceServer`, so moving to a remote server later changes one package.
- **SpiceDB embedded (`pkg/embedded`).** It runs Check in-process but offers no relationship writes, schema writes or lookups, which the sync needs.
- **Write tuples directly from each service.** Simpler to read, but there are many write paths (services, invitation acceptance, Clerk ingest, cascades), and a missed one leaves the store wrong with no signal. Marking from the events every write must already emit covers them all, transactionally.

## Consequences

- The first decision after a write pays for the sync of that organization; after a role or permission change, for a model rebuild only.
- The model grows as roles × permissions and reaches OpenFGA's 256 KB limit around 60 roles that each grant 35 permissions. Storing role grants as tuples avoids that at the cost of fan-out writes; see [authorization with OpenFGA](../technical/authorization-openfga.md#limits-and-follow-ups).
- A decision checks every catalog permission so it can report them all, which costs hundreds of datastore reads for large catalogs.
- The binary carries OpenFGA and its dependencies, and Anchor owns OpenFGA's migrations in the `openfga` schema.
- An organization API key that expires without a write keeps its tuples until the next reconcile. Introspection refuses it first, so no decision reaches the store.
- The check cache is off: a cached answer could outlive a write. Turning it on needs invalidation tied to the marks.
