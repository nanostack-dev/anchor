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
- Product API keys are Anchor *management* credentials and keep the fixed `anchor_prd_apikey_` prefix. Configurable product-level prefixes apply only to organization API keys (`*_org_apikey_`).
- Contract first: update `openapi.yaml`, then regenerate through the repo command. Generated files are never hand-edited.

## Agent skills

### Issue tracker

GitHub Issues on `nanostack-dev/anchor`, driven through the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context — one root `CONTEXT.md` plus `docs/adr/`. See `docs/agents/domain.md`.
