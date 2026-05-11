# Agent Guide: Anchor Core

Organization-as-a-Service (OaaS) platform for multi-tenancy management.

## Key Responsibilities
- Hierarchy management (Product/Org/Workspace).
- RBAC and identity management.

## Tech Stack
- **Language**: Go
- **Frameworks**: Uber FX, go-jet
- **Database**: PostgreSQL

## Best Practices
- Strictly enforce tenancy isolation at the database level.
- Use KSUIDs for all public identifiers.
- Keep API contracts updated in the OpenAPI specification.
- **Internal repository methods**: Repository methods that bypass tenant scoping
  (e.g. `FindByIDInternal`) are reserved for trusted system-internal paths such
  as async queue workers and webhook ingress, where no authenticated tenant
  context exists. They must be suffixed with `Internal` and carry a doc-comment
  explaining when their use is permitted. They must **never** be called from
  tenant-facing API handlers.
- OpenAPI enum/type reuse rules:
  - Define each unique enum as its own schema under `components/schemas`.
  - Reference shared enums via `$ref` instead of re-declaring inline enum values in request/response fields.
  - For server codegen typing, always add `x-go-type` and `x-go-type-import` on enum schemas when they map to domain types.
  - Keep request and response fields aligned to the same shared schema when they represent the same semantic value.
- Repository return convention:
  - `Create` and `Update` repository methods must return domain objects as values, not pointers.
  - After update operations, always re-query and return the updated domain value.

## Git Conventions
- **Commit Messages**: Follow [Conventional Commits](https://www.conventionalcommits.org/):
  - `feat: implement /me endpoint with JWT auth`
  - `fix: resolve organization isolation in queries`
  - `refactor: extract API key validation logic`
- **Branch Naming**: When working on tracked tasks, include ticket number:
  - Format: `<type>/<TICKET-ID>-<description>`
  - Examples: `feat/NAN-42-me-endpoint-jwt`, `fix/NAN-30-integration-base`
  - For untracked work: `<type>/<description>` (e.g., `docs/update-readme`)
