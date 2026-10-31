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
| `c.Users()` | create, get, delete, search, list, a user's organizations (`SearchOrganizations` pages them) |
| `c.Introspect(ctx, rawKey, scopes...)` | resolve an organization API key without knowing its org |
| `org.Members()` | list, search, get, add, remove, set role |
| `org.Invitations()` | create, list, search by status and email, get, update, delete, accept |
| `c.Invitations()` | search every organization of the product by status and email |
| `org.Resource(type, id)` | put (register, rename, move), get, delete, delete a tree, `Members()`: assign, get, remove, search |
| `org.Resources()` | search by type, ID, name and parent |
| `c.AccessForUser(id)`, `c.AccessForAPIKey(raw)` | evaluate on an organization or a resource: `On(resource).Can(ctx, perms...)` |
| `org.APIKeys()` | create, get, update, delete, search, list, validate |
| `org.License()` | get (cached), instantiate, adjust, diff, report usage |
| `anchorsdk.Events(secret)` | typed Standard Webhooks ingest (`http.Handler`) |

### Product events

Anchor POSTs signed Standard Webhooks to the product event endpoint. Verify and
dispatch in a few lines:

```go
h, err := anchorsdk.Events(os.Getenv("ANCHOR_WEBHOOK_SECRET"))
if err != nil {
	slog.Error("webhook handler", "err", err)
	os.Exit(1)
}
http.Handle("/webhooks/anchor", h.OrganizationCreated(
	func(ctx context.Context, e anchorsdk.OrganizationCreated) error {
		slog.Info("organization created", "id", e.OrganizationID)
		return nil
	},
).ProductUserDeleted(func(ctx context.Context, e anchorsdk.ProductUserDeleted) error {
	return removeUser(ctx, e.ProductUserID)
}))
```

`EventTypes()` lists the catalog. Unknown types still return 200 after `OnAny`.

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

A product that is its own identity provider sends its own ID for the user on create, and then
evaluates access by that ID:

```go
user, err := c.Users().Create(email).Name(name).ExternalID(accountID).Do(ctx)
_, err = o.Members().Add(ctx, user.Id, roleID)

decision, err := c.AccessForUser(accountID).InOrganization(o.ID()).Permissions("flows:read").Do(ctx)
```

The external ID is printable ASCII of at most 255 characters, unique within the product (a duplicate
matches `ErrConflict`), and cannot change after the create. A Clerk integration of the same product
uses the same IDs, and its events overwrite the email and name of a user created with a Clerk user ID.

### Resources and access

A resource is a thing the product protects, of a type the product declared, named by the product's
own ID. A handle calls nothing until a method sends:

```go
ws := o.Resource("workspace", workspaceID)
_, err = ws.Put("Default").Do(ctx)                                 // under the organization
_, err = o.Resource("folder", folderID).Put("Specs").Under("workspace", workspaceID).Do(ctx)
_, err = ws.Members().Assign(ctx, productUserID, editorRoleID)    // pusr_...; replaces their role there

// Share a document Anchor does not know yet: register it under its folder and give the member a role
// on it, in one transaction. The API key needs resource:write besides resource_member:write.
_, err = o.Resource("document", docID).Put("Pricing page").Under("folder", folderID).
    Assign(ctx, productUserID, viewerRoleID)

ok, err := c.AccessForUser(externalUserID).On(ws).Can(ctx, "flow:run")
decision, err := c.AccessForAPIKey(rawKey).On(ws).Permissions("flow:run", "flow:edit").Do(ctx)

for r, err := range o.Resources().Search().Types("workspace").All(ctx) { ... }
```

Every call is retried on transient failures: `Put`, `Assign` and `Put(...).Assign` change nothing when sent again, and
a retried delete whose first attempt succeeded answers success. `Delete` refuses a resource that
holds another resource or a role (`ErrConflict`); `DeleteTree` removes them too. `Can` needs at
least one permission and answers false on a denial; `Do` returns the decision with the reason.

### Invitations

```go
created, err := o.Invitations().Create("alice@example.com", roleID).Do(ctx)
// Anchor sends no email and issues no token: tell the person yourself.
pending, err := o.Invitations().Search().Statuses(nanoclient.Pending).Do(ctx)

// When a person signs in, list their invitations by their VERIFIED email addresses.
mine, err := c.Invitations().Search().Emails(verified...).Statuses(nanoclient.Pending).Do(ctx)
_, err = c.Organization(mine.Items[0].OrganizationId).Invitations().
	Accept(ctx, mine.Items[0].Id, productUserID)
```

Update works only on a pending invitation that has not expired; to give an invitation more time,
update its expiry. Accept needs an existing product user and does not compare email addresses: the
product checks that the invitation email is one of the person's verified addresses before it
calls accept.

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

### Licensing

```go
license, err := o.License().Get(ctx)     // every field's value, cached
license.Values["max_flows"]              // any, per the product's license schema

_, err = o.License().Instantiate(ctx, templateID)

_, err = o.License().Adjust().
	Set("max_flows", 500).
	Do(ctx)

_, err = o.License().ReportUsage("max_flows", 37).Do(ctx)          // a gauge
_, err = o.License().ReportUsage("monthly_runs", 412).
	From(periodStart).
	Do(ctx)                                                         // an open windowed counter
```

`License.Get` never blocks and never returns an error meaning "denied" — it returns values, and the
statement that stops a user belongs in your own repository (ADR-0001 in the Anchor repo). The read is
cached per organization and, on a failed refresh past the cache TTL, fails open by default: the last
known values are served (`LicenseSnapshot.Stale` is `true`) rather than an error, so an Anchor outage
does not block a paying customer. A permanent failure (a revoked key, a deleted organization) is
never papered over this way. Opt into fail-closed instead:

```go
c, err := anchorsdk.New(anchorsdk.Config{
	// ...
	LicenseCache: anchorsdk.LicenseCachePolicy{TTL: 10 * time.Second, Strict: true},
})
```

`Instantiate` and `Adjust` write straight through to the cache, so the very next `Get` on that
organization needs no round trip. Call `o.License().Invalidate()` if the license changed some other
way — another service, the admin UI.

A license read carries, per limit field, the latest reported usage and the status Anchor derives from
it — `within_limit`, `at_limit`, `exceeded`, `stale`. `LicenseSnapshot.Limit` returns one as a
decision value, with the headroom arithmetic and a threshold predicate already on it:

```go
snapshot, err := o.License().Get(ctx)

limit, ok := snapshot.Limit("max_flows")     // ok is false for a field that is not a limit
limit.Status                                 // nanoclient.LicenseUsageStatus
remaining, known := limit.Remaining()        // known is false when nothing was ever reported
if limit.Over(0.8) {                         // false when there is no usage to compare
	// prompt an upgrade before the ceiling, not at it
}

for _, limit := range snapshot.Limits() {    // every limit field, sorted by key
	// ...
}
```

`stale` means the field has never been reported against, so there is no current number to trust. It
reads "unknown", not "breached" — do not treat it as a denial.

Usage travels inside the cached snapshot. Anchor recomputes it on every read, but a cached snapshot
was computed when it was fetched, so a `ReportUsage` call reaches `Get` only on the next live
refresh. Call `o.License().Invalidate()` between the two to read back what you just reported.

**Not yet available:** reading the usage series
([`GET .../usage/series`](https://github.com/nanostack-dev/anchor/issues/68) shipped, but this facade
does not wrap it yet).

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

### Reading every page

A search pages with an opaque cursor. Every search builder has `Cursor(string)`, and every result carries
`NextCursor`, which is nil on the last page. `All(ctx)` follows it for you and yields each item:

```go
for org, err := range c.Organizations().Search().Query("acme").Limit(100).All(ctx) {
	if err != nil {
		return err
	}
	fmt.Println(org.Name)
}
```

To walk the pages yourself, send each `NextCursor` back and stop when it is nil. Never stop on `Count < Limit`
or on `Total`.

```go
search := c.Organizations().Search().Limit(100)
for {
	page, err := search.Do(ctx)
	if err != nil {
		return err
	}
	// use page.Items
	if page.NextCursor == nil {
		break
	}
	search.Cursor(*page.NextCursor)
}
```

A cursor belongs to one filter and one sort, so keep both unchanged while you walk. Anchor refuses a cursor
that was issued for another sort, one that is malformed, and one sent together with `Offset`, each with a 400
(`ErrInvalid`). Use `Offset` only to jump to a page number in a table, and iterate with the cursor.

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
