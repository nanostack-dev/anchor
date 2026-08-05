# anchorsdk — design notes

The product-facing SDK for Anchor. Wraps the generated OpenAPI client with a fluent, typed surface.

## Decisions already made

**Location.** `clients/go/anchorsdk/`, package `anchorsdk`, inside the existing `github.com/nanostack-dev/anchor/clients/go` module. Import path `github.com/nanostack-dev/anchor/clients/go/anchorsdk`.

Same module as the generated client it wraps, so the two version together — no second `go.mod` to keep in step. The directory name matches the package name, avoiding the `dir != package` mismatch linters flag. `anchorsdk` rather than `anchor` because consumers already import the generated client and often have their own `anchor` package (echopoint has `internal/clients/anchor`); a distinct name removes the collision.

**Standard library only.** No `nanostack-framework` import, ever. Anchor is source-available with a public Go client; consumers outside the Nanostack stack must be able to depend on this without inheriting the framework, and coupling the SDK's version to the framework's would force lockstep upgrades. This is why `retry.go` exists instead of using `pkg/retry`, and why errors are plain Go errors instead of `pkg/fault`.

**Scope: what a product *uses*, not what an admin *operates*.** In: email send, organizations, organization members, workspaces, organization API keys (including validate/introspect), product users, and later licensing. Out: creating products, platform users, platform invitations, and the permission/role catalog — those belong to the admin UI and Terraform, and stay reachable via `Client.Raw()`.

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

`Client.Organization(id)` returns an `*Org` handle exposing `Members()`, `Workspaces()`, `APIKeys()`. That keeps the organization ID out of every sub-call.

**Errors.** Every method returns `*Error` on failure, carrying `Op`, `StatusCode`, and Anchor's structured `Details`. Classify with `errors.Is` against `ErrNotFound`, `ErrForbidden`, `ErrUnauthorized`, `ErrConflict`, `ErrInvalid`, `ErrPermanent`. `ErrPermanent` matches every 4xx except 429.

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
| `workspaces.go` | done — create, get, update, delete, search, list |
| `apikeys.go` | done — create/update builders, get, delete, search, list, validate, `Client.Introspect` |
| `users.go` | done — create, get, delete, search, list, a user's organizations |
| `README.md` | done — consumer-facing quickstart |
| `anchorsdk_test.go` | done — table-driven against an `httptest.Server` |

Where the build departed from the plan:

- **`ptr` is gone.** The repo's `modernize` linter rejects it outright (`newexpr: ptr can be an inlinable wrapper around new(expr)`) and the module is on Go 1.26, so every call site uses the `new(expr)` builtin. `decode`, `expectSuccess`, `retrying`, `permanentError`, and `transportError` are all in use.
- **Workspaces gained `Search`/`List`.** The plan listed only CRUD, but Anchor exposes the search endpoint and every sibling facade wraps one; omitting it made `Workspaces` the odd facade out.
- **Product users have no update.** The generated client has no `UpdateProductUser` operation — Anchor does not expose one. That is documented on the `Users` type. Attach/detach is `Members().Add`/`Remove`, cross-referenced from `users.go`.
- **Three pre-existing lint findings were cleared** so the module lints at zero: `mnd` on `jitter`'s `d / 2` (now the `jitterSpread` const), `gosec` G101 on `productAPIKeyHeader` (renamed `productAuthHeader` — it names a header, it is not a credential), and `mnd` plus `revive` in the generated-client package's hand-written `config.go`. One suppression was added: `//nolint:gosec` on `rand.Int64N` in `jitter`, since backoff spread is a scheduling decision that needs no CSPRNG.

## Remaining work

Licensing, as `license.go`, once its design is settled.

## Generated-client facts worth knowing

- All ID types are `= string` aliases (`Ksuid`, `ProductIdParameter`, `OrganizationIdParameter`, …), so SDK signatures take plain `string`.
- Response types follow oapi-codegen convention: `Body []byte`, `HTTPResponse *http.Response`, `JSON200`/`JSON201`/…, and a `StatusCode()` method.
- Organization operations are named `…ProductOrganization…` (`CreateProductOrganizationWithResponse`, `GetProductOrganizationWithResponse`), not `…Organization…`.
- Request bodies are often aliases: `CreateProductOrganizationJSONRequestBody = ProductOrganizationRequest`, `AddOrganizationMemberJSONRequestBody = OrganizationMemberRequest`, `CreateOrganizationAPIKeyJSONRequestBody = OrganizationAPIKeyCreateRequest`, `CreateOrganizationWorkspaceJSONRequestBody = ProductWorkspaceRequest`.
- `client.gen.go` and `types.gen.go` are generated. Never hand-edit them.
- **A declared-JSON status with an unparseable body destroys the whole response.** For any status the spec declares a JSON body for (`JSON400`, `JSON401`, `JSON403`, …), the generated parser unmarshals it and returns `(nil, err)` on failure — the status code is lost, so the SDK can only report a transport error and will retry what was really a permanent 4xx. Nothing the SDK can do about it from outside the generated client; it matters when writing tests, where every error stub must carry a real `ApiErrorResponse` body. An undeclared status (404, 409, 500, …) parses fine with any body.
- Not every operation declares the same error statuses. `GetProductOrganization` declares only 401/403; `SendEmail` declares only 400.

## Verify

```
cd clients/go && go build ./... && go test ./... && golangci-lint run --path-mode=abs --timeout 5m --fix
```

Done when build succeeds, tests pass, and lint exits 0.
