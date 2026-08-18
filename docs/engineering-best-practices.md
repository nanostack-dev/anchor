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

## API conventions

### The `?include=` query parameter

A read operation can let the caller ask for a related resource in the same call. The organization endpoints in anchor are the reference implementation — `OrganizationIncludeParameter` in `apps/anchor/cmd/http/openapi.yaml`. Obey the rules below in both repos.

**Shape.** Declare one reusable parameter component per aggregate, and one enum schema for its values. Use `in: query` with `style: form` and `explode: false`. The caller then sends one key with comma-separated values — `?include=license`. Do not accept a repeated key. One key per request gives one canonical value in proxy caches, access logs, and test assertions. It also keeps the parameter one `$ref` that every read operation of the aggregate shares.

Give each aggregate its own enum. A value that is valid for one aggregate is not valid for another. A global include enum lets a caller ask a product endpoint for `license`.

**An enum, never a free string.** Map the enum schema to the domain type with `x-go-type` and `x-go-type-import`. The generated binding then refuses an unknown value with a 400, before the request gets to the service. A free string moves that work into the service. The service must then invent an error, or discard the value with no message to the caller.

**Absent is not empty.** Carry the related resource as an optional pointer on the domain object, and as an optional property in the response. Nil means "the caller did not ask for it". Nil never means "the parent does not have it". Write that sentence on the domain field and in the OpenAPI description of the property. A consumer that reads absence as "none" gets a wrong answer from every call that omits the include.

**One statement per included resource.** Read the page first. Then fill in each named resource for the whole page in one statement. Use a batch repository method that takes the parent IDs and returns a map keyed by parent ID — see `FindByOrganizations` in anchor. Never call the single-parent read in a loop. A loop turns a page of 100 rows into 101 statements. With the batch method, an include costs one more statement for the whole page, at any page size. Put that cost in the parameter description, so the caller knows the price.

**The owner of the resource owns the batch read.** Declare the batch method on the subsystem that owns the resource, and reach it from the parent through that subsystem's service. In anchor, `attachIncludes` in `internal/service/organization_service.go` calls `ListByOrganizations` on the license service, which calls `FindByOrganizations` in `internal/license/repository/`. The parent repository never selects from another subsystem's tables. A join there copies the owner's tenant scope, cache rules, and column mapping into a second place, where the owner cannot see them.

**Tenant scope.** A related resource can be tenant-scoped when its parent is not. Put `TenantID` on the read input with `validate:"required_with=Include"`. Read the tenant in the handler only when the caller named an include. A read without an include then needs no tenant.

**An include never carries derived data.** An include returns the stored record only. Data that a dedicated endpoint computes stays on that endpoint. In anchor the license include returns `license.OrganizationLicense`. The license route returns `license.OrganizationLicenseRead`, which adds usage and a derived status. Two types put the rule under the compiler. There are two reasons for the rule. Derived data has its own cost, cache, and freshness rules, and a page-wide include multiplies that cost where no caller can see it. Two sources for one computed value also drift apart.

**Add the parameter without a break.** Keep the parameter `required: false`. In the Go SDK, add it as a variadic argument on a single-resource read — `Get(ctx, id, include ...Include)` — and as an `Include(...)` builder method on a search. Existing callers then compile with no change. Never add the include as a new positional argument. Never change a response type to carry it. A new enum value is an additive change. Removal of an enum value is a breaking change.

## Testing

### HTTP plumbing belongs in the test SDK

Each repo keeps a test SDK for its component tests — `apps/anchor/cmd/it/shared/dsl` in anchor, `cmd/it/ct/sdk` in echopoint. A test file holds what is about the behavior under test. The credential, the request, and the status assertions belong in the SDK.

**The SDK owns the credential.** The SDK mints the key and holds it on the handle it returns — `ProductContext.Organizations(scopes...)` in anchor. Name no scope, and the handle gets a key with every scope. Name scopes only when the authorization is the subject of the test. A test about a read must not list the scopes a read needs. That list is noise, and it goes stale on the next permission change.

**The SDK owns the request and the status.** One act builds the body, calls the generated client, and carries the `require.NoError` plus status plus `require.NotNil` triplet once. The act returns the decoded body, not the response. Call `t.Helper()` in every act, so a failure points at the line in the test file, not at the SDK.

**The SDK also owns the untouched response.** A test whose subject is a refusal needs the response with no assertion on it. Anchor gives each act a `*Raw` twin for that — `GetRaw` next to `Get`. Echopoint reaches the same escape hatch through `Client()` on the resource handle. Without one of the two, a refusal test builds the whole call again by hand.

**The test file keeps two things.** It keeps the request body that is specific to the feature under test. It also keeps a deliberate scope choice, when that choice is itself the assertion.

**Move the plumbing on the second test.** A test that builds a request, checks the error, and checks the status starts late on its own subject. Those same lines repeat in every other test of the route. Put them in the SDK as soon as a second test needs them.

## Verification

- After Go edits: `golangci-lint run --path-mode=abs --timeout 5m --fix`. No `//nolint` or other suppressions without prior approval.
- Run the narrowest relevant tests per the repo's test guide.

## Git

- Conventional Commits.
- Branches: tracked `<type>/<TICKET-ID>-<description>`, untracked `<type>/<description>`.
