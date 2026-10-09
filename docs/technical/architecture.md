# Anchor architecture

Anchor owns the Product → Organization → Workspace hierarchy, identity directory, RBAC, organization credentials and licensing vocabulary defined in [CONTEXT.md](../../CONTEXT.md). Consumer products authenticate their own users and gate their own licensed actions; Anchor validates and records their state.

## Components and data flow

```text
Platform administrator / Product backend / Anchor UI
    -> OpenAPI HTTP handlers + auth/tenant middleware
    -> domain validation and services, wired by Uber FX
    -> transactor + go-jet repositories -> TimescaleDB/PostgreSQL
    -> durable product-event queue -> signed Product endpoint delivery
```

- [apps/anchor/cmd/app/app.go](../../apps/anchor/cmd/app/app.go) composes the service. [OpenAPI](../../apps/anchor/cmd/http/openapi.yaml) generates the strict HTTP interface; [internal/api](../../apps/anchor/internal/api/) implements it.
- Existing hierarchy services/repositories live in `apps/anchor/internal/service` and `internal/repository`. New subsystems own FX package trees such as [license](../../apps/anchor/internal/license/), [events](../../apps/anchor/internal/events/), email and integrations; [ADR-0007](../adr/0007-first-feature-slice-in-anchor.md) records that boundary.
- [Migrations](../../apps/anchor/migrations/) own schema changes. Generated go-jet models, transaction boundaries and service validation follow [engineering practices](../engineering-best-practices.md). TimescaleDB aggregates usage history under [ADR-0005](../adr/0005-timescaledb-for-usage-history.md).
- Redis backs authentication/cache behavior. Product API key scope changes evict caches through supported management endpoints; see [permission cache](../api-key-permission-cache.md).
- Product writes and their event enqueue share a transaction. [Event worker](../../apps/anchor/internal/events/worker.go) makes signed Standard Webhooks deliveries from durable queue state. Inbound identity-provider callbacks remain integration webhooks; [ADR-0017](../adr/0017-product-events-use-standard-webhooks.md) records the protocol. A self-rescheduling queue job also reconciles every Clerk instance on an interval; see [Clerk reconcile scheduler](clerk-reconcile-scheduler.md).
- [anchor-ui](../../anchor-ui/) is the light-only React administration app. [clients/go](../../clients/go/) is a separate Go module generated from the same contract and replaced locally by the service/test module.

[Dependencies and source ownership](dependencies.md) describes shared libraries and external CI/deployment. The [documentation index](../README.md) links current capability guides and preserved specs; a proposal in a spec is not proof that it shipped.

## Integration ingestion decision

An integration instance derives `CanIngest` and its diagnostic `IngestionBlockReason` from one private decision. Disabled state takes precedence over missing or blank webhook secrets, followed by lifecycle status. Active is the only allowed state; unknown statuses remain blocked. Error diagnostics are arbitrary stored text, so even the text `active` cannot authorize ingestion. Exported methods and reason strings stay stable.

## Licensing row locks

Licensing uses the existing framework transactor context for locked repository reads: call `FindByOrganization(transactor.ForUpdate(txCtx), ...)` inside the existing transaction. The retained repository lookup keeps its tenant/product/organization predicate and one-row limit. Keep the decorator inline on the read; subsequent writes use the undecorated transaction context.

Template synchronization and adjustment retain their product advisory lock before the row read. Adjustment backfill uses its existing transaction without that advisory lock; migration inherits its outer session lock. These arrangements are distinct and must not be combined during a readability refactor.

## Service operation phases

`CreateWithMember` resolves its existing organization/membership result through a private idempotency helper before acquiring the existing creation lock. Metadata validation still precedes that lookup; absent entities after a membership lookup remain server-invariant errors.

Product API-key updates prepare a requested permission replacement separately from the transaction/refetch and post-commit cache eviction. Omitted permissions skip preparation; an explicit empty replacement still checks mutability and clears permissions.

Email sends check rate limits through a private helper after rendering/variable encoding and before selecting or creating the durable send row. Dedupe, failed-row reuse, provider dispatch and the bounded detached terminal-status write retain their ordering and contexts.
