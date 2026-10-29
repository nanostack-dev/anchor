# Authorization engine

Access decisions come from OpenFGA ([ADR-0021](../adr/0021-access-decisions-come-from-openfga.md), [ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) E3 and E5). The move shipped in steps (ADR-0022 G4, milestone M2):

| Step | What it adds | Read by decisions? |
| --- | --- | --- |
| PR 4 | OpenFGA runs inside Anchor; each Product can get its own store | no |
| PR 5 | The catalog compiler writes each Product's model; the [syncer](authorization-fgasync.md) keeps its tuples in step with each write, with marks, a replay, a sweeper and a reconcile | no |
| PR 6 | Shadow: every decision runs on both evaluators and logs any difference | logged only |
| PR 7 (this) | Decisions come from OpenFGA; the old evaluator's permission check is removed | yes |

## The engine

`internal/authz/engine` serves OpenFGA's own API in-process (`openfgav1.OpenFGAServiceServer`), on Anchor's database:

- OpenFGA keeps its tables in the `openfga` schema, which migration `000068` creates. Its connections use that schema as their only search path, so jet generation and Anchor's own schema don't see its tables.
- **It starts beside the application, not before it.** A background loop runs OpenFGA's migrations and opens its server, retrying with a backoff of up to 30 s. Until it is up, `Provider.Get` answers `ErrNotReady`. A slow or failing OpenFGA migration therefore never stops Anchor from starting or serving; only the work that needs the engine waits.
- OpenFGA's migrations run under the `openfga-migrations` session advisory lock, because replicas start together and OpenFGA's migrator takes no lock of its own. A connection that can't release the lock is dropped, not returned to the pool.
- **Invalid indexes are rebuilt.** OpenFGA builds some indexes concurrently, and a rerun skips an index that exists even when an interrupted build left it invalid. After its migrations, still under the lock, the engine looks for invalid indexes in the `openfga` schema and rebuilds them with `REINDEX INDEX CONCURRENTLY`.
- OpenFGA logs warnings and errors as JSON through its own logger, so a datastore fault behind an internal error stays diagnosable.
- The check cache is off: a write must apply to the next decision, and a cached answer could outlive it.
- Callers depend on the service interface, so running OpenFGA as its own deployment later changes only this package.

### Connections

Each replica opens up to 10 OpenFGA connections, beside Anchor's own pool. Decisions, syncs and the sweeper share them. Size it against the database's `max_connections` across every replica, including the extra replica of a start-first deploy.

## Stores

Each Product gets one OpenFGA store, recorded in `authz_stores` (`product_id` → `store_id`). `store.Stores.StoreID` returns it and creates it on first use. It refuses to run inside a transaction (`ErrInTransaction`), because the OpenFGA store it creates commits on its own and a rollback would leave it unrecorded. When two replicas create one at once, the first insert wins and the others delete the store they made. After an insert that fails in a way that may still have committed, such as a cancelled request, it reads the row again and deletes its store only if that store is not the one recorded. Deleting a Product removes its row; its store is left for the sweeper to remove after its next reconcile pass.

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

Three sources of [`internal/authz/source`](../../apps/anchor/internal/authz/source) write a Product's tuples. Each owns its own tuples and syncs one key at a time:

| Source | Key | Fact in Anchor's tables | Tuples |
| --- | --- | --- | --- |
| `organization_assignments` | Organization *O* | User *U* holds role row *R* in *O* | `role_assignment:O.R # assignee @ user:U`, plus, once per role held in *O*, `role_assignment:O.R # role @ role:R` and `organization:O # assignment @ role_assignment:O.R` |
| `role_grants` | Role row *R* | *R* holds permission *P* | `role:R # grants_P @ user:*` |
| `organization_api_keys` | Organization *O* | Active, unexpired API key *K* of *O* holds *P* | `organization:O # can_P @ api_key:K` |

There is one assignment per role held in an Organization, with every member holding the role as an assignee. A check on the Organization therefore walks its roles, a handful, and looks up the subject among each one's assignees; its cost doesn't grow with the number of members. The ID holds the role, so the role tuple never changes: a member who changes role moves from one assignment's assignees to another's.

A role's grants have one owner, the role, whether it is managed or custom. So an Organization that lets a managed role go deletes only its own assignment links, and every other Organization holding that role keeps its grants.

## Keeping the store in step

Anchor's tables stay the source of truth (ADR-0022 E5). [`internal/authz/fgasync`](../../apps/anchor/internal/authz/fgasync) keeps every store in step with them; [the fgasync guide](authorization-fgasync.md) describes its interface, its rules and how to add a source.

1. **Mark.** Every write emits its event inside its transaction. `fgasync.Marker`, an event observer, asks each source which keys the event makes stale and upserts one row of `authz_sync_marks` per stale scope, in the same transaction: `(product, source, key)`, every key of a source (`key = '*'`) for a cascade, or the whole Product (`source = ''`) for a catalog change. A repeated mark bumps its revision.
2. **Replay after commit.** The API's strict middleware `NewAuthzReplayMiddleware` collects what a request marked and, once the handler returned and its transaction committed, calls `Syncer.Replay` before the response is written, so the next request already sees the change. A write outside an API request, such as a background job, is left to the sweeper.
3. **Sweep.** Every 5 s, `Syncer.Sweep` syncs the oldest marks.
4. **Reconcile.** Each pass rebuilds the one Product rebuilt longest ago, when that was more than 15 minutes back (`authz_stores.reconciled_at`). A Product never rebuilt comes first, with or without a store, so every Product gets its first syncer without waiting for a write. A rebuild deletes every tuple no source claims, such as the grants of a role deleted without an event, and, one page at a time, the stores of deleted Products. With many Products, each is rebuilt about once every *products × 5 s*.

Syncs take transaction-scoped advisory locks, waiting up to 3 s for a busy one. A key's sync holds the Product's lock (`authz-sync:<product>`) in share mode and its key's (`authz-sync:<product>:<source>:<key>`) exclusively, so writes to different Organizations or roles of one Product sync side by side. A model write, a whole-Product step and a rebuild hold the Product's lock exclusively. A key's sync that finds the catalog changed since the model writes the model in its own exclusive step first. It deletes a mark only if its revision did not move, and revisions come from one sequence, so a write landing during a sync is never lost, even when its mark was deleted and made again. Tuple deletes go before writes, so a sync cut short never leaves a subject with both an old and a new grant; a tuple already there or already gone is not an error, so a sync cut short is replayed whole. A failed replay leaves its mark for the sweeper. One replica sweeps at a time (`authz-sweep` session lock); a Product whose sync fails is postponed a minute behind the others, so it never starves them.

### Why a mark, not a tuple write

A write to Postgres and a write to OpenFGA cannot share one transaction. If Anchor wrote the tuple after its commit, a crash between the two writes would lose the tuple. If it wrote the tuple before its commit, a rollback would leave a tuple for a change that never happened. The mark is a Postgres row, so it commits or rolls back with the write that caused it. This is the transactional outbox pattern.

A mark records **which key to read again**, not what changed. A sync then makes the store match Postgres for that key (`Desired` → `fga.Diff` against `Stored` → `Apply`). Anchor does not push each event's delta, because:

- **Two writes can race.** A request removes a member and another adds them back. Both commit, but their deltas could reach the store in the other order and leave it wrong for good. A sync always ends on what Postgres holds.
- **Cascades emit no events.** Deleting a product user removes its memberships through `ON DELETE CASCADE`. A sync finds them gone; a delta would have nothing to push.
- **One function maps state to tuples.** Deltas would need one translator per event type, each kept in step with `Desired`.
- **A sync can run late or twice.** Order does not matter, many marks for one key collapse into one row, and the retry needs no stored payload.

The cost is one key read per access-changing write. For `organization_assignments` it grows with the Organization's members, and the engine is in-process, so it adds no network round trip. If it becomes slow, a narrower key (one member) keeps these properties; a delta does not.

### Walkthrough: adding a member

[`authorization-sync-flow.html`](authorization-sync-flow.html) steps through this flow table by table, row by row, with a second scenario for a write that lands during a sync. In short, for `POST …/organizations/org_acme/members` that adds Bob as `role_admin`:

| Step | Who | What it writes |
| --- | --- | --- |
| 1 | `NewAuthzReplayMiddleware` | Puts an empty list of marked scopes on the context (`fgasync.CollectMarks`) |
| 2 | `AddMember` | Checks the Organization, the product user, the role and that Bob is not a member, outside any transaction |
| 3 | `applyMembership`, request transaction | `INSERT organization_memberships`; `Emit` queues the webhook in `pgqueue_jobs`; the Marker upserts `authz_sync_marks (product, organization_assignments, org_acme)` at revision *n* and adds that scope to the list |
| 4 | `COMMIT` | The membership, the webhook job and the mark become visible together |
| 5 | The handler | Returns its `201` as a value; nothing is sent yet |
| 6 | `Syncer.Replay` | Syncs the scope with a 30 s limit that ignores a client disconnect |
| 7 | Sync transaction | Takes `authz-sync:<product>`, reads the marks (revision *n*), checks the model hash, reads Acme's members and the assignment tuples its store holds, and diffs them |
| 8 | OpenFGA, its own transaction | Writes `role_assignment:org_acme.role_admin # assignee @ user:bob` to `openfga.tuple` and `openfga.changelog`, and commits at once |
| 9 | Sync transaction | `DELETE authz_sync_marks … AND revision = n`, then `COMMIT`, which releases the lock |
| 10 | The middleware | Sends the `201`; the next request already sees Bob |

If step 6 to 9 fails, or the lock stays busy for more than 3 s, the error is logged and the client still gets its `201`. The mark stays, and the sweeper syncs it within 5 s. The OpenFGA write in step 8 is not part of the sync transaction: if the sync fails after it, the tuple stays, which is correct because it matches Postgres.

**A write during a sync.** Carol joins Acme after step 7 read Acme's members. Her transaction moves the mark to revision *n + 1* and commits; it does not wait, because the sync read the mark with a plain `SELECT` and an advisory lock blocks no row writes. Bob's sync writes only Bob's tuple, and its delete of revision *n* removes no row. Carol's replay waits for the lock, reads revision *n + 1*, writes her tuple and deletes the mark.

## Decisions

The access evaluator (`internal/access/service`) still resolves who the subject is in Postgres, then asks OpenFGA what it holds (`decision.Checker`, provided as `access/service.PermissionChecker`):

1. **Postgres resolves the subject.** A user is found by external ID, then its membership in the Organization; an API key goes through introspection, which refuses an invalid, inactive or expired key; a named workspace must belong to the Organization. A subject stopped at any of these steps is denied with its reason, and OpenFGA is not asked.
2. **OpenFGA answers each action.** `Checker.Holds` checks `can_<permission>` for `user:<product user>` or `api_key:<key>` on `organization:<id>`, through `store.Opened.Holds` (`BatchCheck`s of at most 50) within 5 s, under the model `authz_stores` records. A workspace decision is checked on its Organization, since a workspace's permissions are its Organization role's.
3. **A permission the model doesn't define is not held** (`MISSING_PERMISSION`) and is not asked: `store.Stores.Open` reads which relations the model defines, once per model ID, since a model never changes. OpenFGA reports an unknown relation with the same code as throttling, so any check it fails on fails the decision.
4. **A store answers once a full rebuild has filled it** (`authz_stores.reconciled_at`, `store.Opened.Rebuilt`). Until then the store may hold only the keys written since, and would deny the others. The first decision for a Product never rebuilt runs the rebuild, once per process and within 30 s, while the other decisions for that Product wait for it.

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

The component suite checks, once its tests pass and before any sweep, that OpenFGA holds what the tables imply: every member's role permissions and every active key's permissions are compared with OpenFGA's answer for every permission of the Product (`cmd/it/ct/authz_sync_check_test.go`).

## Known limits

- **API key expiry writes nothing.** A key that expires by time keeps its tuples until a rebuild. Decisions resolve a key through introspection in Postgres first, which refuses an expired key, so those tuples are never read.
- **A rebuild or a model write holds every key sync of its Product off.** A rebuild holds the Product's lock exclusively while it reads the whole store; writes in that Product wait at most 3 s, then leave their mark to the sweeper. Syncs of one key still run one at a time.
- **A catalog change replays its whole Product** before its response, bounded at 30 s; a batch of catalog writes, such as a Terraform apply, repeats it per write.
- **One rebuild per sweep.** The sweeper rebuilds one Product every 5 s, so each Product is rebuilt every 15 minutes only up to about 180 Products; beyond that, a rebuild comes around less often.
- **A deleted Organization's custom roles keep their grants until a rebuild.** Their rows cascade away without events, but no assignment links to them any more, so they grant nothing.

## Tests

- **Unit, no database.** `internal/authz/fga` tests tuples, the diff, batched and idempotent writes, paging and model relations against a real in-memory OpenFGA (`fgatest`). `internal/authz/fgasync` tests every sync rule through a toy source, with `fgasynctest`'s in-memory repository and lock: marks and revisions, the race with a write landing mid-sync, every-key and whole-Product scopes, a renamed source, a busy lock, model writes and the dropping of undefined relations, the replay, the sweeper's postponing, the reconcile and orphan stores. `internal/authz/source` checks each source's stale keys, its tuples, that the three own disjoint tuples of one Organization, and that every event type is classified. `internal/authz/model` tests the compiler.
- **Service, on Postgres.** `TestAuthzSync`, `TestAuthzReconcile`, `TestAuthzMarks` and `TestAuthzReplayAfterCommit` in `apps/anchor/cmd/it/service` run each source against Anchor's tables. `TestAuthzStores` covers the first store, concurrent creation and the deletion of the stores that lost, a missing Product, a lookup inside a transaction, and a check answered from a model and a tuple written through the engine.
- **Decisions.** `TestAuthzDecisions` covers a decision that follows the store (a deleted tuple denies until a reconcile repairs it), the first decision for a Product never rebuilt, a partly synced store, concurrent first decisions, an engine that isn't ready (503), an undeclared permission, and more actions than one `BatchCheck` takes. `TestAccessEvaluationSeesWritesOnTheNextCall` covers revocations only OpenFGA answers: a permission removed from a role, a member moved to another role, and a permission deleted from the catalog for an API key.
- **The component suite**, once its tests pass and before any sweep, compares every member's and active key's permissions in the tables with OpenFGA (`cmd/it/ct/authz_sync_check_test.go`).
