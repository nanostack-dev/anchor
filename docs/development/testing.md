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

Status/error contract changes also update Echopoint flow assertions in both organizations. Follow the [flow-suite procedure](flow-suite-testing.md) and run `echopoint --profile <prod|dev> flows run --tag anchor --environment dev` for each profile; one profile's pass does not cover the other.

[Go Build and Test](../../.github/workflows/go.yml) selects backend/frontend verification through reusable ci-workflows and reports `ci-ok`. [Isolated browser CI](../../.github/workflows/e2e-app.yml) runs the managed app gate and cleanup for its selected paths; deployed smoke and infra post-deploy suites are separate boundaries. Preserve the PR preview marker and distinguish executed, skipped, blocked and unknown checks. Documentation-only changes need link/command provenance checks, with CI selection reported accurately.
