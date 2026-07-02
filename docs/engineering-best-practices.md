# Engineering Best Practices — Anchor & Echopoint

Shared engineering rules for `anchor` and `echopoint`. Repo-specific guidance lives in each repo's `AGENTS.md`; for the topics below, this document is the source of truth in both repos. Keep the two copies (`anchor/docs/engineering-best-practices.md`, `echopoint/docs/engineering-best-practices.md`) identical — update both in the same work session.

## The service layer owns all business rules

The database enforces structural integrity only. Every business rule — input shape, enum membership, value ranges, cross-entity consistency — lives in Go, in the domain/service layer.

### 1. Every service input goes through the validator

- Define a dedicated `*Input` struct per operation in the domain layer (`internal/domain/<aggregate>/`), with `validate:"..."` struct tags. Inputs, DB structs, and domain objects are distinct types.
- Validation uses `go-playground/validator/v10` through the shared wrapper `github.com/nanostack-dev/nanostack-framework/pkg/validate` (`ValidateStruct`). Custom tags available: `notblank`, `strongpassword`, `permission_name`, `request_url`.
- **Every exported service method validates its input as its first action** — before any repository call, transaction, or external call. Two sanctioned styles:
  - Inline: `if err := validateStruct(input); err != nil { return ..., err }` at the top of the method.
  - `Validate()` method on the input type when semantic rules exceed what tags express (e.g. `CreateScheduleInput.Validate(minRunInterval)`): run `ValidateStruct` first, then the semantic checks.
- Cross-entity rules that need lookups (name uniqueness per tenant, "tags exist", "recipient is a member") run in the service after the structural pass, returning a 400-mapped domain error (`fault.NewWithStatus` / feature `InvalidInputError`).
- A service method that accepts more than trivial identifiers and performs no validator call is a defect, even if the handler or DB would catch bad input.

### 2. No business rules in SQL — no CHECK constraints, no business triggers

- New migrations MUST NOT add `CHECK` constraints. This includes enum whitelists (`status IN (...)`), non-empty guards (`length(name) > 0`), and range guards (`timeout_seconds > 0`). All of these are validator tags (`oneof`, `notblank`, `min`) or service logic.
- New migrations MUST NOT add triggers, functions, or views that encode business behavior. The only sanctioned trigger is the shared `update_updated_at_column()` `updated_at` touch.
- The DB layer keeps: primary keys, foreign keys (with explicit `ON DELETE` policy), `UNIQUE` constraints/indexes, `NOT NULL`, column defaults, and plain performance indexes.
- Enum values are defined once, in Go domain code — never duplicated between a Go type and a SQL `CHECK`.
- **Legacy exceptions:** CHECK constraints predating this rule exist (anchor migrations `000001`–`000022`, e.g. `chk_integration_instances_status`, email subsystem checks; echopoint migrations `000001`, `000006`, `000023`, e.g. method/status/runner-type checks). Do not extend them. When a change touches a column governed by a legacy CHECK (e.g. adding an enum value), move the rule to domain validation and drop the CHECK in the same migration.

## Service layer conventions

- Method shape: validate input → business/cross-entity rules → `transactor.InTx` / repository calls. Never call the repository before validation passes.
- `Create`/`Update` repository methods return domain values, not pointers; re-query after update.
- Tenant-scoped by default at repository/service boundaries. Methods that bypass tenant scope are suffixed `*Internal`, documented, and never called from tenant-facing handlers.
- Wiring via Uber FX modules; DB access via go-jet generated models; schema changes via migrations only, never hot DB edits.
- Spec-first API work: update `openapi.yaml`, regenerate through the repo command, never hand-edit generated files. Enums are shared component schemas referenced by `$ref`, with `x-go-type`/`x-go-type-import` when mapped to domain types.

## Verification

- After Go edits: `golangci-lint run --path-mode=abs --timeout 5m --fix`. No `//nolint` or other suppressions without prior approval.
- Run the narrowest relevant tests per the repo's test guide.

## Git

- Conventional Commits.
- Branches: tracked `<type>/<TICKET-ID>-<description>`, untracked `<type>/<description>`.
