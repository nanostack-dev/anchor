# anchorsdk — design notes

The product-facing SDK for Anchor. Wraps the generated OpenAPI client with a fluent, typed surface.

## Decisions already made

**Location.** `clients/go/anchorsdk/`, package `anchorsdk`, inside the existing `github.com/nanostack-dev/anchor/clients/go` module. Import path `github.com/nanostack-dev/anchor/clients/go/anchorsdk`.

Same module as the generated client it wraps, so the two version together — no second `go.mod` to keep in step. The directory name matches the package name, avoiding the `dir != package` mismatch linters flag. `anchorsdk` rather than `anchor` because consumers already import the generated client and often have their own `anchor` package (echopoint has `internal/clients/anchor`); a distinct name removes the collision.

**Standard library only.** No `nanostack-framework` import, ever. Anchor is source-available with a public Go client; consumers outside the Nanostack stack must be able to depend on this without inheriting the framework, and coupling the SDK's version to the framework's would force lockstep upgrades. This is why `retry.go` exists instead of using `pkg/retry`, and why errors are plain Go errors instead of `pkg/fault`.

**Scope: what a product *uses*, not what an admin *operates*.** In: email send, organizations, organization members, resources and the roles held on them, access evaluation, organization API keys (including validate/introspect), product users, and later licensing. Out: creating products, platform users, platform invitations, and the permission/role catalog and resource types — those belong to the admin UI and Terraform, and stay reachable via `Client.Raw()`.

**Product ID bound at construction.** Every product-scoped generated method takes `productId` as its first parameter. The SDK binds it once in `New` so it never appears at a call site.

**Fluent surface.** Builders for anything with more than two inputs; direct methods for the rest.

```go
c, err := anchorsdk.New(anchorsdk.Config{
    BaseURL:       "https://anchor.example.com",
    ProductID:     "prd_3iXYZ",
    ProductAPIKey: os.Getenv("ANCHOR_PRODUCT_API_KEY"),
})

err = c.Email().Template("welcome").To("a@b.com").Var("name", "Bob").Send(ctx)

org, err := c.Organizations().Create("Acme").Description("...").Do(ctx)

o := c.Organization(org.Id)
members, err := o.Members().List(ctx)
key, err := o.APIKeys().Create("ci").Permissions("flow:read").Do(ctx)
```

`Client.Organization(id)` returns an `*Org` handle exposing `Members()`, `Resource()`, `Resources()`, `APIKeys()`. That keeps the organization ID out of every sub-call.

**Errors.** Every method returns `*Error` on failure, carrying `Op`, `StatusCode`, and Anchor's structured `Details`. Classify with `errors.Is` against `ErrNotFound`, `ErrForbidden`, `ErrUnauthorized`, `ErrConflict`, `ErrInvalid`, `ErrPermanent`. `ErrPermanent` matches every 4xx except 429.

**Pagination.** Every search pages with an opaque keyset cursor. A search builder has `Cursor(string)`, and its result is the generated list response, which carries `NextCursor`. `All(ctx)` returns an `iter.Seq2[Item, error]` that follows `NextCursor` until it is absent: it never counts items, because only the cursor is exact. The iterator clones the builder's request for each page, so a builder can run `All` twice and a caller's `Offset` applies to the first page only (Anchor refuses a cursor combined with an offset). `Users.Organizations` reads every page of `SearchOrganizations`, so it stays complete for a user in many organizations. The SDK never builds or parses a cursor.

**Retry.** Transport errors, 5xx, and 429 are retried with exponential backoff and jitter (3 attempts, 200ms base, 2s cap). Every other 4xx returns on the first attempt. Jitter prevents concurrent callers recovering from one outage from re-converging on Anchor in lockstep.

`Error.forcePermanent` covers the case where a 2xx still means "retrying cannot help" — the email send returns 201 for a deduped replay of a previously FAILED record, so the record's own status is trusted over the HTTP code. That behaviour is ported from echopoint's existing wrapper and must be preserved.

## Built so far

| file | state |
|---|---|
| `doc.go` | done — package doc, usage, scope |
| `errors.go` | done — `Error`, sentinels, `decode`, `expectSuccess` |
| `retry.go` | done — `RetryPolicy`, backoff, jitter |
| `sdk.go` | done — `Config`, `Client`, `New`, options, `Org` handle |
| `email.go` | done — fluent send; the FAILED-record rule is preserved |
| `organizations.go` | done — create/update builders, get, delete, search, list |
| `members.go` | done — list, search, get (plus role permissions), add, remove, set role |
| `resources.go` | done — `Resource` handle: put (with `Under`), put and assign a member in one call (`Put(...).Assign`), get, delete, delete a tree; `Resources().Search()` |
| `resource_members.go` | done — roles on a resource: assign, get, remove, search |
| `access.go` | done — evaluate on an organization or a resource; `On(resource)`, `Can` |
| `apikeys.go` | done — create/update builders, get, delete, search, list, validate, `Client.Introspect` |
| `users.go` | done — create, get, delete, search, list, a user's organizations (`SearchOrganizations`, `Organizations` reads every page) |
| `paginate.go` | done — the cursor iterator behind every `All` |
| `events.go` | done — Standard Webhooks ingest, typed handlers, `Events(secret)` |
| `license.go` | partial — read (live), instantiate, opt-in `LicenseCache` (`license_cache.go`, fail-open), adjust, diff, report usage, `LimitUsage` decision value; usage series not wrapped |
| `README.md` | done — consumer-facing quickstart |
| `anchorsdk_test.go` | done — table-driven against an `httptest.Server` |
| `license_test.go` | done — decision-shaped routing, error classification, `LicenseCache` fail-open and stale-write guard |

Where the build departed from the plan:

- **`ptr` is gone.** The repo's `modernize` linter rejects it outright (`newexpr: ptr can be an inlinable wrapper around new(expr)`) and the module is on Go 1.26, so every call site uses the `new(expr)` builtin. `decode`, `expectSuccess`, `retrying`, `permanentError`, and `transportError` are all in use.
- **Product users have no update.** The generated client has no `UpdateProductUser` operation — Anchor does not expose one. That is documented on the `Users` type. Attach/detach is `Members().Add`/`Remove`, cross-referenced from `users.go`.
- **Three pre-existing lint findings were cleared** so the module lints at zero: `mnd` on `jitter`'s `d / 2` (now the `jitterSpread` const), `gosec` G101 on `productAPIKeyHeader` (renamed `productAuthHeader` — it names a header, it is not a credential), and `mnd` plus `revive` in the generated-client package's hand-written `config.go`. One suppression was added: `//nolint:gosec` on `rand.Int64N` in `jitter`, since backoff spread is a scheduling decision that needs no CSPRNG.
- **Tests split into a second file.** `license_test.go` sits alongside `anchorsdk_test.go` in the same `anchorsdk_test` package rather than growing the single file further — the suite crossed 1,100 lines and licensing's own scenarios (routing, error classification, the `LicenseCache` behaviour tests) read better grouped. Both files share the `stubServer`/`newTestClient`/`assertJSONEqual` helpers; `license_test.go` adds one local `newLicenseTestClient`, which fixes retries at one attempt so request counts are exact.

### Licensing specifics (`license.go`)

Built now, against what is merged plus PR #85's branch (`feat/65-usage-observations`):

- **`License.Get`** — every field's value, a plain live read with the retry policy. It is uncached. A hidden per-client cache made Echopoint serve licenses up to 30s stale on each replica after a change, so the cache is no longer built in.
- **`LicenseCache`** (`license_cache.go`) — the opt-in cache. `NewLicenseCache(client, store, policy)` reads through a `LicenseCacheStore` (`Load`, `Store`, `Invalidate`); a nil store means `NewMemoryLicenseCacheStore`. A product picks the store, so replicas can share one. Fail-open on a stale entry, as described below. `licensecachetest.RunStoreTests` is the conformance suite for a store.
- **Generation guard** — `Load` returns the organization's generation, `Store` writes only if it is unchanged, and `Invalidate` advances it. A live read that began before an `Invalidate` therefore cannot write its old result back. A failed `Load` disables the later `Store`, because the generation is unknown.
- **`License.Instantiate`**, **`License.Adjust`** (a builder: `Set`/`Values`) and **`Licensing.Migrate`** touch no cache. Call `LicenseCache.Invalidate` after them when the next cached read must see the change.
- **`License.Diff`** — uncached, matching the epic's "state and history are separate" split.
- **`License.ReportUsage`** — a builder (`From`/`To`) over PR #85's `ReportOrganizationUsage`. Anchor derives usage on every license read (ADR-0012), so `License.Get` shows a report at once; a `LicenseCache` shows it after its TTL or an `Invalidate`.

Judgment calls made without a human in the loop, flagged here rather than silently baked in:

- **Fail-open only covers non-permanent refresh failures.** A transport error, a 5xx, or 429-with-retries-exhausted serves the stale cache; a permanent 4xx (403, 404, a bad request) does not, even with `Strict` off. The epic's "an outage must not block a paying customer" is about outages specifically — serving frozen data forever behind a revoked key or a deleted organization seemed like the worse failure mode, and it reuses `Error.Permanent()` rather than inventing a second classification. Worth a reviewer's eye: this is a reading of intent, not a line the issue states explicitly.
- **30s default cache TTL.** Chosen, not derived from anything in the epic or ADRs — long enough that a hot-path check adds no per-request round trip, short enough that an out-of-band change is visible reasonably quickly. Overridable via `LicenseCachePolicy.TTL`.
- **`LimitUsage` is a value on the snapshot, not a `Check()` call.** With nanostack-dev/anchor#70 merged, the license read carries usage and a derived status, so stories 37–39 are answerable. They are answered by `LicenseSnapshot.Limit`/`Limits` returning a `LimitUsage` with `Remaining`, `Fraction`, and `Over` on it — no method named `Check`, which reads like a gate ADR-0001 forbids. `Remaining` and `Fraction` return `(value, ok)` so an unreported field cannot masquerade as zero headroom, and `Over` is false whenever there is no usage to compare, keeping a threshold warning from firing on absent data.
- **`Adjust()` takes no arguments**, unlike `Create`/`Update` elsewhere in this package which take their one unambiguous required field positionally. A license adjustment has no field that is always "the" field being changed, so forcing one into the call felt arbitrary; `Set`/`Values` on the builder carry the whole thing.

## Remaining work

- Reading the usage series. nanostack-dev/anchor#68 shipped `GetOrganizationUsageSeries` on the generated client; this facade does not wrap it yet.

## Generated-client facts worth knowing

- All ID types are `= string` aliases (`Ksuid`, `ProductIdParameter`, `OrganizationIdParameter`, …), so SDK signatures take plain `string`.
- Response types follow oapi-codegen convention: `Body []byte`, `HTTPResponse *http.Response`, `JSON200`/`JSON201`/…, and a `StatusCode()` method.
- Organization operations are named `…ProductOrganization…` (`CreateProductOrganizationWithResponse`, `GetProductOrganizationWithResponse`), not `…Organization…`.
- Request bodies are often aliases: `CreateProductOrganizationJSONRequestBody = ProductOrganizationRequest`, `AddOrganizationMemberJSONRequestBody = OrganizationMemberRequest`, and `CreateOrganizationAPIKeyJSONRequestBody = OrganizationAPIKeyCreateRequest`.
- `client.gen.go` and `types.gen.go` are generated. Never hand-edit them.
- **A declared-JSON status with an unparseable body destroys the whole response.** For any status the spec declares a JSON body for (`JSON400`, `JSON401`, `JSON403`, …), the generated parser unmarshals it and returns `(nil, err)` on failure — the status code is lost, so the SDK can only report a transport error and will retry what was really a permanent 4xx. Nothing the SDK can do about it from outside the generated client; it matters when writing tests, where every error stub must carry a real `ApiErrorResponse` body. An undeclared status (404, 409, 500, …) parses fine with any body.
- Not every operation declares the same error statuses. `GetProductOrganization` declares only 401/403; `SendEmail` declares only 400.

## Verify

```
cd clients/go && go build ./... && go test ./... && GOWORK=off go tool -modfile=../../apps/anchor/tools/golangci-lint/go.mod golangci-lint run --path-mode=abs --timeout 15m --fix
```

Done when build succeeds, tests pass, and lint exits 0.
