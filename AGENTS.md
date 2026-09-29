# Anchor Agent Guide

Go OaaS core for hierarchy, identity, RBAC, and tenancy.

Shared cross-repo engineering rules: `docs/engineering-best-practices.md` (source of truth, kept identical with echopoint).

## Invariants

- Every exported service method validates its input via `nanostack-framework/pkg/validate` before any repository or transaction call.
- Business rules live in the service layer, never in SQL. New migrations must not add `CHECK` constraints or business triggers — the DB keeps PK/FK/UNIQUE/NOT NULL/defaults and the `updated_at` trigger only. One exception: time-series aggregation is delegated to TimescaleDB (`time_bucket`, continuous aggregates, retention and compression policies) — see `docs/adr/0005-timescaledb-for-usage-history.md`. Interpreting a series is still service-layer work.
- New subsystems get their own package tree with an fx module (`internal/email/`, `internal/license/`), not another file in the flat `internal/service` and `internal/repository` packages. API handler methods stay in `internal/api` because the generated `StrictServerInterface` is implemented by one struct. See `docs/adr/0007-first-feature-slice-in-anchor.md`.
- Tenant-facing paths stay tenant-scoped at the repository/service boundary. Methods that bypass tenant scope must be named `*Internal`, documented, and never called from tenant-facing handlers.
- Public IDs are KSUIDs.
- `Create`/`Update` repository methods return domain values, not pointers — re-query after update.
- OpenAPI enums are shared component schemas referenced by `$ref`, with `x-go-type`/`x-go-type-import` when mapped to domain types.
- A read that offers a related resource uses `?include=` — one shared enum parameter per aggregate, absent never means empty, one statement per included resource, and no derived data. See `docs/engineering-best-practices.md`.
- Product API keys are Anchor *management* credentials and keep the fixed `anchor_prd_apikey_` prefix. Configurable product-level prefixes apply only to organization API keys (`*_org_apikey_`).
- Contract first: update `openapi.yaml`, then regenerate through the repo command. Generated files are never hand-edited.
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
- Avoid comments — name variables and functions clearly instead. Comment only a genuinely complex algorithm.
- Product-scoped writes that change a catalogued resource must `events.Emit` inside the same `transactor.InTx` as the write. Catalogued resources: organization, membership, organization invitation, workspace, organization API key, product user, organization license, product role, product resource permission. Add the type to `internal/events` first. Clerk ingest is a product write: emit the matching product-user event. Cover the emit with a CT that configures an endpoint (`ProductContext.CaptureEvents`) and waits with `WaitFor`. `CaptureEvents` asserts Standard Webhooks on every captured delivery (signature, headers, envelope). No endpoint configured is not an error: the worker completes with an empty fan-out. Assign and unassign of a resource permission on a role emit `product.role.updated` only when the assignment actually changes.

## Pull requests

- Follow `.github/pull_request_template.md`. `gh pr create` starts from it. Keep every section, and fill each one in.
- The preview checkbox controls the preview environment. Select it to deploy a preview for the pull request. Clear it to destroy the preview.
- Never delete the `<!-- preview-deploy -->` marker on that line. CI finds the checkbox with the marker, not with the label text.
- CI reads the checkbox live on each build, so select it before you push. A checkbox selected later makes `preview-toggle.yml` re-run the full build, because the preview image is tagged from the merge commit of the build.
- A cleared checkbox starts the cleanup as soon as you save the description. A closed pull request always destroys the preview.

## Agent skills

### Issue tracker

GitHub Issues on `nanostack-dev/anchor`, driven through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — one root `CONTEXT.md` plus `docs/adr/`. See `docs/agents/domain.md`.

### Coding style

Anchor-specific Go/testing practices learned during review — reuse before building, verifying behavior before swapping in a replacement, comment discipline, test fixtures. Not synced with echopoint. See `docs/agents/coding-style.md`.
