# Authorization engine

Access decisions come from OpenFGA ([ADR-0021](../adr/0021-access-decisions-come-from-openfga.md), [ADR-0022](../adr/0022-resource-types-scope-roles-and-permissions.md) E3 and E5). The move shipped in steps (ADR-0022 G4, milestone M2):

| Step | What it adds | Read by decisions? |
| --- | --- | --- |
| PR 4 | OpenFGA runs inside Anchor; each Product can get its own store | no |
| PR 5 | The catalog compiler writes each Product's model; tuples follow each write, with marks, a sweeper and a reconcile | no |
| PR 6 | Shadow: every decision runs on both evaluators and logs any difference | logged only |
| PR 7 | Decisions come from OpenFGA; the old evaluator's permission check is removed | yes |
| PR 13 (milestone M4) | Decisions on any resource object, with a contextual parent for an unregistered resource | yes |

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
| A registered [resource](resources.md) with row ID *X* of type *T*, under resource *Y* of type *S*, or under *O* itself | `rt_T:X # parent @ rt_S:Y`, or `rt_T:X # parent @ organization:O` |
| A [role assignment](resources.md#role-assignments): user *U* holds role row *R* on resource *X* of type *T* | `role_assignment:X.R # assignee @ user:U`, plus, once per role held on *X*, `role_assignment:X.R # role @ role:R` and `rt_T:X # assignment @ role_assignment:X.R` |

### Resource tuples

A resource object is `rt_<slug>:<Anchor row id>`, never the Product's identifier: that one is unique only in its Organization and type, and OpenFGA object IDs are at most 256 characters. Its tuples follow their own marks and syncs (`model.ResourceState`), separate from the Organization's, so a write to one resource never reads the rest of the registry:

- **Mark.** A `resource.*` or `resource.role_assignment.*` event marks its resource's object in `authz_resource_marks` (`product_id`, `object`, with the Organization for logs), with a revision from the same sequence as `authz_sync_marks`. That table keeps its shape, because the previous release upserts it during a rolling deploy.
- **Sync.** `Syncer.SyncResources(product, objects)` holds the Product's lock in share mode and each object's exclusively, so it runs beside Organization syncs and other resources' syncs: the rows of those objects give the desired tuples (the parent tuple and the role assignments while the row exists, nothing once it is gone), a read of each object, and of each assignment object it links or should link, gives the stored ones, the diff is applied deletes first, and the marks are cleared by revision. An assignment object holds the resource's row ID, so it belongs to that one resource and reading it costs one read per role held there. A parent tuple the model's `parent` relation would refuse is left out, as a permission the catalog no longer defines is; so is an assignment link on a type the model lacks. The role's grants are not part of it: they are the Organization's roles, synced with the Organization.
- **Replay and sweep.** The request's replay syncs the objects its writes marked; the sweeper replays the oldest resource marks, one resource sync per Product, with the same postpone after a failure and the same `authz.mark_stale` log.
- **Rebuild.** A reconcile implies every registered resource's tuples, so it writes any that a replay missed and deletes the tuples of resources that are gone, such as the resources of a deleted Organization, which cascade without events.

There is one assignment per role held in an Organization, or on a resource, with every member holding the role there as an assignee. `model.Assignment(object, role)` names it by the Organization's ID or the resource's row ID; both are prefixed KSUIDs, so the two never meet, and the Organization's assignment IDs are the ones earlier releases wrote. A check on the Organization therefore walks its roles, a handful, and looks up the subject among each one's assignees; its cost doesn't grow with the number of members. The ID holds the role, so the role tuple never changes: a member who changes role moves from one assignment's assignees to another's. An Organization sync reads the Organization's object, derives each linked assignment's role tuple from its ID, reads its assignees, and reads the roles the Organization owns or its members hold. It neither reads nor implies resource tuples. It never deletes the grants of a role template a membership still holds itself, as a release before copies wrote, since other Organizations may hold it; a Product sync brings those in step.

## Keeping the store in step

Anchor's tables stay the source of truth (ADR-0022 E5):

1. **Mark.** Every write emits its event inside its transaction. `service.Marker`, an event observer, turns an event that changes access into a row of `authz_sync_marks` in the same transaction: the Organization, or the whole Product (`organization_id = ''`) for a catalog change or a deleted product user. A repeated mark bumps its revision. The `resource.*` events mark their resource object instead ([Resource tuples](#resource-tuples)); they are never delivered to a product endpoint, but observers see them like any other.
2. **Replay after commit.** The API's strict middleware `NewAuthzReplayMiddleware` collects what a request marked and, once the handler returned and its transaction committed, syncs it before the response is written, so the next request already sees the change: each Organization it marked, or the whole Product after a catalog change. A write outside an API request, such as a background job, is left to the sweeper.
3. **Sweep.** Every 5 s, `service.Sweeper` replays the oldest marks: a Product sync when one is product-scope, else each Organization.
4. **Reconcile.** Each pass rebuilds the one Product rebuilt longest ago, when that was more than 15 minutes back (`authz_stores.reconciled_at`). A Product never rebuilt comes first, with or without a store, so every Product gets its first mirror without waiting for a write. A rebuild deletes every tuple no Organization implies, such as the grants of a deleted role, and, one page at a time, the stores of deleted Products. With many Products, each is rebuilt about once every *products × 5 s*.

Syncs take transaction-scoped advisory locks, waiting up to 3 s for a busy one. An Organization sync holds the Product's lock (`authz-sync:<product>`) in share mode and its Organization's (`authz-sync:<product>:<organization>`) exclusively, so writes to different Organizations of one Product sync side by side. A model write, the template grants and a rebuild hold the Product's lock exclusively. An Organization sync that finds the catalog changed since the model writes the model in its own exclusive step first. It deletes a mark only if its revision did not move, and revisions come from one sequence, so a write landing during a sync is never lost, even when its mark was deleted and made again. Tuple deletes go before writes, so a sync cut short never leaves a subject with both an old and a new grant; a tuple already there or already gone is not an error, so a sync cut short is replayed whole. A failed replay leaves its mark for the sweeper. One replica sweeps at a time (`authz-sweep` session lock); a Product whose sync fails is postponed a minute behind the others, so it never starves them.


## Decisions

The access evaluator (`internal/access/service`) still resolves who the subject is in Postgres, then asks OpenFGA what it holds (`service.Checker`, provided as `access/service.PermissionChecker`):

1. **Postgres resolves the subject and the object.** A user is found by external ID, then its membership in the Organization; an API key goes through introspection, which refuses an invalid, inactive or expired key; a workspace resolves to its resource of the Product's `workspace` type, or, without one, to its built-in row in the Organization ([access evaluation](access-evaluation.md#workspaces)); a resource of the Product's own types resolves to its registered row, or to the registered parent an unregistered one names ([access evaluation](access-evaluation.md#resources)). A subject or resource stopped at any of these steps is denied with its reason, and OpenFGA is not asked.
2. **OpenFGA answers each action on the object.** `Checker.Holds` checks `can_<permission>` for `user:<product user>` or `api_key:<key>` on the object `PermissionCheck.Object` names, in `BatchCheck`s of at most 50 and within 5 s, under the model `authz_stores` records. The object is `organization:<id>` for an Organization, and `rt_<slug>:<row id>` for a registered resource, a workspace's included. `PermissionCheck.Fallback` names a second object for the actions the first one's type doesn't hold: a workspace sets it to its Organization, so an Organization-scoped permission is decided there, as before workspaces were registered. A built-in workspace without a resource is checked on its Organization alone. Each resource type holds `can_<permission>` through its assignments or its parent, so a check on a document walks up its stored chain to the Organization, its API keys included.
3. **An unregistered resource is checked through a contextual tuple.** Its object is `rt_<slug>:unregistered`, which no stored tuple names, since stored objects carry prefixed row IDs. Each `BatchCheck` item carries the one tuple `rt_<slug>:unregistered # parent @ <parent object>`; OpenFGA reads it for that check only and never stores it.
4. **What the model doesn't define is not held** (`MISSING_PERMISSION`) and is not asked: the checker reads, once per model ID since a model never changes, the relations each type defines and the types its `parent` relation accepts. A permission the object's type doesn't hold, a type the model lacks (a catalog change not compiled yet), or a contextual parent its `parent` relation would refuse, is answered not held without a call, unless the check's fallback object defines it: OpenFGA would answer an error, and reports an unknown relation with the same code as throttling, so any item it fails on fails the decision.
5. **A store answers once a full rebuild has filled it** (`authz_stores.reconciled_at`). Before that it may hold only the Organizations written since, and would deny the others. The first decision for a Product never rebuilt runs the rebuild, once per process and within 30 s, while the other decisions for that Product wait for it.

Errors:

- The engine not being ready, a failed or slow check, an item OpenFGA fails on, or a rebuild that can't take its lock answers `503 ACCESS_DECISIONS_UNAVAILABLE`, logged at warn level with the request's fields; the caller retries.
- A Postgres failure answers 500, like the rest of the evaluator. A caller that hangs up is not logged.

The decision's context (`permissions`, the role and the key in the evaluation response, and `granted_by` on request) is still read from Anchor's tables. It describes the identity; the answer comes from OpenFGA. When the two disagree on an action of a decision checked on the Organization, the evaluator logs `"event": "authz.decision_drift"`, sampled at 20 lines a minute. A write's own replay window can show one; a lasting one is a sync defect. A resource decision, a registered workspace's included, is not compared: roles held on resources grant more than the Organization role lists.

### Depth

A check on the deepest chain the registry accepts, 15 levels counting the Organization (ADR-0022 A4), plus an unregistered resource one level below it, stays within OpenFGA's default resolution limit of 25, which the engine keeps. `TestAccessDecisionsOnResources` in `apps/anchor/cmd/it/service` checks both an Organization role, which walks the whole chain, and a role on the top resource.

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
- **A resource write's replay reads its own objects only.** Measured by `TestOrganizationSyncWithManyResources` (embedded engine on Postgres 18 in a local container, Apple M2 Max), with 10 and with 1,000 registered resources in the Organization: one registration's replay takes 6–8 ms at both sizes, and an Organization sync 5–6 ms at both, since it reads no resource. A cascade delete replays one object per deleted resource. An earlier design that read every resource object in each Organization sync took 0.35–0.42 s at 1,000 resources, about 0.4 ms each, and would have passed the 3 s lock wait near 7,000.
- **A deleted Organization's resource tuples wait for the rebuild.** Its resources cascade without `resource.deleted` events. The resources that held role assignments get a mark from their `resource.role_assignment.deleted` events, and their replay drops all their tuples; no resource mark names the others; no row names their objects either, so nothing reads them before the reconcile deletes them.
- **Member removals and cascades mark one object per resource.** A member who held roles on many resources, or a cascade over a large subtree, replays each of those objects before its response.
- **One rebuild per sweep.** The sweeper rebuilds one Product every 5 s, so each Product is rebuilt every 15 minutes only up to about 180 Products; beyond that, a rebuild comes around less often. Writes that leave no mark rely on it: copies the role backfill and reads make (PR 2b) emit nothing. They hold their template's permissions, so the answers stay the same. A copy a write makes (`organizationrole` `Resolve`) emits the internal `organization.role.copied`, which marks its Organization: a role assignment on a resource marks only the resource, and the copy's grants are the Organization's to sync.

## Tests

`TestAccessDecisionsOnResources` covers the deepest chain and the contextual parent. `TestAuthzDecisions` covers a decision that follows the store (a deleted tuple denies until a reconcile repairs it), the first decision for a Product never rebuilt, a partly synced store, concurrent first decisions, an engine that isn't ready (503), an undeclared permission, and more actions than one `BatchCheck` takes. `TestAccessEvaluationSeesWritesOnTheNextCall` covers revocations only OpenFGA answers: a permission removed from a role, a member moved to another role, and a permission deleted from the catalog for an API key. `TestAuthzSync`, `TestAuthzReconcile`, `TestAuthzMarks`, `TestAuthzReplayAfterCommit` and `TestAuthzOrphanStores` cover each sync rule, the marks and their replay, and `internal/authz/model` has unit tests for the compiler and the tuples. `TestResourceMirror` covers the marks and parent tuples of a registration, a move, a delete and a cascade, an Organization sync that leaves resource objects alone, the sweeper's replay of a resource mark, and the reconcile that removes a deleted Organization's resource tuples; `TestResourceEvents` in the component suite checks that a registration's replay moves the store. `TestAuthzStores` in `apps/anchor/cmd/it/service` covers the first store, concurrent creation and the deletion of the stores that lost, a missing Product, a lookup inside a transaction, and a check answered from a model and a tuple written through the engine.
