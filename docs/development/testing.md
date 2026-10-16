# Testing and generation

Use [agent workflow](agent-workflow.md) for affected-area local E2E before pushing behavior changes and current-head CI as the complete final gate. Browser fixtures, coverage and responsive review remain owned by [anchor-ui/e2e/guide](../../anchor-ui/e2e/guide/README.md) and the [local review adapter](../../.claude/skills/anchor-feature-review/SKILL.md).

| Check | Working directory and command |
| --- | --- |
| Focused component test (Docker/testcontainers) | `apps/anchor`: `go test ./cmd/it/ct/... -run '<TestName>' -count=1` |
| New/changed CT parallelism | `apps/anchor`: `go test ./cmd/it/ct/... -count=1 -shuffle=on`, then `go test ./cmd/it/ct/... -count=1 -race -parallel 32` |
| Service/package selection | `apps/anchor`: `go test ./cmd/it/service/... -count=1` or the affected `./internal/<package>/...` |
| Backend lint | `apps/anchor`: `golangci-lint run --config ../../.golangci.yml --timeout 5m`; root `lint-fix.sh` applies fixes |
| Backend build | `apps/anchor`: `go build ./...` |
| UI gates | `anchor-ui`: `pnpm check`, `pnpm typecheck`, `pnpm test`, `pnpm build`, `pnpm test-storybook` |
| Affected ordinary browser journey | `anchor-ui`: `pnpm test:e2e:app <spec>`; inspect `test:e2e:affected`/selection guidance before using its planner |
| Complete app verification | `anchor-ui`: `pnpm test:e2e:verify --base <actual-PR-base-ref>` |
| Responsive browser review | `anchor-ui`: `pnpm test:e2e:review <spec>`; bootstrap + mobile/tablet/desktop, per local adapter |

Read [implementation rules](agent-rules.md) for parallel CT isolation and [engineering practices](../engineering-best-practices.md) for test SDK ownership. A root-directory lint that prints `0 issues` is not a module check. UI `build` does not check types.

The Go verification workflow runs for pull requests against any base branch, including stacked parent branches, so every layer receives `ci-ok` on its own head. Change detection still selects backend/frontend jobs. Preview flag/upsert, preview toggle and preview teardown remain limited to a `main` PR base; main push deployment conditions are unchanged.

## Regenerate contracts and database models

1. Change `apps/anchor/cmd/http/openapi.yaml` before generated server/client code. Schema changes are migrations, never hot database edits.
2. From `apps/anchor`, run `./generate_anchor.sh` (Docker, migrate, Go, pnpm and openssl required). It generates the server, go-jet models and frontend client using a throwaway TimescaleDB on port 25895.
3. From the repository root, run `make generate-client` for the separate `clients/go` SDK. Verify intended generated diffs and relevant builds/tests.
4. Update the route-security golden only for an intentional policy change: from `apps/anchor`, `UPDATE_ROUTE_SECURITY=1 go test ./internal/security/ -run TestContractSecurityIsUnchanged`, then review `internal/security/testdata`.

## Deployed API suite and CI

Status/error contract changes also update the Echopoint flow assertions of the `anchor` suite. It lives only in the prod echopoint organization (CLI profile `prod`) and targets the dev APIs through its `dev` environment; the dev organization does not copy it. Follow the [flow-suite procedure](flow-suite-testing.md) and run `echopoint --profile prod flows run --tag anchor --environment dev`. The post-deploy gate runs the same selection.

[Go Build and Test](../../.github/workflows/go.yml) selects backend/frontend verification through reusable ci-workflows and reports `ci-ok`. Its `migration-lint` job (part of `ci-ok`) lints the files in `apps/anchor/migrations` that a pull request adds or modifies with squawk and ci-workflows' custom checks; its rules, exclusions and the `-- lint: allow-backfill <reason>` opt-out are in [ci-workflows migration lint](https://github.com/nanostack-dev/ci-workflows/blob/main/docs/technical/migration-lint.md). To run it locally, install the pinned squawk with `install-squawk.sh` from a ci-workflows checkout and run `migration_lint.py --migrations-dir apps/anchor/migrations --pg-version 18 --base origin/main` (both under `scripts/migration-lint/`). [Isolated browser CI](../../.github/workflows/e2e-app.yml) runs the managed app gate and cleanup for its selected paths; deployed smoke and infra post-deploy suites are separate boundaries. Preserve the PR preview marker and distinguish executed, skipped, blocked and unknown checks. Documentation-only changes need link/command provenance checks, with CI selection reported accurately.

## Cursor-paginated searches

Every search has a case in `TestCursorSearches` (`apps/anchor/cmd/it/ct/cursor_search_test.go`). A case seeds more than two pages of rows, ties their sort column with one `UPDATE` scoped to the case's own ids, and gives the SQL that lists the same ids in the documented order. The shared walker then follows `next_cursor` three rows at a time and asserts the rows, the total, the missing `next_cursor` on the last page, and `400 INVALID_CURSOR` for a malformed cursor, a cursor with an offset and a cursor issued for another sort. Add a case when you add a search; do not write a second walker. Run it with `go test ./cmd/it/ct/... -run TestCursorSearches -count=1` from `apps/anchor`. The repository suite has cursor walks over a composite key, a nullable key and a plain table in `cmd/it/repository/cursor_pagination_repository_test.go`, which runs manually. See [cursor-paginated searches](../technical/cursor-pagination.md).

## Concurrent same-name writes

`TestConcurrentSameNameWritesReturnConflict` (`apps/anchor/cmd/it/ct/name_unique_race_test.go`) covers every public create or rename that pre-checks a unique name, email, slug or key. Each case fires eight concurrent writes of one value for five rounds, and asserts exactly one success and a 409 with the resource's duplicate code for every other write. Resources whose names are unique regardless of case lower-case the name for half of the writers, which exercises the case-insensitive index. When the mapping was added (anchor#237), every case except workspace create failed with a 500 against the previous code; workspace create already locks its organization row. Run it with `go test ./cmd/it/ct/... -run TestConcurrentSameNameWritesReturnConflict -count=1` from `apps/anchor`.

Product permissions have no public create, and registration serializes on its invitation row unless several tenants invite one email. `TestConcurrentUniqueInsertsReturnSentinel` (`apps/anchor/cmd/it/service/name_unique_race_test.go`) races their repository inserts once, eight writers at a time, and asserts one success and the repository sentinel for every other insert. Run it with `go test ./cmd/it/service/... -run TestConcurrentUniqueInsertsReturnSentinel -count=1` from `apps/anchor`.

Add a case to one of the two tests for every new uniqueness pre-check.

## API-key pagination boundaries

The product API-key and organization API-key repository tests list their five page boundaries as named local cases: full, first, second, partial and beyond-end without sort. Every case checks total matches separately from the returned count, length and ordered IDs. Preserve the two repository scopes and the existing sequential fixture execution when extending these tables. Those tables page by offset, which a search still accepts.
