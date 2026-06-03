# Anchor Agent Guide

Scope: Go OaaS core for hierarchy, identity, RBAC, and tenancy. Read this file once, then search only the feature files you need.

## Start Here

- OpenAPI/API work: find `openapi.yaml` first, update contract before generated code.
- DB work: use go-jet generated models; avoid raw SQL unless the repo already requires it nearby.
- App wiring: follow existing Uber FX modules and providers.

## Invariants

- Tenant-facing paths must stay tenant-scoped at repository/service boundaries.
- Repository methods that bypass tenant scope must be named `*Internal`, documented, and never called from tenant-facing handlers.
- Public IDs use KSUIDs.
- `Create`/`Update` repository methods return domain values, not pointers; re-query after update.
- OpenAPI enums are shared component schemas, referenced by `$ref`, with `x-go-type`/`x-go-type-import` when mapped to domain types.

## Verification

- Run the narrowest relevant Go tests.
- If generated code changes, regenerate through the repo command; never hand-edit generated files.

## Git

- Conventional Commits.
- Branches: tracked `<type>/<TICKET-ID>-<description>`, untracked `<type>/<description>`.
