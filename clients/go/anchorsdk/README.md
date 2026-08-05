# anchorsdk

The product-facing Go SDK for [Anchor](https://github.com/nanostack-dev/anchor). It wraps the
generated OpenAPI client with a fluent, typed surface for what a product backend does at runtime:
send transactional email, manage its organizations and their members, issue organization API keys,
and resolve inbound credentials.

Depends only on the standard library and the generated client — no framework, no other modules.

```bash
go get github.com/nanostack-dev/anchor/clients/go@latest
```

## Quickstart

```go
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/nanostack-dev/anchor/clients/go/anchorsdk"
)

func main() {
	c, err := anchorsdk.New(anchorsdk.Config{
		BaseURL:       "https://anchor.example.com",
		ProductID:     "prd_3iXYZ",
		ProductAPIKey: os.Getenv("ANCHOR_PRODUCT_API_KEY"),
	})
	if err != nil {
		slog.Error("anchor client", "err", err)
		os.Exit(1)
	}

	ctx := context.Background()

	org, err := c.Organizations().Create("Acme").
		Description("Leading provider of innovative solutions").
		Meta("billing_ref", "cust_abc123").
		Do(ctx)
	if err != nil {
		slog.Error("create organization", "err", err)
		os.Exit(1)
	}

	slog.Info("created", "org", org.Id)
}
```

The product ID is bound once at construction, so it never appears at a call site.

## Configuration

| field | required | default |
|---|---|---|
| `BaseURL` | yes | — |
| `ProductID` | yes | — |
| `ProductAPIKey` | yes | — sent as `X-Product-Api-Key` |
| `HTTPClient` | no | `&http.Client{Timeout: 10 * time.Second}` |
| `Retry` | no | 3 attempts, 200ms base delay, 2s cap |

`WithRequestEditor` adds a per-request hook (tracing headers, a tenant hint) applied after the SDK's
own authentication header. `WithClientOption` passes a generated-client option straight through.

## Facades

`Client` exposes product-scoped facades directly, and everything scoped to a single organization
through an `Org` handle so the organization ID is bound once:

```go
org := c.Organization("org_3iXYZ")
```

| entry point | operations |
|---|---|
| `c.Email()` | fluent send |
| `c.Organizations()` | create, get, update, delete, search, list |
| `c.Users()` | create, get, delete, search, list, a user's organizations |
| `c.Introspect(ctx, rawKey, scopes...)` | resolve an organization API key without knowing its org |
| `org.Members()` | list, search, get, add, remove, set role |
| `org.Workspaces()` | create, get, update, delete, search, list |
| `org.APIKeys()` | create, get, update, delete, search, list, validate |

### Email

```go
err := c.Email().
	Template("welcome").          // or TemplateID("etpl_...")
	To("new.user@company.com").
	ToName("Bob").
	Dedupe(signupID).             // Anchor de-duplicates per (product, key)
	Var("plan", "pro").
	Vars(map[string]any{"seats": 3}).
	Send(ctx)
```

A `201` does not mean delivered: a deduped replay of a previously FAILED record also returns `201`.
The SDK trusts the persisted record's own status over the HTTP code and reports that case as a
**permanent** failure, so a caller holding a lock does not spin on it.

### Organizations and members

```go
org, err := c.Organizations().Create("Acme").
	FoundingMember(userID, roleID).   // organization + membership in one transaction
	Do(ctx)

o := c.Organization(org.Id)

members, err := o.Members().List(ctx)
_, err = o.Members().Add(ctx, userID, roleID)
_, err = o.Members().SetRole(ctx, userID, otherRoleID)
err = o.Members().Remove(ctx, userID)
```

A product user exists at product scope; membership is a separate grant. Create users with
`c.Users()`, then attach them with `Members().Add`. Anchor exposes no update operation for a product
user.

### API keys

```go
key, err := o.APIKeys().Create("ci").
	Description("CI runner").
	Permissions("flow:read", "flow:execute").   // required, and immutable afterwards
	ExpiresIn(90 * 24 * time.Hour).
	Do(ctx)

// key.Value is the clear key — Anchor never discloses it again.
```

On an inbound request, resolve the caller's key:

```go
result, err := c.Introspect(ctx, rawKey, "flow:read")
```

A key that is valid but lacks a required scope yields `403`, so the error matches `ErrForbidden`;
the response body naming the missing scopes is kept on `Error.Body`.

### Searching

Every search facade is a builder over the same shape:

```go
page, err := c.Organizations().Search().
	Query("acme").
	Names("Acme Corporation").
	SortBy(nanoclient.ProductOrganizationSearchRequestSortByName, nanoclient.ASC).
	Limit(20).
	Offset(40).
	Do(ctx)
```

`List(ctx)` is shorthand for `Search().Do(ctx)`.

## Errors

Every method returns `*anchorsdk.Error` on failure, carrying the operation name, the HTTP status,
and Anchor's structured details.

```go
if errors.Is(err, anchorsdk.ErrNotFound) {
	// ...
}

var apiErr *anchorsdk.Error
if errors.As(err, &apiErr) {
	for _, d := range apiErr.Details {
		slog.Warn("anchor", "code", d.Code, "field", d.Field, "message", d.Message)
	}
}
```

| sentinel | matches |
|---|---|
| `ErrInvalid` | 400, 422 |
| `ErrUnauthorized` | 401 |
| `ErrForbidden` | 403 |
| `ErrNotFound` | 404 |
| `ErrConflict` | 409 |
| `ErrPermanent` | every 4xx except 429, plus terminal outcomes returned with a 2xx |

## Retry

Transport failures, 5xx, and 429 are retried with exponential backoff and jitter. Every other 4xx
returns on the first attempt. Jitter keeps concurrent callers recovering from one outage from
re-converging on Anchor in lockstep.

```go
c, err := anchorsdk.New(anchorsdk.Config{
	// ...
	Retry: anchorsdk.RetryPolicy{MaxAttempts: 1}, // disable retrying
})
```

`errors.Is(err, ErrPermanent)` is the signal for an outer retry loop to stop; the SDK's own loop
already does.

## Scope

This SDK covers what a product *uses*. Platform administration — creating products, managing
platform users and invitations, maintaining the permission and role catalog — is operated through
the admin UI and Terraform. Those operations stay reachable on the generated client:

```go
raw := c.Raw() // loses the SDK's retry and error classification
```
