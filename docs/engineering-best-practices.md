# Engineering Best Practices — Anchor & Echopoint

Shared engineering rules for `anchor` and `echopoint`. Repo-specific guidance lives in each repo's `AGENTS.md`; for the topics below, this document is the source of truth in both repos. Keep the two copies (`anchor/docs/engineering-best-practices.md`, `echopoint/docs/engineering-best-practices.md`) identical — update both in the same work session.

## The service layer owns all business rules

The database enforces structural integrity only. Every business rule — input shape, enum membership, value ranges, cross-entity consistency — lives in Go, in the domain/service layer.

### 1. Every service input goes through the validator

- Define a dedicated `*Input` struct per operation in the domain layer (`internal/domain/<aggregate>/`), with `validate:"..."` struct tags. Inputs, DB structs, and domain objects are distinct types. Exception: if the arguments are only identifiers (ids, plus an optional credential such as a token), pass them as parameters. Do not wrap them in an `*Input` struct. Check emptiness inline. Use `*Input` when the operation carries a body: several fields, optionals, or semantic rules.
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

## Database migrations and rollout

A rolling deploy runs the old and the new application against the new schema at the same time, and a production table is never empty. Each rule below prevents a change that passes on a fresh database and fails on a loaded one or mid-rollout. The `migration-lint` job of `go.yml` enforces the index and backfill rules on every migration a pull request adds ([how it works](https://github.com/nanostack-dev/ci-workflows/blob/main/docs/technical/migration-lint.md)); `-- lint: allow-backfill <reason>`, `-- lint: allow-hypertable-index <reason>` and `-- squawk-ignore <rule>` record a reviewed exception in the diff.

**Migrations are append-only once on `main`.** Dev and prod may already have run them, so a change is always a new migration. Both repos follow this; Echopoint's `AGENTS.md` links its `migrations-verify` CI job and the postmortem of the deleted migration.

**Build an index on an existing table with `CREATE INDEX CONCURRENTLY IF NOT EXISTS`, alone in its migration file.** A plain `CREATE INDEX` blocks writes for the whole build. golang-migrate sends a single-statement file without a transaction. A second statement turns the file into an implicit transaction, and PostgreSQL refuses `CONCURRENTLY` inside one. The down file is `DROP INDEX CONCURRENTLY IF EXISTS`.

- A TimescaleDB hypertable cannot build concurrently. Use `CREATE INDEX IF NOT EXISTS ... WITH (timescaledb.transaction_per_chunk)` and a plain `DROP INDEX IF EXISTS` down file, as Echopoint migration `000066` does.
- A failed concurrent build leaves an INVALID index and a dirty migration. Drop the index by name, force the migration version back, then rerun. `IF NOT EXISTS` would skip the invalid index by name and record the migration as applied with an index no query can use.

**Never backfill a large table with one `UPDATE` in a migration.** The row locks last until commit and block every writer on the table (echopoint#644 review). Prefer a derived value as a PostgreSQL 18 `GENERATED ALWAYS AS (...) VIRTUAL` column with a concurrent expression index: adding it is metadata-only and it is always consistent, as in Echopoint migration `000084`. When a stored value is unavoidable, backfill in batches from an application job.

**Expand, then contract.** Every migration must work with the previous application version still running.

- Add nullable or defaulted columns.
- Never rename or drop a column the running version uses in the same release. Do it in a later release, once no running version reads it.
- Never require application code to maintain a derived column. Old pods do not know about it and keep writing stale values. Compute it in the database (the `VIRTUAL` column above).

**A new config field is optional with a default.** The framework loader treats an untagged field as required, so tag it `optional:"true"` and apply the default in code. A deployment whose config predates the field must still start. echopoint#645 crashed at startup in e2e because of a required `anchor.page_size`. Test the service with the deployed config shape, not only with the file in the repo.

## API conventions

### The `?include=` query parameter

A read operation can let the caller ask for a related resource in the same call. The organization endpoints in anchor are the reference implementation — `OrganizationIncludeParameter` in `apps/anchor/cmd/http/openapi.yaml`. Obey the rules below in both repos.

**Shape.** Declare one reusable parameter component per aggregate, and one enum schema for its values. Use `in: query` with `style: form` and `explode: false`. The caller then sends one key with comma-separated values — `?include=license`. Do not accept a repeated key. One key per request gives one canonical value in proxy caches, access logs, and test assertions. It also keeps the parameter one `$ref` that every read operation of the aggregate shares.

Give each aggregate its own enum. A value that is valid for one aggregate is not valid for another. A global include enum lets a caller ask a product endpoint for `license`.

**An enum, never a free string.** Map the enum schema to the domain type with `x-go-type` and `x-go-type-import`. The generated binding then refuses an unknown value with a 400, before the request gets to the service. A free string moves that work into the service. The service must then invent an error, or discard the value with no message to the caller.

**Absent is not empty.** Carry a single related resource as an optional pointer on the domain object. For collections, use a plain slice in the domain and pass the include choice explicitly to the API mapper, as role permissions do. Keep the response property optional: omit an unrequested collection, and return `[]` for a requested collection with no records. A pointer to the response slice preserves this distinction with JSON `omitempty`; it does not belong on the domain collection. An absent response property means "the caller did not ask for it", never "the parent does not have it". Document that distinction in the OpenAPI property description. A consumer that reads absence as "none" gets a wrong answer from every call that omits the include.

**One statement per included resource.** Read the page first. Then fill in each named resource for the whole page in one statement. Use a batch repository method that takes the parent IDs and returns a map keyed by parent ID — see `FindByOrganizations` in anchor. Never call the single-parent read in a loop. A loop turns a page of 100 rows into 101 statements. With the batch method, an include costs one more statement for the whole page, at any page size. Put that cost in the parameter description, so the caller knows the price.

**The owner of the resource owns the batch read.** Declare the batch method on the subsystem that owns the resource, and reach it from the parent through that subsystem's service. In anchor, `attachIncludes` in `internal/service/organization_service.go` calls `ListByOrganizations` on the license service, which calls `FindByOrganizations` in `internal/license/repository/`. The parent repository never selects from another subsystem's tables. A join there copies the owner's tenant scope, cache rules, and column mapping into a second place, where the owner cannot see them.

**Tenant scope.** A related resource can be tenant-scoped when its parent is not. Put `TenantID` on the read input with `validate:"required_with=Include"`. Read the tenant in the handler only when the caller named an include. A read without an include then needs no tenant.

**An include never carries derived data.** An include returns the stored record only. Data that a dedicated endpoint computes stays on that endpoint. In anchor the license include returns `license.OrganizationLicense`. The license route returns `license.OrganizationLicenseRead`, which adds usage and a derived status. Two types put the rule under the compiler. There are two reasons for the rule. Derived data has its own cost, cache, and freshness rules, and a page-wide include multiplies that cost where no caller can see it. Two sources for one computed value also drift apart.

**Add the parameter without a break.** Keep the parameter `required: false`. In the Go SDK, add it as a variadic argument on a single-resource read — `Get(ctx, id, include ...Include)` — and as an `Include(...)` builder method on a search. Existing callers then compile with no change. Never add the include as a new positional argument. Never change a response type to carry it. A new enum value is an additive change. Removal of an enum value is a breaking change.

### Collections are cursor-paginated searches

Every persisted resource that the API returns as a collection has a search operation, and every search pages with a cursor. The framework primitive is `transactor.StrictKeys` (which refuses a sort field the search does not map) with `PageBuilder.Keyset`; its contract is [ADR 0001 in nanostack-framework](https://github.com/nanostack-dev/nanostack-framework/blob/main/docs/adr/0001-keyset-cursor-pagination.md).

**Shape.** `POST /<collection>/search`, with `filter`, `sort`, optional full-text and `pagination` in the body. Nest the route under the parent when the parent scopes the collection (`/organizations/{organization_id}/members/search`). The response carries `items`, `total`, `count` and `next_cursor`.

**Cursor contract.**

- `pagination.cursor` is optional. Absent starts at the first row. Send back the previous response's `next_cursor` to continue, with the same filter and sort.
- `next_cursor` is absent or null on the last page. Iterate until it is absent; never infer the end from `count < limit` or from `total`.
- `total` counts every match of the filter, not the rows left after the cursor.
- A cursor is opaque. Clients never build, parse or store one beyond the walk that produced it. A cursor issued for another sort, a malformed one, or a cursor sent with a non-zero `offset` answers 400 (`INVALID_CURSOR`).
- `offset` stays for page-number navigation in a UI table. Machine iteration (syncs, exports, SDK `All` helpers, MCP tools, the CLI, Terraform data sources) always uses the cursor.

**Validate before shortcuts.** A search validates its whole input before any early return, such as an empty scope: the sort field, the filter enum values, and the cursor (`search.Pagination.Validate` and `Order.Seek`). An unknown `sort_by` or filter value answers 400, never a silent fallback to the default. Otherwise the same request answers 400 or 200 depending on how much data the caller has (echopoint#644 and #645 reviews).

**Ordering is total.** Every sortable field maps to a typed key (`StringKey`, `EnumKey`, `TimeKey`, `IntKey`, `Nullable*` for a nullable column), and the ordering ends with the primary key as the tiebreak. A search without a requested sort orders by its documented default, still ending with the id. Never order a page by a non-unique column alone.

**Indexes.** A search on a table that grows without bound has a composite index matching its default ordering behind its scope, for example `(organization_id, created_at, id)`. The seek is then one range scan, at any depth.

**Static catalogs and aggregates are not resources.** A fixed list compiled into the binary (node definitions, permissions), a live in-memory view (connected runners) and an aggregate computed per request (analytics buckets, usage series) return their array directly. Pagination there would be a fiction. State the reason in the operation description.

**GET lists.** A new collection gets a search, never a `GET` list. An existing `GET` list of a persisted resource is deprecated in the spec, with `deprecated: true` and the search named in its description, once the search exists. First-party clients move to the search in the same change.

**Replacing a list or a consumer.** Before moving an endpoint or a consumer to a search, write the parity matrix of the old behavior and pin each row with a test: the limit cap and whether it read everything, the matching semantics (case sensitivity, `LIKE` wildcards), the default order, the fields present in items, and the totals. A difference is deliberate and named in the PR. The cursor-search rollout (anchor#234, echopoint#644, echopoint#645) caught deprecated email lists clamped at 1000, a template search that lost `description` and case-insensitivity, and folder trees truncated at 100.

**Reading every row.** A caller that needs every row walks `next_cursor` to the end. A single page of N rows is a defect, unless the collection is bounded by design and the bound is stated where the call is made.

**SDK.** A search builder exposes `Cursor(string)` and the result exposes `NextCursor`. Offer an iterator (`All(ctx)` / `Pages(ctx)`) that follows `next_cursor`, so callers never write the loop themselves.

**Tests.** Every search has an integration test that walks more than one page by cursor across rows with tied sort values, and asserts that each row comes back exactly once, in order. An invalid cursor answers 400.

**Conformance.** Every search has a `searchtest` suite in the component tests: walks for its default order and every sort field, refusals in every scope including short-circuit ones, and forged cursors where the service issues them.

### HTTP error statuses

The status follows the **addressing** of the thing that is wrong: where the caller named it, not what the method does to it. A `GET` and a custom action that name the same missing resource in the path answer the same status. Pick the row from the table, then use the named constructor.

| What is missing or wrong | Where the caller named it | Status | Constructor |
| --- | --- | --- | --- |
| A resource named in the URI path | Path | 404 | `fault.NotFound` |
| An entity named in the request body or the query string | Body or query | 400 | `fault.BadRequest` |
| A field that fails a `validate:"..."` tag or a semantic rule | Body | 400 | `validate.ValidateStruct`, or `fault.BadRequest` |
| A non-credential header that is absent or malformed | Header | 400 | `fault.BadRequest` |
| A credential that is absent, malformed, expired, or does not authenticate | Header | 401 | `fault.Unauthorized` |
| A permission the authenticated principal does not hold | Nowhere | 403 | `fault.Forbidden` |
| Current state refuses the request, and a later or different request can succeed | Nowhere | 409 | `fault.Conflict` |
| A licensed limit the caller can free, by deleting something or by changing plan | Nowhere | 409 | `fault.Conflict` |
| More requests than the window allows | Nowhere | 429 | `fault.TooManyRequests` |
| A server invariant the caller could not have influenced | Nowhere | 500 | A bare wrapped error |

These nine rows are the whole set. Every status in it has a semantic constructor, so `fault.NewWithStatus` has one remaining use, and only one: **499**, for a long-poll the client abandoned before the server answered. Nothing is written to that client, and the status exists so the request boundary logs the outcome at info instead of firing the 5xx alerts. Echopoint's `errClientClosedRequest` in `internal/feature/runners/handler.go` is the only instance. Do not add a second one without the same two properties: no response reaches anyone, and a real status would misreport the service's health.

Five statuses stay out. 402 is reserved for future use by RFC 9110 §15.5.3, so a licensed limit is a 409 and never a 402. 410 is covered under 404, below. 422 is covered under validation, below. 424 is covered under 409: Anchor's `EMAIL_INTEGRATION_NOT_FOUND` is a product that configured no integration, which someone can configure, so a later request succeeds. 405 and 501 come from `chi` and from the generated server; service code never builds them.

**A path with two identifiers answers 404 on whichever one is absent.** `/organizations/{organization_id}/resource/{resource_id}` addresses one target through two segments. Either segment failing to resolve means the target does not exist, so both answer 404. Depth does not change the rule, and neither does which segment failed.

Read the parent before the child, or scope the child read by the parent. One of the two, never neither. A child read by its own identifier alone returns a row that belongs to a different parent, and answering 200 with it is a cross-tenant leak wearing a valid URL. Echopoint's `GetExecutionOfFlow` in `internal/feature/executions/executions_read.go` takes the first route and then guards the second: it reads the flow, answers `FLOW_NOT_FOUND` when the flow is absent, and answers `EXECUTION_NOT_FOUND` for an execution of another flow, because `FindExecutionByID` is scoped by organization and not by flow. Anchor's `GetLicense` takes the second route: `FindByOrganization(tenantID, productID, organizationID)` cannot return another organization's license, so no second guard is needed.

Give the parent its own error code only when the caller acts differently on it. The extra code costs a read of the parent that a parent-scoped child read would have skipped. Anchor pays that cost on the instantiate route, where `ORGANIZATION_NOT_FOUND` and `ORGANIZATION_LICENSE_NOT_FOUND` are distinct, and it pays it without an extra round trip: the parent 404 comes off a foreign-key violation on the insert, not off a lookup. Echopoint's `GetExecution` declines the cost and answers one undistinguished 404. Both are correct.

**A path-named resource answers 404 on every method.** RFC 9110 §15.5.5 attaches 404 to the target resource, and says nothing about the method. Anchor already follows this on an action: `POST /v1/products/{product_id}/licensing/templates/{license_template_id}/archive` declares a 404, the same status the `GET` on that template declares. The template is absent, so the target is absent. The router that matched the path did not know the method either.

There is a defensible house convention that says the opposite: an action call is not a fetch, so a missing target is the caller's mistake and earns a 400. Anchor and Echopoint reject it, for two reasons. It makes the status depend on a route's category, so every agent and every caller has to know which routes are actions. It also splits one absent resource across two statuses, so a client that retries on 404 has to learn a second rule per route. The specification does not settle this. The cost of the second rule does.

**A body-named or query-named entity answers 400.** The target resource exists; a value the caller supplied does not resolve. Anchor already states the rule in code, at `resolveLicenseTemplate` in `internal/service/organization_service.go` — creating an organization with an unknown `template_id` returns `ORGANIZATION_LICENSE_TEMPLATE_NOT_FOUND` as a 400, with this comment: "A bad request, not the 404 the license route answers: this call addressed the organization collection, which exists." The license route itself returns `LICENSE_TEMPLATE_NOT_FOUND` as a 404, because there the template is the path. One template, two statuses, one rule.

Echopoint applies the same rule to the launch body. `ResolveForFlowExecution` in `internal/feature/variables/service.go` returns `ENVIRONMENT_KEY_NOT_FOUND` as a 400 for an unknown `environment_key`.

Put the unresolved value in `Metadata`, so a form can point at the offending field without parsing prose.

**Tenant HTTP context is established once.** This rule is Echopoint's: Anchor does not carry the organization in the request context. Echopoint's `AuthMiddleware` validates and normalizes the organization after tenant Bearer/API-key authentication. Protected tenant handlers read `contextkeys.OrganizationID(ctx)` before writing a response; they do not repeat the fallible presence check. Missing context is a typed wiring invariant contained by that middleware, logged once and rendered as the existing generic 500. Other panics are rethrown. `RequireOrganizationID` remains for optional/background contexts and middleware validation. Public operations, `/me`, `/me/invitations`, `/init`, invitation acceptance, the global `/administrations/cloud-fleet` operations, and JobToken callbacks retain their distinct context contracts. Job-token runner callbacks use the accessor only inside the API-key branch. Actor, permission and resource-ownership checks remain separate; `AuthContext.ActorID()` selects the principal's ID without guaranteeing it is present.

**A broken server invariant answers 500.** RFC 9110 §15.5.1 makes 400 conditional on "something that is perceived to be a client error". When the server cannot read back a row it wrote itself, there is no client error to perceive. The caller named nothing, so there is nothing for the caller to correct, and a 400 tells them to correct it. A 404 is equally wrong: it reports the resource as absent when the transaction that created it committed.

`PublishTemplate` in `internal/email/service/service.go` holds both cases in one method. The first guard, in `lockTemplate`, is a 404, because the caller named `in.TemplateID` in the path:

```go
if found.IsAbsent() {
    return nil, ErrEmailTemplateNotFound
}
```

The second guard re-reads the version the transaction just marked published. The caller never named it:

```go
foundPublished, err := s.versionRepo.FindByID(txCtx, draft.ID)
if err != nil {
    return err
}
if foundPublished.IsAbsent() {
    return fmt.Errorf("publish: version %s missing after its status update", draft.ID)
}
```

Return a bare wrapped error, not a `fault`. `StrictErrorHandler` in the framework's `modules/httpserver` collapses an unmodelled error to a generic 500 and logs it at error level, which is what an invariant break needs. Reach for `fault.Internal` only when the 500 is an intentional, modelled outcome rather than a bug. Anchor's `ErrEmailDeliveryFailed` is that case: an email relay that refuses the handoff is a known operational failure, so it carries a stable code instead of the generic `UNEXPECTED_ERROR`. Declare that 500 in `openapi.yaml` as well.

**409 is a state you can retry past.** Use it when the request is well-formed, the target exists, and current state refuses it, and when a later or different request can succeed. Echopoint has four of this shape: `FLOW_VERSION_CONFLICT` (optimistic concurrency), `FLOW_TREE_CONFLICT` (another folder operation holds the lock), `IDEMPOTENCY_KEY_CONFLICT` (the key was reused with different parameters), and `RUNNER_JOB_EVENT_SEQUENCE_GAP` (events arrived out of order). Anchor's `PERMISSION_NAME_DUPLICATE` is the same shape on a uniqueness rule.

**A deleted resource answers 404, not 410.** RFC 9110 §15.5.11 makes 410 a statement that the removal is permanent and intentional. A server can only make that statement if it kept a record of the removal, and neither repo keeps tombstones for deleted rows. 410 would therefore have to be guessed from the absence of a row, which is the one thing it must not be.

Do not add 410, and do not add tombstones to reach it. It buys one thing — a client that stops retrying a URL — and every consumer of these APIs already treats 404 as terminal. Anchor's Go SDK does it in code: `Permanent()` in `clients/go/anchorsdk/errors.go` reports every 4xx apart from 429 as terminal, so a 410 would change nothing a caller does. Deleted reads as absent, and absent is 404.

**Validation is 400.** `validate.ValidateStruct` returns 400, neither OpenAPI contract declares a 422, and no call site uses `fault.Unprocessable` or `fault.Invalid`. Keep it that way. 422 is the more precise word for content that parses but breaks a semantic rule, and the price of that precision is a second status on every write route, a regenerated client for every consumer, and a rewrite of every component test that reads `resp.JSON400`. The `code` field already carries the distinction that 422 would carry. Do not add 422 to either contract.

One sentinel dissents. Anchor's `ErrEmailDeliveryRejected` is a 422, built with `fault.NewWithStatus`, for a relay that permanently refused the message. It is the only 422 in either repo, and no contract declares it, so it reaches a generated client as an unmodelled error. Under this rule it is a 400. Move it when a change touches `internal/email/service/service.go`.

Field-scoped detail belongs on the detail, not in the message. `fault.NewWithDetails` with a single `Detail{Code, Message, Field}` and `http.StatusBadRequest` is the sanctioned form — see `errLicenseFieldNameDuplicate` in Anchor's `internal/license/service/schema_service.go`.

**401 is who you are. 403 is what you may do.** Return 401 when the request carried no credential, or a credential that failed to authenticate — absent, malformed, expired, revoked, or wrong. Return 403 when authentication succeeded and the principal is not permitted. An expired session token is a 401, not a 403. A missing non-credential header is a 400, not a 401: the request is malformed, and no credential was judged.

RFC 9110 §15.5.2 requires a `WWW-Authenticate` header on every 401. Neither repo sends one: `fault.WriteJSON` writes a status, a `Content-Type`, and the error body, and no authentication challenge. This is a known deviation, and the fix belongs in the framework. Do not work around it one route at a time.

**A resource in another tenant answers 404.** Repository and service reads are tenant-scoped, so a row outside the caller's tenant returns nil and falls into the 404 row of the table. Keep it there. A 403 would confirm that the identifier names a real resource, which is exactly what the tenant boundary exists to hide.

**Use the semantic constructor.** Every status in the table has one, so new code never calls `fault.NewWithStatus`. Call `fault.NotFound`, `fault.BadRequest`, `fault.Unauthorized`, `fault.Forbidden`, `fault.Conflict`, `fault.TooManyRequests`, or `fault.Internal`. The constructor puts the decision in the call, where a reader and a reviewer see it, instead of in a `http.Status...` argument three lines below the code string. `fault.New`, `fault.NewBadRequest`, and `fault.NewBadRequestWithMetadata` are deprecated; new code does not call them.

Both repos carry `fault.NewWithStatus` call sites that predate this rule — 41 in Anchor, 68 in Echopoint. Leave them alone until you touch the surrounding code. When a change touches a sentinel, move it to the semantic constructor in the same change.

**Declare the status in the contract.** Every status a route can return is a response in `openapi.yaml`. Anchor's contract declares 400, 401, 403, and 404 only, while its services also return 409, 424, and 429. A status the contract omits reaches the generated client as an unmodelled error. Add the response in the same change that adds the status.

## Testing

### Component-test names

Describe every component-test scenario and its expected observable result in a plain-English `t.Run` name. Use descriptive names on existing subtests; table-driven tests use their case description or a readable name composed from the scenario and its variant. Keep fixture identifiers and request data separate from display names. Keep case names concise when fixtures derive validated resource names from `t.Name()`.

Keep top-level `Test…` identifiers stable for existing test filters. For a standalone CT, put its behavior in a named subtest and keep an existing top-level `t.Parallel()` and mirror it as the first statement of the new child, with its fixture setup inside the child. This retains parallel scheduling and satisfies the subtest parallelism checks. Preserve serial exceptions, fixture ownership, assertions, and existing subtest parallelism.

```go
func TestDeleteFlow(t *testing.T) {
    t.Parallel()
    t.Run("Deleting a flow removes its variables", func(t *testing.T) {
        t.Parallel()
        // Arrange, act, and assert the scenario here.
    })
}
```

The name is the behavior summary shown by Go's test runner; update it when the scenario or expected result changes. Use comments for non-obvious invariants and parallelism exceptions rather than duplicating the scenario name. This standard applies to existing and new CTs in both repositories.

### HTTP plumbing belongs in the test SDK

Each repo keeps a test SDK for its component tests — `apps/anchor/cmd/it/shared/dsl` in anchor, `cmd/it/ct/sdk` in echopoint. A test file holds what is about the behavior under test. The credential, the request, and the status assertions belong in the SDK.

**The SDK owns the credential.** The SDK mints the key and holds it on the handle it returns — `ProductContext.Organizations(scopes...)` in anchor. Name no scope, and the handle gets a key with every scope. Name scopes only when the authorization is the subject of the test. A test about a read must not list the scopes a read needs. That list is noise, and it goes stale on the next permission change.

**The SDK owns the request and the status.** One act builds the body, calls the generated client, and carries the `require.NoError` plus status plus `require.NotNil` triplet once. The act returns the decoded body, not the response. Call `t.Helper()` in every act, so a failure points at the line in the test file, not at the SDK.

**The SDK also owns the untouched response.** A test whose subject is a refusal needs the response with no assertion on it. Anchor gives each act a `*Raw` twin for that — `GetRaw` next to `Get`. Echopoint reaches the same escape hatch through `Client()` on the resource handle. Without one of the two, a refusal test builds the whole call again by hand.

**The test file keeps two things.** It keeps the request body that is specific to the feature under test. It also keeps a deliberate scope choice, when that choice is itself the assertion.

**Move the plumbing on the second test.** A test that builds a request, checks the error, and checks the status starts late on its own subject. Those same lines repeat in every other test of the route. Put them in the SDK as soon as a second test needs them.

## Verification

- After Go edits, follow the [Go lint procedure](development/testing.md#go-lint). No `//nolint` or other suppressions without prior approval.
- Run the narrowest relevant tests per the repo's test guide.

## Git

- Conventional Commits.
- Branches: tracked `<type>/<TICKET-ID>-<description>`, untracked `<type>/<description>`.
