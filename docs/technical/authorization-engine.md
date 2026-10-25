# Authorization engine

Access decisions come from OpenFGA ([ADR-0021](../adr/0021-access-decisions-come-from-openfga.md), [ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) E3 and E5). The move shipped in steps (ADR-0022 G4, milestone M2):

| Step | What it adds | Read by decisions? |
| --- | --- | --- |
| PR 4 | OpenFGA runs inside Anchor; each Product can get its own store | no |
| PR 5 | The catalog compiler writes each Product's model; tuples follow each write, with marks, a sweeper and a reconcile | no |
| PR 6 | Shadow: every decision runs on both evaluators and logs any difference | logged only |
| PR 7 (this) | Decisions come from OpenFGA; the old evaluator's permission check is removed | yes |

## The engine

`internal/authz/engine` serves OpenFGA's own API in-process (`openfgav1.OpenFGAServiceServer`), on Anchor's database:

- OpenFGA keeps its tables in the `openfga` schema, which migration `000070` creates. Its connections use that schema as their only search path, so jet generation and Anchor's own schema don't see its tables.
- **It starts beside the application, not before it.** A background loop runs OpenFGA's migrations and opens its server, retrying with a backoff of up to 30 s. Until it is up, `Provider.Get` answers `ErrNotReady`. A slow or failing OpenFGA migration therefore never stops Anchor from starting or serving; only the work that needs the engine waits.
- OpenFGA's migrations run under the `openfga-migrations` session advisory lock, because replicas start together and OpenFGA's migrator takes no lock of its own. A connection that can't release the lock is dropped, not returned to the pool.
- **Invalid indexes are rebuilt.** OpenFGA builds some indexes concurrently, and a rerun skips an index that exists even when an interrupted build left it invalid. After its migrations, still under the lock, the engine looks for invalid indexes in the `openfga` schema and rebuilds them with `REINDEX INDEX CONCURRENTLY`.
- OpenFGA logs warnings and errors as JSON through its own logger, so a datastore fault behind an internal error stays diagnosable.
- The check cache is off: a write must apply to the next decision, and a cached answer could outlive it.
- Callers depend on the service interface, so running OpenFGA as its own deployment later changes only this package.

### Connections

Each replica opens up to 10 OpenFGA connections, beside Anchor's own pool. Decisions, syncs and the sweeper share them. Size it against the database's `max_connections` across every replica, including the extra replica of a start-first deploy.

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
| Role row *R* bundles permission *P* | `role:R # grants_P @ user:*` |
| Active, unexpired API key *K* holds *P* | `organization:O # can_P @ api_key:K` |

There is one assignment per role held in an Organization, with every member holding the role as an assignee. A check on the Organization therefore walks its roles, a handful, and looks up the subject among each one's assignees; its cost doesn't grow with the number of members. The ID holds the role, so the role tuple never changes: a member who changes role moves from one assignment's assignees to another's. An Organization sync reads the Organization's object, derives each linked assignment's role tuple from its ID, reads its assignees, and reads the roles the Organization owns or its members hold. It never deletes the grants of a role template a membership still holds itself, as a release before copies wrote, since other Organizations may hold it; a Product sync brings those in step.

## Keeping the store in step

Anchor's tables stay the source of truth (ADR-0022 E5):

1. **Mark.** Every write emits its event inside its transaction. `service.Marker`, an event observer, turns an event that changes access into a row of `authz_sync_marks` in the same transaction: the Organization, or the whole Product (`organization_id = ''`) for a catalog change or a deleted product user. A repeated mark bumps its revision.
2. **Replay after commit.** The API's strict middleware `NewAuthzReplayMiddleware` collects what a request marked and, once the handler returned and its transaction committed, syncs it before the response is written, so the next request already sees the change: each Organization it marked, or the whole Product after a catalog change. A write outside an API request, such as a background job, is left to the sweeper.
3. **Sweep.** Every 5 s, `service.Sweeper` replays the oldest marks: a Product sync when one is product-scope, else each Organization.
4. **Reconcile.** Each pass rebuilds the one Product rebuilt longest ago, when that was more than 15 minutes back (`authz_stores.reconciled_at`). A Product never rebuilt comes first, with or without a store, so every Product gets its first mirror without waiting for a write. A rebuild deletes every tuple no Organization implies, such as the grants of a deleted role, and, one page at a time, the stores of deleted Products. With many Products, each is rebuilt about once every *products × 5 s*.

Syncs take transaction-scoped advisory locks, waiting up to 3 s for a busy one. An Organization sync holds the Product's lock (`authz-sync:<product>`) in share mode and its Organization's (`authz-sync:<product>:<organization>`) exclusively, so writes to different Organizations of one Product sync side by side. A model write, the template grants and a rebuild hold the Product's lock exclusively. An Organization sync that finds the catalog changed since the model writes the model in its own exclusive step first. It deletes a mark only if its revision did not move, and revisions come from one sequence, so a write landing during a sync is never lost, even when its mark was deleted and made again. Tuple deletes go before writes, so a sync cut short never leaves a subject with both an old and a new grant; a tuple already there or already gone is not an error, so a sync cut short is replayed whole. A failed replay leaves its mark for the sweeper. One replica sweeps at a time (`authz-sweep` session lock); a Product whose sync fails is postponed a minute behind the others, so it never starves them.


## Decisions

The access evaluator (`internal/access/service`) still resolves who the subject is in Postgres, then asks OpenFGA what it holds (`service.Checker`, provided as `access/service.PermissionChecker`):

1. **Postgres resolves the subject.** A user is found by external ID, then its membership in the Organization; an API key goes through introspection, which refuses an invalid, inactive or expired key; a named workspace must belong to the Organization. A subject stopped at any of these steps is denied with its reason, and OpenFGA is not asked.
2. **OpenFGA answers each action.** `Checker.Holds` checks `can_<permission>` for `user:<product user>` or `api_key:<key>` on `organization:<id>`, in `BatchCheck`s of at most 50 and within 5 s, under the model `authz_stores` records. A workspace decision is checked on its Organization, since a workspace's permissions are its Organization role's.
3. **A permission the model doesn't define is not held** (`MISSING_PERMISSION`) and is not asked: the checker reads which relations the model defines on `organization`, once per model ID, since a model never changes. OpenFGA reports an unknown relation with the same code as throttling, so any item it fails on fails the decision.
4. **A store answers once a full rebuild has filled it** (`authz_stores.reconciled_at`). Before that it may hold only the Organizations written since, and would deny the others. The first decision for a Product never rebuilt runs the rebuild, once per process and within 30 s, while the other decisions for that Product wait for it.

Errors:

- The engine not being ready, a failed or slow check, an item OpenFGA fails on, or a rebuild that can't take its lock answers `503 ACCESS_DECISIONS_UNAVAILABLE`, logged at warn level with the request's fields; the caller retries.
- A Postgres failure answers 500, like the rest of the evaluator. A caller that hangs up is not logged.

The decision's context (`permissions`, the role and the key in the evaluation response) is still read from Anchor's tables. It describes the identity; the answer comes from OpenFGA. When the two disagree on an action, the evaluator logs `"event": "authz.decision_drift"`, sampled at 20 lines a minute. A write's own replay window can show one; a lasting one is a sync defect.

### Freshness

Decisions never look for marks (E5). A write that changes access usually reaches the store before its response, so the next request sees it. Otherwise the mark stays and decisions see the previous state until the sweeper replays it:

- the replay's lock stayed busy for 3 s, for example behind a rebuild of the same Product, or the process stopped: the next sweep, within 5 s;
- the write was made outside an API request, such as a Clerk ingest: the next sweep;
- the Product's sync is failing: the sweeper retries it every minute.

A mark replayed more than a minute after it was made is logged as `"event": "authz.mark_stale"`.

The component suite checks, once its tests pass and before any sweep, that OpenFGA holds what the tables imply: every member's role permissions and every active key's permissions are compared with OpenFGA's answer for every permission of the Product (`cmd/it/ct/authz_mirror_test.go`).

## Known limits

- **API key expiry writes nothing.** A key that expires by time keeps its tuples until a rebuild. Decisions resolve a key through introspection in Postgres first, which refuses an expired key, so those tuples are never read.
- **A rebuild or a model write holds every Organization sync of its Product off.** A rebuild holds the Product's lock exclusively while it reads the whole store; writes in that Product wait at most 3 s, then leave their mark to the sweeper. Writes in one Organization still sync one at a time.
- **A catalog change replays its whole Product** before its response, bounded at 30 s; a batch of catalog writes, such as a Terraform apply, repeats it per write.
- **One rebuild per sweep.** The sweeper rebuilds one Product every 5 s, so each Product is rebuilt every 15 minutes only up to about 180 Products; beyond that, a rebuild comes around less often. Writes that leave no mark rely on it: copies the role backfill and reads make (PR 2b) emit nothing. They hold their template's permissions, so the answers stay the same.

## Tests

`TestAuthzDecisions` covers a decision that follows the store (a deleted tuple denies until a reconcile repairs it), the first decision for a Product never rebuilt, a partly synced store, concurrent first decisions, an engine that isn't ready (503), an undeclared permission, and more actions than one `BatchCheck` takes. `TestAccessEvaluationSeesWritesOnTheNextCall` covers revocations only OpenFGA answers: a permission removed from a role, a member moved to another role, and a permission deleted from the catalog for an API key. `TestAuthzSync`, `TestAuthzReconcile`, `TestAuthzMarks`, `TestAuthzReplayAfterCommit` and `TestAuthzOrphanStores` cover each sync rule, the marks and their replay, and `internal/authz/model` has unit tests for the compiler and the tuples. `TestAuthzStores` in `apps/anchor/cmd/it/service` covers the first store, concurrent creation and the deletion of the stores that lost, a missing Product, a lookup inside a transaction, and a check answered from a model and a tuple written through the engine.
