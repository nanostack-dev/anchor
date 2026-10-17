# Go conventions

Use these rules with the local implementation rules and engineering practices. No workspace guide is required.

- Reuse the pinned framework and pgkit APIs before adding an app utility; read their source/versioned documentation. Wire application dependencies with Uber FX, keep transport in handlers, business validation in domain/service code, persistence in repositories and conversion in mappers.
- Pass `context.Context` first and honor cancellation. Handle every error and wrap with `%w` where context is added. Keep multi-statement writes inside the existing transactor.
- Public resource identifiers are KSUIDs. Tenant-facing paths remain tenant-scoped. Bypasses are named `*Internal`, documented and unreachable from tenant handlers.
- Collection transformations use `nanostack-framework/pkg/functional`: `Slice(...).Map/Filter/UniqueBy/FoldLeft/ToMap`. Use `Option` for optional values instead of a pointer or `(T, bool)`, `Result` for composed result values instead of an ad hoc `(T, error)` transformation, and `Tuple2` for pairs. Preserve the repository's exported boundary signatures and documented `Find…` return shape. A plain loop is appropriate for side effects, early exit or a functional expression that reads worse; state the reason in review.
- go-jet scan destinations use generated `model` types. qrm matches columns by table prefix: a local row struct can silently scan zero values. When unavoidable, alias every selected column to the local struct's name; integration suites enable `qrm.GlobalConfig.StrictScan`.
- Durable background work uses pgkit's persisted queue or explicit database-backed state. A detached `go func()` cannot own work that must survive process restart.
- Generated server, client and database models are outputs of the contract/schema and project generators; never edit them by hand. Regenerate in the owning Go module and verify generated drift, lint, narrow relevant tests and build after Go/schema edits.
- Use pure package tests for pure logic and the local integration suites/fixtures for DB and external-service behavior. Preserve shuffle/race checks and the documented parallelism exceptions. Obtain prior approval before adding lint suppressions.

Commands and product-specific exceptions are in [testing](testing.md), [implementation rules](agent-rules.md) and [engineering practices](../engineering-best-practices.md).
