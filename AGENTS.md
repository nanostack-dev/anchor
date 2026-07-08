# Anchor Agent Guide

Scope: Go OaaS core for hierarchy, identity, RBAC, and tenancy. Read this file once, then search only the feature files you need.

## Start Here

- OpenAPI/API work: find `openapi.yaml` first, update contract before generated code.
- DB work: use go-jet generated models; avoid raw SQL unless the repo already requires it nearby.
- App wiring: follow existing Uber FX modules and providers.

## Invariants

- Shared cross-repo rules: `docs/engineering-best-practices.md` (source of truth, kept identical with echopoint).
- Every exported service method validates its input via the shared validator (`nanostack-framework/pkg/validate`) before any repository/transaction call.
- Business rules live in the service layer, never in SQL: new migrations must not add `CHECK` constraints or business triggers (DB keeps PK/FK/UNIQUE/NOT NULL/defaults and the `updated_at` trigger only).
- Tenant-facing paths must stay tenant-scoped at repository/service boundaries.
- Repository methods that bypass tenant scope must be named `*Internal`, documented, and never called from tenant-facing handlers.
- Public IDs use KSUIDs.
- `Create`/`Update` repository methods return domain values, not pointers; re-query after update.
- OpenAPI enums are shared component schemas, referenced by `$ref`, with `x-go-type`/`x-go-type-import` when mapped to domain types.
- Product API keys are Anchor management credentials for a product and must keep the fixed `anchor_prd_apikey_` prefix. Product-level prefix configuration applies only to organization API keys (`*_org_apikey_`) issued for organizations under that product.

## Verification

- Run the narrowest relevant Go tests.
- If generated code changes, regenerate through the repo command; never hand-edit generated files.

## Git

- Conventional Commits.
- Branches: tracked `<type>/<TICKET-ID>-<description>`, untracked `<type>/<description>`.
