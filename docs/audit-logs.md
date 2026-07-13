# Audit Logs

Product-facing audit trail for Anchor. Records who did what, to which resource, when — across the Product → Organization → Workspace hierarchy. Append-only, tenant-scoped, queryable via API and the Anchor UI.

## Why

Anchor is the identity/RBAC core. It is the only service that sees permission grants, membership changes, and API key lifecycle events. "Who granted admin?" is a table-stakes enterprise/compliance question (SOC 2 access-grant evidence), and every product built on Anchor inherits the gap if Anchor doesn't record it. The integration subsystem already has its own audit log (`integration_audit_logs`); this feature generalizes that proven pattern to the whole platform.

## Research summary

Design synthesized from WorkOS Audit Logs, Auth0 Log Events, Stripe Events, GitHub Audit Log API, Retraced, Zitadel's eventstore, and Postgres audit-table practice. Key take-aways adopted:

| Decision | Source / rationale |
| --- | --- |
| Action names: dotted `resource.verb_past_tense` (`organization.created`, `member.role_updated`) | Unanimous across WorkOS / Stripe / GitHub / Retraced. Prefix filtering works (`organization.*`). Never encode success/failure in the name (Auth0's `s`/`f` codes are the anti-pattern). |
| Typed columns for every filterable attribute, JSONB only for the variable tail | Zitadel `events2`: promote org/action/actor/target/time to columns; `payload jsonb` for the rest. Never filter core UX inside JSONB. |
| Actor as `{type, id, name-snapshot}`; type ∈ user / api_key / system | WorkOS + Retraced model machine actors explicitly. Names are denormalized snapshots so the row stays readable after the actor is deleted. |
| Target as `{type, id, name-snapshot}` (single, not array) | Retraced uses single target; WorkOS uses an array. Anchor's mutations are single-target; a `metadata` blob covers secondary resources without the cost of a join table. |
| Outcome as a field (`SUCCESS`/`FAILURE`), not encoded in action | Retraced `is_failure`. |
| Immutable append-only table: no `updated_at`, insert-only repository, no update/delete code paths | audit-trigger-91plus pattern; matches existing `integration_audit_logs` ("No updated_at: audit logs are immutable"). |
| Write in the same request flow, never fail the caller | letsbuildsolutions.com: audit insert alongside the mutation. Anchor keeps the existing `writeAuditLog` semantics: synchronous insert, error logged, mutation never rolled back for an audit failure. |
| Offset pagination via existing `SearchRequest` convention | Anchor's established `POST …/search` + `PaginationRequest` (limit ≤ 100) pattern. Cursor pagination (Auth0 checkpoint style) can be added later for export/tailing. |
| Tenant-first composite indexes `(product, created_at DESC)` etc. | letsbuildsolutions.com / GitHub flat denormalized rows for SIEM-style querying. |
| Partition-ready, not partitioned | Crunchy/pg_partman guidance: don't partition below tens of millions of rows. Anchor audit volume is admin-action-scale. KSUID ids are k-sortable, and rows are pruneable by `created_at` if retention is added later. |
| Retention: none in v1 | WorkOS ships 30d default → 365d paid; Anchor is single-tenant-mode today. Add a retention job (partition drop or batched delete) when volume demands. |

Not adopted (deliberately): trigger-based row auditing (supa_audit — no actor/context, wrong semantics), pgaudit (server-log DBA tool), log-stream-only audit (Ory — unqueryable per tenant), hash chains / tamper-evidence receipts (Retraced) — revisit if a compliance customer asks.

## Data model

Migration `000023_audit_logs`:

```sql
CREATE TABLE audit_logs (
    id                 VARCHAR(255) PRIMARY KEY, -- KSUID prefix: alog_
    platform_tenant_id VARCHAR(255) NOT NULL,
    product_id         VARCHAR(255) NOT NULL,    -- no FK: rows survive product deletion
    organization_id    VARCHAR(255),             -- NULL for product-level events
    action             VARCHAR(100) NOT NULL,    -- dotted: organization.created
    outcome            VARCHAR(20)  NOT NULL DEFAULT 'SUCCESS',
    actor_type         VARCHAR(30)  NOT NULL,    -- PLATFORM_USER | PRODUCT_API_KEY | ORGANIZATION_API_KEY | SYSTEM
    actor_id           VARCHAR(255),
    actor_name         VARCHAR(255),             -- denormalized snapshot
    target_type        VARCHAR(50)  NOT NULL,    -- organization | workspace | membership | api_key | role | ...
    target_id          VARCHAR(255),
    target_name        VARCHAR(255),             -- denormalized snapshot
    request_id         VARCHAR(255),             -- correlate with request logs
    metadata_json      JSONB,                    -- variable detail (changed fields, secondary ids)
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- No updated_at: audit logs are immutable.
);
```

Scoping columns are denormalized (`platform_tenant_id`, `product_id`, `organization_id`) so tenant-scoped reads need no JOIN — unlike `integration_audit_logs`, which joins through `integration_instances`. Audit rows carry no foreign keys at all: they must survive deletion of everything they reference, including the product itself (a `product.deleted` entry is written after the product row is gone, and a cascade would erase the product's entire trail).

Indexes (product-first, newest-first):

```sql
CREATE INDEX idx_audit_logs_product_created  ON audit_logs (product_id, created_at DESC);
CREATE INDEX idx_audit_logs_org_created      ON audit_logs (organization_id, created_at DESC) WHERE organization_id IS NOT NULL;
CREATE INDEX idx_audit_logs_product_action   ON audit_logs (product_id, action, created_at DESC);
CREATE INDEX idx_audit_logs_product_actor    ON audit_logs (product_id, actor_id, created_at DESC);
CREATE INDEX idx_audit_logs_product_target   ON audit_logs (product_id, target_type, target_id, created_at DESC);
```

No GIN index on `metadata_json` — nothing filters inside it.

## Action taxonomy

`<resource>.<verb_past_tense>`, resources matching Anchor's domain nouns:

- `product.created` / `product.updated` / `product.deleted`
- `organization.created` / `organization.updated` / `organization.deleted`
- `organization.member_added` / `organization.member_removed` / `organization.member_role_updated`
- `workspace.created` / `workspace.updated` / `workspace.deleted`
- `organization_api_key.created` / `organization_api_key.updated` / `organization_api_key.deleted`
- `product_api_key.created` / `product_api_key.updated` / `product_api_key.deleted`
- `role.created` / `role.updated` / `role.deleted` / `role.permission_assigned` / `role.permission_unassigned`
- `permission.created` / `permission.updated` / `permission.deleted`
- `resource_permission.created` / `resource_permission.updated` / `resource_permission.deleted`
- `product_user.created` / `product_user.deleted`

Constants live in `internal/domain/audit/audit_log.go`. New actions are added there — never free-strings at call sites.

v1 covers product-plane events only. Platform-plane events (platform invitations, platform users) have no owning product and are future work — they'd need a nullable `product_id` or a platform-scoped read path.

## Write path

`AuditLogService.Record(ctx, audit.Entry)`:

1. Fills `ID` (KSUID `alog_`), `CreatedAt`, defaults `Outcome` to `SUCCESS`.
2. Resolves actor from request context (`security.GetCurrentUserID` → `PLATFORM_USER`; product-scope-without-user → API key actor; neither → `SYSTEM`).
3. Inserts synchronously via the repository. On error: logs, never returns the error — a failed audit write must not fail the mutation (same semantics as the existing integration `writeAuditLog`).

Call sites are the mutating service methods (organization, workspace, membership, API key, role, invitation services), placed after the mutation succeeds. `ctx` may be a transactor tx context, in which case the audit row commits atomically with the mutation.

The repository is insert-only: `Create` and scoped list methods exist; no update or delete methods are defined, and none may be added.

## API

Contract-first in `cmd/http/openapi.yaml` (before regen), following the existing search convention:

```
POST /v1/products/{product_id}/audit-logs/search
security: platformBearerAuth
```

v1 is bearer-only (the Anchor UI use case). Product API key access (`audit_log:read` scope) requires adding the permission to the product permission catalog — future work.

Request (`AuditLogSearchRequest` = `SearchRequest` + filter):

```yaml
pagination: { limit: 1-100 (default 20), offset }
sort_direction: ASC | DESC        # over created_at; default DESC
full_text_search: string          # ILIKE over action, actor_name, target_name
filter:
  organization_id: string
  actions: [string]
  actor_types: [AuditLogActorType]
  actor_id: string
  target_type: string
  target_id: string
  outcome: AuditLogOutcome
  created_after: date-time
  created_before: date-time
```

Response (`AuditLogListResponse` = `PagedListResponse` + `items: [AuditLogResponse]`), where `AuditLogResponse` mirrors the table columns with `metadata` parsed as an object. Enums (`AuditLogActorType`, `AuditLogOutcome`) are shared `$ref` component schemas per repo invariant.

The endpoint is tenant-scoped: handler pulls `tenantID` from `security.GetTenantID(ctx)`, service filters `platform_tenant_id` + `product_id` in the repository. No `*Internal` read is exposed over HTTP.

After a product is deleted its audit rows are retained (no FK) but become unreachable through this product-scoped endpoint — the auth middleware 404s on the deleted product. A platform-scoped read for retained trails is future work.

`POST …/search` (not GET) so the hey-api generator emits a TanStack Query hook with a rich filter body, matching every other datatable in anchor-ui.

## Frontend (anchor-ui)

Product-scoped page mirroring the API Keys page:

- `src/routes/products/audit-logs.tsx` + `ROUTE_PATHS.PRODUCT_AUDIT_LOGS` + sidebar entry (product group, `ScrollText` icon).
- `src/pages/product-audit-logs.tsx` — `useProduct()` + `<Page>` shell.
- `src/components/product/audit/AuditLogDatatable.tsx` — `AnchorDataTable` in server-side mode over `searchAuditLogsOptions`, columns: time, action (badge), actor, target, organization, outcome (`StatusBadge`). Filters: action (faceted), actor type (faceted), outcome (faceted), date range (`DateRangeFilter` — its first consumer), plus full-text search.
- `src/components/product/audit/AuditLogDetailSheet.tsx` — right-side `Sheet` showing all fields + pretty-printed `metadata` JSON. Row click opens it; no extra endpoint needed.

Light-only, semantic tokens, Lucide icons, no Storybook (not used in anchor-ui).

## Out of scope (v1) — future work

- Retention policies / partitioning (`pg_partman`), per-product retention tiers.
- CSV export job (WorkOS-style create → poll → signed URL).
- SIEM / webhook streaming per organization.
- Organization-scoped viewer endpoint for end-customer admin portals (Retraced viewer-token pattern).
- Tamper-evidence hashes.
- Auth events (`user.signed_in` etc.) — v1 covers management-plane mutations only.
