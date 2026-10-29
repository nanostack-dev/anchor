# Authorization sync (fgasync)

The syncer keeps every Product's OpenFGA store in step with Anchor's tables (ADR-0022 E5). Anchor's tables stay the source of truth. A store holds the tuples they imply.

A module syncs its slice of the tables by writing one **source**: a small type that says which events make which keys stale, which keys exist, the tuples a key implies, and the tuples a key owns in the store. The syncer does everything else, once for every source: marks in the write's transaction, the replay after commit, the sweeper, the full rebuild, locking, model writes, retries and batching.

Read this guide to add a source, to change one, or to operate the sync. [`authorization-fgasync.html`](authorization-fgasync.html) shows the same framework visually: its layers, the contract, one sync step by step, who owns which tuples, and which marks each event leaves. [The authorization engine](authorization-engine.md) describes the engine, the stores, the model and Anchor's sources. [The sync flow](authorization-sync-flow.html) shows one write table by table.

```mermaid
flowchart LR
  W[Write service] -- Emit in tx --> E[events.Emit]
  E -- ObserveTx --> M[fgasync.Marker]
  M -- Stale --> P[Sources<br/>assignments · role grants · API keys]
  M -- upsert --> K[(authz_sync_marks<br/>product · source · key)]
  R[Replay middleware] -- after commit --> X[fgasync.Syncer]
  S[Sweeper 5 s · reconcile 15 min] --> X
  X -- Desired / Stored --> P
  X -- Diff · Apply --> F[(OpenFGA store)]
  X -- ClearIfUnchanged --> K
```

## Packages

| Package | What it is | Depends on |
| --- | --- | --- |
| [`fga`](../../apps/anchor/internal/authz/fga) | The generic OpenFGA surface: `Tuple`, `Diff`, `Store` (read every page, apply in batches, check), `Relations` of a model, `WriteModel` | OpenFGA's API only |
| [`fga/fgatest`](../../apps/anchor/internal/authz/fga/fgatest) | A real OpenFGA server on an in-memory datastore, for unit tests | `fga` |
| [`fgasync`](../../apps/anchor/internal/authz/fgasync) | The sync engine: `Source`, `Syncer`, `Marker`, `Scope`, and the Postgres adapters of its seams | `fga`, `events` |
| [`fgasync/fgasynctest`](../../apps/anchor/internal/authz/fgasync/fgasynctest) | In-memory adapters of the syncer's seams: `Repository`, `Locker`, `Stores`, `Engines` | `fgasync`, `fga` |
| [`source`](../../apps/anchor/internal/authz/source) | Anchor's sources and its model source | `fgasync`, `model`, Anchor's tables |
| [`model`](../../apps/anchor/internal/authz/model) | Anchor's OpenFGA vocabulary: type and relation names, and the catalog compiler | OpenFGA's language |

`fga` and `fgasync` know nothing of Anchor's domain. Only `source` and `model` do.

## Words

| Word | Meaning |
| --- | --- |
| Source | One slice of the tables, synced into every Product's store. It has a stable name, such as `organization_assignments`. |
| Key | The unit a source syncs: one Organization, one role. |
| Scope | What a mark makes stale: one key of one source, every key of a source (key `*`), or the whole Product (no source). |
| Mark | A row of `authz_sync_marks`: a stale scope and a revision. |
| Desired | The tuples a key implies, read from the tables now. |
| Stored | The tuples a key owns in the store now. |
| Sync | Make Stored equal Desired: write what is missing, delete what is left over. |

## The interface

```go
type Source interface {
	// Name identifies the source in marks. It must stay stable.
	Name() string
	// Stale lists the keys an event makes stale, or none.
	Stale(event events.Event) ([]string, error)
	// Keys lists every key the Product holds now.
	Keys(ctx context.Context, productID string) ([]string, error)
	// Desired lists the tuples the key implies now.
	Desired(ctx context.Context, productID, key string) ([]fga.Tuple, error)
	// Stored lists the tuples the key owns in the store.
	Stored(ctx context.Context, store fga.Reader, key string) ([]fga.Tuple, error)
}
```

The syncer also needs a `ModelSource`, which reads what a Product's model compiles from, and a `ModelChanges` function, which says which events change the model. In Anchor both come from the catalog (`source.Catalog`, `source.ChangesModel`).

Callers use five methods of `*fgasync.Syncer` and one of `*fgasync.Marker`:

| Call | Used by | Does |
| --- | --- | --- |
| `Marker.ObserveTx(ctx, event)` | `events.Emit`, inside the write's transaction | Marks every scope the event makes stale |
| `fgasync.CollectMarks(ctx)` | The replay middleware | Collects what a request marks |
| `Syncer.Replay(ctx, scopes)` | The replay middleware, after commit | Syncs what the request marked |
| `Syncer.Sync(ctx, scope)` | Replay, sweep, tests | Syncs one scope and clears the marks it covers |
| `Syncer.Reconcile(ctx, productID)` | The sweeper | Syncs the whole Product, then deletes every tuple no source claims |
| `Syncer.Sweep(ctx)`, `Syncer.Run(lc)` | The application | Runs the sweeper on one replica at a time |

## The rules a source must keep

1. **Own disjoint tuples.** Two sources, or two keys of one source, must never return the same tuple from `Stored`. If they do, each sync deletes the other's tuples. `TestOwnership` in `source` writes every source's tuples for one Organization and checks that each `Stored` returns its own and nothing else. Add your source to it.
2. **`Stored` covers `Desired`.** `Stored(key)` must return every tuple `Desired(key)` can return. A tuple that `Stored` cannot see is written again on every sync and never deleted.
3. **Keep the name stable.** The name is stored in marks. A mark that names an unknown source, for example after a rename, syncs the whole Product and is then cleared, so a rename is safe but costs one full sync per Product.
4. **`Stale` is pure.** It runs inside the write's transaction, on every event. It reads only the event: no query, no I/O. An error rolls the write back, so return one only for an event that breaks its own contract, such as a role event without a role ID.
5. **Use `fgasync.EveryKey` for a cascade.** When a change reaches many keys without naming them, such as a deleted product user whose memberships cascade away, return `[]string{fgasync.EveryKey}`.
6. **`Desired` reads the tables as they are now, scoped to the Product.** A key that no longer exists implies no tuple. Filter every query by the Product, so a key of another tenant implies nothing.
7. **`Keys` lists every key that exists.** The syncer adds the keys that only a mark still names, such as a deleted Organization, so `Keys` never needs to remember deleted ones.
8. **Nothing a source needs may emit events.** The `Marker` runs inside every write that emits an event, and fx builds it from the sources. A source that depends on a service that emits events creates a dependency cycle. Read tables through `source.Tables`, not through services.

The syncer does the rest:

- It drops each desired tuple whose relation the current model does not define. A permission deleted from the catalog therefore leaves its grants deleted, not rejected.
- It deletes before it writes, in batches of 100, and treats a tuple already there or already gone as done.
- It compiles a model once per hash per process, and writes it only when the hash differs from the one recorded in `authz_stores`.

## The replay middleware

The replay middleware syncs a write into OpenFGA **before** the client gets its response. A client that gets its 201 therefore sees the change on its next request: it reads its own writes. [`authorization-replay-middleware.html`](authorization-replay-middleware.html) shows the timeline with and without it.

Without it, only the sweeper syncs marks. An admin adds Ana to Acme and gets a 201. The browser loads Ana's page at once, the check asks OpenFGA, and Ana gets a 403 until the next sweep, up to 5 s later.

[`api.NewAuthzReplayMiddleware`](../../apps/anchor/internal/api/authz_replay.go) wraps every strict handler:

```go
collectCtx, marked := fgasync.CollectMarks(ctx) // 1. an empty basket in the context
response, err := f(collectCtx, w, r, request)   // 2. the handler writes and commits; the Marker adds org_acme to the basket
if err == nil {
	replayer.Replay(ctx, marked())              // 3. sync org_acme: read Postgres, diff, apply, clear the mark
}
return response, err                            // 4. only now the client gets its 201
```

- **It runs after the commit, because the sync must.** `Desired` reads Postgres on another connection, so it sees Ana's row only once the write commits. When the handler returns, its transaction has committed.
- **It syncs only what this request marked.** `Marker.ObserveTx` writes the mark row and also adds the scope to the request's basket. The middleware never reads `authz_sync_marks`, so it never syncs another request's marks.
- **No service calls a sync.** Every write that emits an access event gets the replay.

| Case | Result |
| --- | --- |
| The handler returns an error | The transaction rolled back with its mark. Nothing is replayed. |
| The replay fails (OpenFGA down, lock busy) | A warning is logged and the client still gets its response. The mark stays, so the sweeper retries it within 5 s. |
| The client disconnects during the replay | The replay ignores the cancellation (`context.WithoutCancel`) and finishes. |
| A catalog change marks the whole Product | The replay stops after 30 s. The sweeper finishes the rest. |
| The request changes no access | The basket is empty and `Replay` returns at once. |
| A background job writes, outside a request | No basket exists. The mark is still written, and the sweeper syncs it. |

The replay is the fast path and the sweeper is the safety net. A mark is cleared only by a sync that covers it, so a failed replay never loses a change.

## Adding a source

This example syncs which Organization owns each project, for a model where `rt_project` has `define parent: [organization]`. The table and the events are an illustration.

**1. Make sure the model defines the relation.** The syncer drops tuples the model does not define, so a missing relation shows up as tuples that never appear, not as an error.

**2. Write the source** in `internal/authz/source/project_parents.go`:

```go
// ProjectParents syncs which Organization each project sits in, so a check
// on a project walks up to its Organization's assignments. Keyed by project.
type ProjectParents struct{ tables Tables }

func NewProjectParents(tables Tables) ProjectParents { return ProjectParents{tables: tables} }

func (ProjectParents) Name() string { return "project_parents" }

func (ProjectParents) Stale(event events.Event) ([]string, error) {
	if isOneOf(event, events.ProjectCreated, events.ProjectMoved, events.ProjectDeleted) {
		return keyOf(event, events.FieldProjectID)
	}
	if isOneOf(event, events.OrganizationDeleted) {
		return []string{fgasync.EveryKey}, nil // its projects cascade away without events
	}
	return nil, nil
}

func (p ProjectParents) Keys(ctx context.Context, productID string) ([]string, error) {
	return p.tables.projectIDs(ctx, productID)
}

func (p ProjectParents) Desired(ctx context.Context, productID, projectID string) ([]fga.Tuple, error) {
	parent, err := p.tables.projectOrganization(ctx, productID, projectID) // Option: none once deleted
	if err != nil {
		return nil, err
	}
	organizationID, found := parent.Get()
	if !found {
		return nil, nil
	}
	return []fga.Tuple{{
		Object:   fga.Ref(model.ResourceType("project"), projectID),
		Relation: model.RelationParent,
		User:     model.Organization(organizationID),
	}}, nil
}

func (ProjectParents) Stored(ctx context.Context, store fga.Reader, projectID string) ([]fga.Tuple, error) {
	return store.Read(ctx, fga.Filter{
		Object: fga.Ref(model.ResourceType("project"), projectID), Relation: model.RelationParent,
	})
}
```

**3. Register it** in [`authz.NewModule`](../../apps/anchor/internal/authz/module.go):

```go
fgasync.AsSource(source.NewProjectParents),
```

**4. Classify its events.** Remove the events it now claims from `source.WithoutAccess()`. `TestEveryEventIsClassified` fails while an event type is both claimed and listed, or neither.

**5. Test it.**

- Add a `TestStale` case per event it claims, and a `TestTuples` case for any pure tuple builder.
- Add its tuples to `TestOwnership`, so it proves it owns disjoint tuples.
- Add a service test in `apps/anchor/cmd/it/service/authz_sync_test.go`: write the rows, `Sync` the key, and `Check` the store. Then change or delete the rows, sync again, and check that the access went.

No migration is needed: marks of every source share `authz_sync_marks`.

## Using `fga` directly

```go
store := fga.NewStore(engine, storeID, modelID)

tuples, err := store.Read(ctx, fga.Filter{Object: "organization:org_acme", Relation: "assignment"})
err = store.Apply(ctx, fga.Diff(stored, desired)) // deletes first, batched, idempotent
allowed, err := store.Check(ctx, "user:pu_ana", model.CanRelation("billing:write"), "organization:org_acme")
```

`fga.RelationsOf(model).Defines(tuple)` tells whether a model defines a tuple's relation. `fga.Ref`, `fga.Wildcard`, `fga.TypeOf` and `fga.IDOf` build and split references.

## Testing toolkit

| Helper | Gives |
| --- | --- |
| `fgatest.NewEngine(t)` | A real OpenFGA server on an in-memory datastore, closed when the test ends |
| `fgatest.StoreWithModel(t, engine, dsl)` | A new store at a model written in OpenFGA's DSL |
| `fgatest.Model(t, dsl)` | A compiled model |
| `fgasynctest.NewRepository(clock)` | Marks and store records in memory, with `AddProduct`, `RemoveProduct` and `Marks()` for assertions |
| `fgasynctest.NewLocker()` | A per-Product lock; `Hold` and `Release` make it busy from the test |
| `fgasynctest.NewStores(engine)` | One store per Product, named after it; `Store(t, repo, productID)` returns it at the recorded model |
| `fgasynctest.Engines{Engine: engine}` | An engine provider that is always ready |

`internal/authz/fgasync/syncer_test.go` builds a whole syncer from these and a toy source (documents and their owners). Copy its `givenWorld` to test the syncer's behaviour with a new kind of source.

## Operating the sync

| Setting | Value | Where |
| --- | --- | --- |
| Wait for a busy Product lock | 3 s, backoff 10 ms to 200 ms | `fgasync.go` |
| Replay limit after commit | 30 s, ignores a client disconnect | `sweeper.go` |
| Sweep interval, batch | 5 s, 100 oldest marks | `sweeper.go` |
| Reconcile interval | 15 min per Product, one Product per sweep | `sweeper.go` |
| Postpone after a failed sync | 1 min | `sweeper.go` |
| Compiled models kept per process | 256 | `model_cache.go` |

- **Locks.** `authz-sync:<product>` is a transaction advisory lock: commit or rollback releases it. `authz-sweep` is a session advisory lock, so one replica sweeps at a time.
- **Logs.** `authz replay after commit failed; the sweeper retries it`, `authz sync failed; it is retried after the other products` and `reconcile repaired the relationship store` (with write and delete counts) are the lines to watch. A reconcile that repairs tuples often means a source misses an event.
- **Force a resync of a Product.** Insert a whole-Product mark; the next sweep syncs it:

  ```sql
  INSERT INTO authz_sync_marks (product_id, source, scope_key)
  VALUES ('prod_tasks', '', '')
  ON CONFLICT (product_id, source, scope_key)
  DO UPDATE SET revision = nextval('authz_sync_mark_revisions'), marked_at = NOW();
  ```

- **Force a rebuild.** `UPDATE authz_stores SET reconciled_at = NULL WHERE product_id = 'prod_tasks';` makes it the next Product to reconcile.

## Why it is built this way

- **A mark says what to read again, not what changed.** [The engine doc](authorization-engine.md#why-a-mark-not-a-tuple-write) explains why: races, cascades without events, and one place that maps state to tuples.
- **The `Marker` is separate from the `Syncer`.** It runs inside every write, so it depends only on the sources and `ModelChanges`. The `Syncer` depends on the model source, which reads services that emit events; one type doing both would close a dependency cycle.
- **The syncer drops tuples the model does not define.** Sources then never need the catalog: after a permission is deleted, its tuples are simply no longer desired, and the sync deletes them.
- **One lock per Product, held per key.** A model write and a whole-Product sync must not race with a key's sync. Holding the lock per key, not per Product sync, keeps a write from waiting behind a whole Product.
- **Seams with two adapters each.** The repository, the lock, the stores and the engine each have a Postgres or OpenFGA adapter and an in-memory one. That is what lets `fgasync` test every rule in milliseconds, without Postgres.
- **No Go generic methods.** Go 1.27 allows them, but the `Source` interface works on strings and `fga.Tuple` only, so a type parameter would add a name without removing a cast.
