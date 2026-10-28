# Authorization engine

Access decisions move from Postgres queries to OpenFGA ([ADR-0021](../adr/0021-access-decisions-come-from-openfga.md), [ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) E3 and E5). The move ships in steps (ADR-0022 G4, milestone M2):

| Step | What it adds | Read by decisions? |
| --- | --- | --- |
| PR 4 | OpenFGA runs inside Anchor; each Product can get its own store | no |
| PR 5 (this) | The catalog compiler writes each Product's model; tuples follow each write, with marks, a sweeper and a reconcile | no |
| PR 6 | Shadow: every decision runs on both evaluators and logs any difference | logged only |
| PR 7 | Decisions come from OpenFGA; the old evaluator is removed | yes |

## The engine

`internal/authz/engine` serves OpenFGA's own API in-process (`openfgav1.OpenFGAServiceServer`), on Anchor's database:

- OpenFGA keeps its tables in the `openfga` schema, which migration `000068` creates. Its connections use that schema as their only search path, so jet generation and Anchor's own schema don't see its tables.
- **It starts beside the application, not before it.** A background loop runs OpenFGA's migrations and opens its server, retrying with a backoff of up to 30 s. Until it is up, `Provider.Get` answers `ErrNotReady`. A slow or failing OpenFGA migration therefore never stops Anchor from starting or serving; only the work that needs the engine waits.
- OpenFGA's migrations run under the `openfga-migrations` session advisory lock, because replicas start together and OpenFGA's migrator takes no lock of its own. A connection that can't release the lock is dropped, not returned to the pool.
- **Invalid indexes are rebuilt.** OpenFGA builds some indexes concurrently, and a rerun skips an index that exists even when an interrupted build left it invalid. After its migrations, still under the lock, the engine looks for invalid indexes in the `openfga` schema and rebuilds them with `REINDEX INDEX CONCURRENTLY`.
- OpenFGA logs warnings and errors as JSON through its own logger, so a datastore fault behind an internal error stays diagnosable.
- The check cache is off: a write must apply to the next decision, and a cached answer could outlive it.
- The engine starts with the application even though nothing reads it yet, so its migrations and datastore are proven in every environment first.
- Callers depend on the service interface, so running OpenFGA as its own deployment later changes only this package.

### Connections

Each replica opens up to 10 OpenFGA connections, beside Anchor's own pool. The pool is idle until something reads the engine. Size it against the database's `max_connections` across every replica, including the extra replica of a start-first deploy, before decisions read it (PR 6).

## Stores

Each Product gets one OpenFGA store, recorded in `authz_stores` (`product_id` → `store_id`). `store.Stores.StoreID` returns it and creates it on first use. It refuses to run inside a transaction (`ErrInTransaction`), because the OpenFGA store it creates commits on its own and a rollback would leave it unrecorded. When two replicas create one at once, the first insert wins and the others delete the store they made. After an insert that fails in a way that may still have committed, such as a cancelled request, it reads the row again and deletes its store only if that store is not the one recorded. Deleting a Product removes its row; its store is left for the reconcile (PR 5) to remove.

## The model

`internal/authz/model` compiles a Product's catalog, its resource types and its permissions with their scopes, into its model (ADR-0022 E3). Roles are data: the model never names one, so creating, changing or deleting a role writes tuples, never a model.

| Type | Relations |
| --- | --- |
| `user`, `api_key` | none |
| `role` | `grants_<permission>: [user:*]`, one per permission |
| `role_assignment` | `role: [role]`, `assignee: [user]`, `can_<permission>: assignee and grants_<permission> from role` |
| each resource type | `parent` (its parent types, itself when it nests in itself), `assignment: [role_assignment]`, and `can_<permission>` for every permission scoped to it or to a type below it: `can_<permission> from assignment or can_<permission> from parent` |
| `organization` | the same, with `[api_key] or` first, so an Organization API key holds a permission directly |

Relation names keep a readable part of the permission name and a hash of it, because permission names hold characters OpenFGA refuses (`:`, `*`) and relations are at most 50 characters. Every resource type but `organization` is named `rt_<slug>`, so no valid slug can shadow a fixed type, collide with another, or be a word of OpenFGA's language (`define`, `and`, `self`). The model carries no comment, so a permission name never reaches its text. The catalog's hash, recorded in `authz_stores.model_hash`, tells whether the catalog changed without compiling it; an unchanged catalog keeps its model. OpenFGA's model limit is raised to 1 MiB: a model grows by about half a kilobyte per permission and type that holds it.

## Tuples

An Organization implies:

| Fact in Anchor's tables | Tuples |
| --- | --- |
| A membership: user *U* holds role row *R* in Organization *O* | `role_assignment:O.R # assignee @ user:U`, plus, once per role held in *O*, `role_assignment:O.R # role @ role:R` and `organization:O # assignment @ role_assignment:O.R` |
| Role row *R* holds permission *P* | `role:R # grants_P @ user:*` |
| Active, unexpired API key *K* holds *P* | `organization:O # can_P @ api_key:K` |

There is one assignment per role held in an Organization, with every member holding the role as an assignee. A check on the Organization therefore walks its roles, a handful, and looks up the subject among each one's assignees; its cost doesn't grow with the number of members. The ID holds the role, so the role tuple never changes: a member who changes role moves from one assignment's assignees to another's. An Organization sync reads the Organization's object, derives each linked assignment's role tuple from its ID, reads its assignees, and reads the roles the Organization owns or its members hold. It never deletes the grants of a managed role, since other Organizations may hold it; a Product sync brings those in step. A custom role belongs to one Organization, so that Organization's sync keeps its grants in step.

## Keeping the store in step

Anchor's tables stay the source of truth (ADR-0022 E5):

1. **Mark.** Every write emits its event inside its transaction. `service.Marker`, an event observer, turns an event that changes access into a row of `authz_sync_marks` in the same transaction: the Organization, or the whole Product (`organization_id = ''`) for a catalog change or a deleted product user. A repeated mark bumps its revision.
2. **Replay after commit.** The API's strict middleware `NewAuthzReplayMiddleware` collects what a request marked and, once the handler returned and its transaction committed, syncs it before the response is written, so the next request already sees the change: each Organization it marked, or the whole Product after a catalog change. A write outside an API request, such as a background job, is left to the sweeper.
3. **Sweep.** Every 5 s, `service.Sweeper` replays the oldest marks: a Product sync when one is product-scope, else each Organization.
4. **Reconcile.** Each pass rebuilds the one Product rebuilt longest ago, when that was more than 15 minutes back (`authz_stores.reconciled_at`). A Product never rebuilt comes first, with or without a store, so every Product gets its first mirror without waiting for a write. A rebuild deletes every tuple no Organization implies, such as the grants of a deleted role, and, one page at a time, the stores of deleted Products. With many Products, each is rebuilt about once every *products × 5 s*.

Every sync runs under the Product's transaction-scoped advisory lock (`authz-sync:<product>`), waiting up to 3 s for another sync. It deletes a mark only if its revision did not move, and revisions come from one sequence, so a write landing during a sync is never lost, even when its mark was deleted and made again. Tuple deletes go before writes, so a sync cut short never leaves a subject with both an old and a new grant; a tuple already there or already gone is not an error, so a sync cut short is replayed whole. A failed replay leaves its mark for the sweeper. One replica sweeps at a time (`authz-sweep` session lock); a Product whose sync fails is postponed a minute behind the others, so it never starves them.

Nothing reads the store for decisions yet; PR 6 compares its answers with the current evaluator.

## Known limits

- **API key expiry writes nothing.** A key that expires by time keeps its tuples until a rebuild. Decisions resolve a key through introspection in Postgres first, which refuses an expired key, so the shadow (PR 6) never compares it, and the switch (PR 7) must keep that resolution.
- **One lock per Product.** Writes to different Organizations of one Product queue behind each other's replay, each waiting at most 3 s before leaving its mark to the sweeper. A rebuild holds the lock while it reads the whole store.
- **A catalog change replays its whole Product** before its response, bounded at 30 s; a batch of catalog writes, such as a Terraform apply, repeats it per write.

## Tests

`TestAuthzSync`, `TestAuthzReconcile`, `TestAuthzMarks`, `TestAuthzReplayAfterCommit` and `TestAuthzOrphanStores` cover each sync rule, the marks and their replay, and `internal/authz/model` has unit tests for the compiler and the tuples. `TestAuthzStores` in `apps/anchor/cmd/it/service` covers the first store, concurrent creation and the deletion of the stores that lost, a missing Product, a lookup inside a transaction, and a check answered from a model and a tuple written through the engine.
