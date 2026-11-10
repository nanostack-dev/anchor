# Anchor implementation rules

Go OaaS core for hierarchy, identity, RBAC, and tenancy.

All code-style paths and command working directories below are repository-relative.

Shared cross-repo engineering rules: `docs/engineering-best-practices.md` (source of truth, kept identical with echopoint).

## Layout and commands

- Backend Go module: `apps/anchor`. Run backend `go` commands there; the repository lint script resolves that module and the root configuration from any working directory.
- UI: `anchor-ui/` (pnpm workspace member; the lockfile is the root `pnpm-lock.yaml`). Go SDK: `clients/go/`.
- Contract: `apps/anchor/cmd/http/openapi.yaml`.
- Regenerate the oapi server, the Go SDK types, the go-jet models, the config and wiring files, and the `anchor-ui/src/client` client: `cd apps/anchor && ./generate_anchor.sh`.
  - Needs `go` and `pnpm` on PATH. The script no longer starts its own database.
  - Runs `go tool oapi-codegen` (server, then the two `clients/go` files), then `go tool nanostack gen` (db, repo, config and wire steps; `Postgres.Scopes` and the db image are declared in `apps/anchor/nanogen/project.go`), then `pnpm run openapi-ts` in `anchor-ui`.
  - `nanostack gen db` reads no database while the migrations, the image and the generated files match `internal/db/gen/anchor/nanogen.sum`. Otherwise it applies `apps/anchor/migrations` to a `timescale/timescaledb` container that it starts with Docker, or to the empty server that `NANO_GEN_DATABASE_URL` names. The first run after a framework bump changes a version line in `nanogen.sum`, so that run needs Docker.
  - CI runs `go tool nanostack gen -check -fresh` (`dag://anchor/generated-code`): it fails when a committed generated file does not match the migrations, the `nanorepo.Implement` declarations, the manifest or Config.
  - Run it from `apps/anchor`: every path in it is relative to that directory.
- Regenerate the Go SDK in `clients/go`: `make generate-client` from the repo root.
- Component tests (CTs): `cd apps/anchor && go test ./cmd/it/ct/... -count=1`. Needs Docker (testcontainers). Add `-run <TestName>` to narrow.
- CI runs `gotestsum --format github-actions -- ./...` in `apps/anchor/cmd/it/ct`, `apps/anchor/cmd/it/service`, and `apps/anchor/internal`. A new or changed CT also needs the shuffle and race commands under Invariants.
- Route security golden: `cd apps/anchor && UPDATE_ROUTE_SECURITY=1 go test ./internal/security/ -run TestContractSecurityIsUnchanged`. Run it only for a deliberate route-security change, and review the diff in `internal/security/testdata`.
- Lint: follow [Go lint](testing.md#go-lint) for iteration modes and required full lint before submission. Use `./scripts/lint.sh backend`, inspect its coverage/result/full log, and rerun after correcting the cause. `./lint-fix.sh` remains an explicit-fix compatibility command.
- UI checks, from `anchor-ui`: `pnpm check`, `pnpm typecheck`, `pnpm test`.
- Raw SQL in a CT: `testDB` (`*sql.DB`), declared in `apps/anchor/cmd/it/ct/license_shared_test.go` and wired in `shared_test.go`. Scope every statement to your own IDs.

## Invariants

- Every exported service method validates its input via `nanostack-framework/pkg/validate` before any repository or transaction call.
- Business rules live in the service layer, never in SQL. New migrations must not add `CHECK` constraints or business triggers — the DB keeps PK/FK/UNIQUE/NOT NULL/defaults and the `updated_at` trigger only. One exception: time-series aggregation is delegated to TimescaleDB (`time_bucket`, continuous aggregates, retention and compression policies) — see `docs/adr/0005-timescaledb-for-usage-history.md`. Interpreting a series is still service-layer work.
- New subsystems get their own package tree with a `nano.Module` (`internal/email/`, `internal/license/`), not another file in the flat `internal/service` and `internal/repository` packages. List the module in `apps/anchor/cmd/app/manifest.go`, and run `go tool nanostack gen` from `apps/anchor` after a change to a provider, a config field, or the method set of a repository bound with `nanorepo.Implement`: `cmd/app`'s tests fail on stale generated files. Background work is a type with `Start`, `Stop` or `Run` (`nano.Starter`, `Stopper`, `Runner`), never a goroutine started from a constructor. API handler methods stay in `internal/api` because the generated `StrictServerInterface` is implemented by one struct. See `docs/adr/0007-first-feature-slice-in-anchor.md`.
- Tenant-facing paths stay tenant-scoped at the repository/service boundary. Methods that bypass tenant scope must be named `*Internal`, documented, and never called from tenant-facing handlers. [Repositories](#repositories) gives the naming rules for a repository bound with `nanorepo.Implement`.
- Public IDs are KSUIDs.
- Log through log/slog with an injected `*slog.Logger` and a context (`logger.InfoContext(ctx, ...)`); zerolog and `pkg/log` are banned by depguard. Log an error from a request path with core/log's `log.ErrorTo(ctx, logger, err, msg, ...)` so the level follows the error: a cancelled request (a cancelled PostgreSQL statement included) logs at warn, a caller fault (a 4xx fault, a client search error) at debug, anything else at error.
- A collection of a persisted resource is a `POST .../search` that pages with a keyset cursor: `transactor.Page(...).Keyset(order)` with `order` from `transactor.StrictKeys(...)` (typed keys, the primary key as the tiebreak, an unmapped sort refused), a documented default order, and a `searchtest` suite in `TestCursorSearches`. Return a client search error (`searchx.IsClientError`: cursor errors and an unknown sort) unlogged from repositories and services; `log.ErrorTo` logs it once at debug. Never add a `GET` list or an `OrderBy` page. See [cursor-paginated searches](../technical/cursor-pagination.md).
- `Create`/`Update` repository methods return domain values, not pointers — re-query after update.
- A uniqueness pre-check is not race-free: a concurrent writer's row stays invisible until it commits. The repository write that can trip the constraint maps it to a repository sentinel with `OnUnique(<sentinel>, "<constraint or index name>")` on its `transactor.Query`, `QueryMap` or `Exec` result, naming every unique guard on the column (the `lower(name)` indexes from migration 000019 included), and the service turns the sentinel into the same 409 as the pre-check ([postmortem](../postmortems/2026-10-16-api-key-name-race-500.md): two same-millisecond API key creates on dev returned 500). Renames are such writes too. Add a case to `TestConcurrentSameNameWritesReturnConflict` (`apps/anchor/cmd/it/ct/name_unique_race_test.go`), or to `TestConcurrentUniqueInsertsReturnSentinel` (`apps/anchor/cmd/it/service/name_unique_race_test.go`) when no public route can reach the constraint concurrently. The license services predate this rule and match the violation in the service with `pgerr.IsUniqueViolation`. `TestEveryUniqueGuardMapsToADomainError` (`apps/anchor/cmd/it/service/unique_index_mapping_test.go`) fails on a new unique index or constraint that no write maps. If no request can reach that violation, add the guard to `uniqueGuardsWithoutMapping` with the reason.
- OpenAPI enums are shared component schemas referenced by `$ref`, with `x-go-type`/`x-go-type-import` when mapped to domain types.
- A read that offers a related resource uses `?include=` — one shared enum parameter per aggregate, absent never means empty, one statement per included resource, and no derived data. See `docs/engineering-best-practices.md`.
- Product API keys are Anchor *management* credentials and keep the fixed `anchor_prd_apikey_` prefix. Configurable product-level prefixes apply only to organization API keys (`*_org_apikey_`).
- Contract first: update `openapi.yaml`, then regenerate (see "Layout and commands"). Generated files are never hand-edited.
- Every create operation has a CT that sends only the contract's required fields and asserts each field the server defaults against the contract's values; enum values keep the contract's spelling in the domain ([postmortem](../postmortems/2026-11-04-product-user-status-enum.md)).
- HTTP error statuses follow `docs/engineering-best-practices.md`. Changing one is a client-visible change with no compile-time check: the anchor API suite is a set of echopoint flows in the database, so a status change passes build, lint, and every Go test and then fails the post-deploy `Echopoint flow suite (anchor)` job. In the same change, update the flow assertions and re-run `echopoint flows run --tag anchor --environment dev` against both the `prod` profile (the CI organization) and `dev`. The flows exist once per organization with different ids.
- New CTs go in the root `apps/anchor/cmd/it/ct` package, never a sub-folder, because each Go package pays a full container and app setup.
- CTs run in parallel. Every CT shares one app server, one database, and one Redis with the others, so write each CT so that it cannot see or change the data of another CT:
  - Call `t.Parallel()` as the first statement. Code before it runs in the serial phase.
  - Build your own fixtures (`createTestProductContext`, `itdsl.Given(t)`, `newLicenseWorld`). The default tenant behind `testOwnerClient` is shared: add to it, but never change or delete its tenant or owner.
  - Make every name, email, and slug unique (`ids.MustNew`, `itshared.Faker.UUID()`). A fixed value such as `invitee@example.com` collides with the same test in another run or with a copy of it.
  - Scope every read to your own IDs. Do not assert an exact total, or a before/after delta, on a list or table that other CTs also write. Filter by your product or IDs, or use `GreaterOrEqual`.
  - Scope every `testDB` statement with a `WHERE` on your own IDs. Never run an unscoped `UPDATE`, `DELETE`, or `TRUNCATE`.
  - Do not change process state: no `os.Setenv`/`t.Setenv`, `os.Stdout`, package variables, or clock overrides.
  - Wait for a result with `require.Eventually` or `EventSink.WaitFor` on your own row or event. Do not use a fixed `time.Sleep` for a positive result.
  - A CT stays serial only when it touches process-wide state: the shared mailpit inbox, the Clerk reconcile queue (any keyed Clerk instance), a full-range continuous-aggregate refresh or chunk drop, or a lock-timing assertion. Leave out `t.Parallel()` and put a `// Not parallel: <reason>.` line on the function. Go runs all serial CTs before the parallel ones start.
  - Subtests run in order by default. Add `t.Parallel()` to a subtest only when the subtest builds its own world and the parent has no shared setup. Then use `t.Cleanup`, not `defer`, in the parent.
  - Before you push a new or changed CT, run `go test ./cmd/it/ct/... -count=1 -shuffle=on` and `go test ./cmd/it/ct/... -count=1 -race -parallel 32` from `apps/anchor`.
- A self-rescheduling pgqueue job keeps itself singleton: each run removes duplicate pending copies, keeping the lowest id. A job that works on one resource (an integration instance, a product) holds a `pglock` advisory lock keyed on that resource, because queue retries and reaped jobs run copies in parallel. See [Clerk reconcile scheduler](../technical/clerk-reconcile-scheduler.md) and [ADR-0020](../adr/0020-self-rescheduling-jobs-collapse-duplicates.md).
- An idempotency key is not race-free either. When a duplicate is a success rather than a conflict, map the unique violation with `OnUnique` and, after the aborted transaction, re-read the committed row and return it, as webhook ingestion does on `(integration_instance_id, external_event_id)`. A write that derives a row's next state from its current one, such as an email template's draft and published pointers, reads that row with a row lock inside one transaction: `transactor.ForUpdate`, or `transactor.WithRowLock(txCtx, postgres.NO_KEY_UPDATE())` when other tables reference it by foreign key and the write leaves its key alone. See [architecture](../technical/architecture.md#service-operation-phases).
- Every CT scenario needs a plain-English `t.Run` name as defined in [component-test names](../engineering-best-practices.md#component-test-names). For other code, prefer clear names and comment only non-obvious behavior or algorithms.
- Product-scoped writes that change a catalogued resource must `events.Emit` inside the same `transactor.InTx` as the write. Catalogued resources: organization, membership, organization invitation, organization API key, product user, organization license, product role, organization custom role, product resource permission, product resource type, resource, resource role assignment. Add the type to `internal/events` first. A definition marked `Undelivered` (`resource.created|updated|deleted`) is never queued nor offered to an endpoint; cover it through its observer's effect, such as the relationship store, and assert that the endpoint receives nothing and that no delivery job is queued. Clerk ingest is a product write: emit the matching product-user event. Cover the emit with a CT that configures an endpoint (`ProductContext.CaptureEvents`) and waits with `WaitFor`. `CaptureEvents` asserts Standard Webhooks on every captured delivery (signature, headers, envelope). No endpoint configured is not an error: the worker completes with an empty fan-out. Assign and unassign of a resource permission on a role emit `product.role.updated` only when the assignment actually changes.
- An `events` TxObserver that takes a lock takes it after the locks of the observers whose type names sort before its own: the emitter runs them in type-name order on every replica, and two replicas taking the same locks in opposite orders deadlock ([postmortem](../postmortems/2026-10-31-tx-observer-order-deadlock.md)).

## Repositories

A repository is an interface and its implementation struct. `nanorepo.Implement` binds them to one go-jet table. The repo step of `go tool nanostack gen` derives each interface method that has no hand-written body from its name, and writes the method to `<file>_repo.gen.go`. The framework reference is [Repositories: `nanostack gen repo`](https://github.com/nanostack-dev/nanostack-framework/blob/v0.23.0-beta.3/docs/generation.md#repositories-nano-gen-repo) (v0.23.0-beta.3). The examples below come from `apps/anchor/internal/repository`. For the output of a derived method, see `apps/anchor/internal/email/repository/template_repository_repo.gen.go`.

- Each repository file has one `func _()` declaration. It binds the interfaces of that file, and it stays next to them (`internal/repository/tenant_repository.go`):

  ```go
  func _() {
  	nanorepo.Implement[TenantRepository, tenantRepositoryImpl](
  		table.PlatformTenants, tenantToDomain,
  	)
  }
  ```

  The implementation embeds the generated base of the table (`platformTenantsBase`) and sets it with `newPlatformTenantsBase(db)`.
- The mapper has no state and cannot fail. Write it as a function, such as `tenantToDomain`, which calls `(&mapper.TenantMapper{}).ToDomain(row)`. Do not use a package variable: lint refuses it (`gochecknoglobals`).
- Use `nanorepo.NoMapper[model.X]` when the mapper has state or can fail. `ProductUserRepository` uses it because `ProductUserMapper.ToDomain` returns `(user.ProductUser, error)`. `IntegrationInstanceRepository` uses it because `IntegrationInstanceMapper` holds a cipher and a logger. With `NoMapper`, the base returns model rows, so a find that returns a domain type does not derive. `DeleteBy`, `ExistsBy` and `Count` methods still derive.
- A derived name says every column that it filters on, joined by `And`. On a scoped table, the scope column comes first: `platform_tenant_id`, `product_id` or `organization_id` (`Postgres.Scopes` in `apps/anchor/nanogen/project.go`). Example: `FindByTenantAndProductAndSlug` on `email_templates` (`internal/email/repository`). `OrganizationRepository.FindByID` does not derive, because `organizations` is scoped by `product_id` and the name does not filter on it first.
- The `Internal` suffix lets only a read derive without a scope column, for example `FindAllInternal` or `CountInternal`. `-explain` and the generated comment mark that read `UNSCOPED`. The rule under [Invariants](#invariants) still applies: each method that bypasses tenant scope has the suffix, a doc comment, and no caller in a tenant-facing handler.
- A delete across tenants never derives, also with the `Internal` suffix. Write it by hand, so that review sees it, and give it the suffix. Example: `events.EndpointRepository.DeleteByProductIDInternal` in `internal/events/repository.go`.
- A find that returns one row derives only when its columns cover the primary key or a unique key. It reads `LIMIT 2` and returns `transactor.ErrMoreThanOneRow` when two rows match. A hand-written `LIMIT 1` on columns that are not unique can return a different row on each call. Replace it with `r.first` and an order that ends on a unique column. `platform_users` has no unique key on `(platform_tenant_id, user_id)`, so `PlatformTenantUserRepository.FindByTenantIDAndUserID` is renamed `FindByTenantAndUserID` and stays hand-written:

  ```go
  func (r *platformTenantUserRepositoryImpl) FindByTenantAndUserID(
  	ctx context.Context, tenantID string, userID string,
  ) (functional.Option[platform.User], error) {
  	return r.first(ctx,
  		r.where.PlatformTenantID(tenantID).AND(r.where.UserID(userID)),
  		r.t.CreatedAt.ASC(), r.t.ID.ASC(),
  	)
  }
  ```

- Before you write a body, run the repo step with `-explain` from `apps/anchor`. Pass the scope columns of `nanogen/project.go`, because the single step does not read them:

  ```sh
  go tool nanostack gen repo -dir internal/repository \
    -scopes platform_tenant_id,product_id,organization_id -explain UserRepository
  ```

  It prints how each method resolves. When a note says that the name also derives, compare the SQL in the note with the body. Delete the body only when both run the same query: the same filter, order and joins. A derived `FindAll` or slice find has no `ORDER BY`, so a body that orders its rows stays. Example: `TenantRepository.FindAll` stays hand-written, because login reads `tenants[0]` (`internal/service/auth_service.go`). The note on `FindByEmail` shows the same query as its body, so that body goes:

  ```text
  FindByEmail  hand-written user_repository.go:61
               note: the name also derives (SELECT * FROM users WHERE email = $1 LIMIT 2 -- at most one row: users_email_key (email); a second row is ErrMoreThanOneRow); the hand-written method wins
  ```

  When a note says that the name would not derive, it gives the reason. Fix the name, or keep the body on the base helpers (`r.findOne`, `r.first`, `r.findAll`, `r.where`).
- Do not give the implementation a method with the name of a base helper: `findOne`, `first`, `findAll`, `exists`, `count`, `deleteWhere`, `updateWhere`, `page`, `query*` or `exec*`. Your method hides the helper. Derived methods then call your method, and their SQL comment no longer shows the query that runs. `organizationAPIKeyRepository` and `productAPIKeyRepository` do this today: their `findOne` adds a join on the permissions table. Rename that helper (for example `findOneWithPermissions`) before a method of those repositories derives.
- A hand-written method that uses the base (`r.t`, `r.where` or a helper) on a scoped table with no predicate on a scope column gets a warning from `nanostack gen`, unless its name ends in `Internal`. Add the scope predicate. Do not ignore the warning: it does not fail the run.
- After you add, remove or rename a method, run `go tool nanostack gen` from `apps/anchor`. CI runs `go tool nanostack gen -check -fresh`, which fails on a stale `<file>_repo.gen.go`.
- In review, read the SQL comment of each derived method in `<file>_repo.gen.go`, and check that the implementation hides no base helper. In `repo_base.gen.go`, read only the predicate list (the `<table>Where` methods) of each table that the change binds. The generator writes the rest of that file in the same way for each table.

## Pull requests

- Before implementing a browser-visible feature or behavior change (including backend changes that affect a UI journey), read `.claude/skills/anchor-feature-review/SKILL.md`; it loads the repo-owned review procedure and binds it to Anchor's Playwright setup.
- Follow `.github/pull_request_template.md`. `gh pr create` starts from it. Keep every section, and fill each one in.
- The preview checkbox controls the preview environment. Select it to deploy a preview for the pull request. Clear it to destroy the preview.
- Never delete the `<!-- preview-deploy -->` marker on that line. CI finds the checkbox with the marker, not with the label text.
- CI reads the checkbox live on each build, so select it before you push. A checkbox selected later makes `preview-toggle.yml` re-run the full build, because the preview image is tagged from the merge commit of the build.
- A cleared checkbox starts the cleanup as soon as you save the description. A closed pull request always destroys the preview.
- `main` has no branch protection and auto-merge is disabled: wait with `gh pr checks <n> --watch`, then `gh pr merge <n> --squash --delete-branch`.
- A merge that touches `clients/go/**` auto-tags `clients/go/vX` through `release-client.yml`. Find your tag with `git tag --contains <merge-sha> 'clients/go/*'` after `git fetch --tags`, not by guessing the next version.

## Agent skills

### Agent workflow

Before implementing a feature or behavioral fix, read and follow the
[local agent workflow](agent-workflow.md)
for affected-area local E2E before pushing, complete CI verification, and the
documented cloud exception. Use this repository's guides and scripts for setup
and scenario selection.

### Issue tracker

GitHub Issues on `nanostack-dev/anchor`, driven through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — one root `CONTEXT.md` plus `docs/adr/`. See `docs/agents/domain.md`.

### Coding style

Anchor-specific Go/testing practices learned during review — reuse before building, verifying behavior before swapping in a replacement, comment discipline, test fixtures. Not synced with echopoint. See `docs/agents/coding-style.md`.

Replace magic numbers with named constants when the value carries domain meaning.
