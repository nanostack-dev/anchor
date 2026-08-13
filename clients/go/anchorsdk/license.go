package anchorsdk

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// License is the facade for one organization's license. Obtain one from an
// [Org] handle:
//
//	license, err := c.Organization(orgID).License().Get(ctx)
//
// Every organization has exactly one license, so unlike [Members],
// [Workspaces], and [APIKeys] this facade has no list or search.
//
// # What this does not do
//
// This facade returns values, never a verdict. It has no method that blocks,
// panics, or returns an error meaning "denied": Anchor validates a license
// write but never gates an action on a license read, and the SDK preserves
// that boundary. The statement that stops a user belongs in the caller's own
// repository — see ADR-0001 (docs/adr/0001-anchor-validates-but-never-gates.md).
//
// A derived per-limit status (within_limit, at_limit, exceeded, stale), and
// therefore a decision value carrying it plus a threshold predicate, are not
// exposed here yet: Anchor does not return status on a license read until
// nanostack-dev/anchor#70 ships, and the usage series a caller would need to
// compute one independently is nanostack-dev/anchor#68. [License.Get] returns
// every field's value today; status will land on
// [nanoclient.OrganizationLicenseResponse] itself once #70 ships, so no shape
// change is expected on this side.
type License struct{ o *Org }

// License returns the license facade for this organization.
func (o *Org) License() License { return License{o: o} }

// defaultLicenseCacheTTL bounds how long [License.Get] serves a cached read
// before attempting a live refresh. 30s keeps a hot-path, permission-style
// check from adding a network round trip to every request, while keeping a
// template swap or an out-of-band Adjust visible reasonably quickly.
const defaultLicenseCacheTTL = 30 * time.Second

// LicenseCachePolicy controls how [License.Get] caches a license read. The
// zero value uses the defaults: a 30s TTL, and fail-open on a stale entry.
//
// The cache lives on the [Client], keyed by organization ID, and is written
// with the fresh value — not merely evicted — by every successful
// [License.Get], [License.Instantiate], and [License.Adjust] made through
// that client, so a write is visible to the very next read without another
// round trip.
type LicenseCachePolicy struct {
	// TTL bounds how long a cached read is served without a live refresh.
	// Zero uses the default.
	TTL time.Duration

	// Strict disables fail-open. By default, once TTL has elapsed, a live
	// refresh that fails with a transient error (a transport failure, a 5xx,
	// or 429 with retries exhausted) still serves the last known value —
	// marked Stale on [LicenseSnapshot] — rather than returning an error, so
	// an Anchor outage does not block a paying customer; see ADR-0001. A
	// permanent failure (any other 4xx: a bad request, a revoked key, an
	// organization that no longer exists) is never papered over regardless of
	// Strict, because unlike an outage it will not resolve on its own.
	//
	// Set Strict to opt into fail-closed: a failed refresh past TTL always
	// returns the error, even one that fail-open would otherwise absorb.
	Strict bool
}

// ttl resolves the effective TTL, applying the default for the zero value.
func (p LicenseCachePolicy) ttl() time.Duration {
	if p.TTL <= 0 {
		return defaultLicenseCacheTTL
	}
	return p.TTL
}

// LicenseSnapshot is a cached read of an organization's license, returned by
// [License.Get].
type LicenseSnapshot struct {
	*nanoclient.OrganizationLicenseResponse

	// Stale reports whether this snapshot was served from cache after a live
	// refresh failed, per [LicenseCachePolicy]'s fail-open default. It still
	// carries the last values Anchor confirmed; consult Stale to decide
	// whether to log or warn, not to deny anything — the SDK itself never
	// does.
	Stale bool
	// FetchedAt is when Anchor last confirmed these values, not when this
	// call returned them. It moves only on a successful live refresh.
	FetchedAt time.Time
}

// licenseCache holds one cached license read per organization ID, for a
// single [Client]. Safe for concurrent use.
type licenseCache struct {
	mu      sync.Mutex
	entries map[string]licenseCacheEntry
}

// licenseCacheEntry is one organization's last known license read.
type licenseCacheEntry struct {
	value     *nanoclient.OrganizationLicenseResponse
	fetchedAt time.Time
}

func newLicenseCache() *licenseCache {
	return &licenseCache{entries: make(map[string]licenseCacheEntry)}
}

// fresh returns a snapshot for organizationID when its cached entry is younger
// than ttl.
func (c *licenseCache) fresh(organizationID string, ttl time.Duration) (*LicenseSnapshot, bool) {
	c.mu.Lock()
	entry, ok := c.entries[organizationID]
	c.mu.Unlock()

	if !ok || time.Since(entry.fetchedAt) >= ttl {
		return nil, false
	}
	return &LicenseSnapshot{OrganizationLicenseResponse: entry.value, FetchedAt: entry.fetchedAt}, true
}

// stale returns organizationID's last known snapshot regardless of age,
// marked Stale, for [License.Get] to fall back to when a live refresh fails.
func (c *licenseCache) stale(organizationID string) (*LicenseSnapshot, bool) {
	c.mu.Lock()
	entry, ok := c.entries[organizationID]
	c.mu.Unlock()

	if !ok {
		return nil, false
	}
	return &LicenseSnapshot{OrganizationLicenseResponse: entry.value, Stale: true, FetchedAt: entry.fetchedAt}, true
}

// store records value as the freshest known license for organizationID,
// timestamped now, and returns the snapshot [License.Get] should hand back.
func (c *licenseCache) store(organizationID string, value *nanoclient.OrganizationLicenseResponse) *LicenseSnapshot {
	now := time.Now()

	c.mu.Lock()
	c.entries[organizationID] = licenseCacheEntry{value: value, fetchedAt: now}
	c.mu.Unlock()

	return &LicenseSnapshot{OrganizationLicenseResponse: value, FetchedAt: now}
}

// delete clears any cached entry for organizationID, so the next Get performs
// a live fetch regardless of TTL.
func (c *licenseCache) delete(organizationID string) {
	c.mu.Lock()
	delete(c.entries, organizationID)
	c.mu.Unlock()
}

// Get returns the organization's current license: every field the product's
// schema declares, with its value. A limit field carries no usage or status
// yet — Anchor computes neither on a license read until #70 ships (see
// [License]) — so a consumer wanting to compare a value against a limit today
// must already know its own field's ceiling and track its own usage.
//
// The read is cached per [LicenseCachePolicy]; see [License] and
// [LicenseCachePolicy] for the fail-open behaviour on a stale entry. Classify
// a returned error exactly as any other SDK call, with [errors.Is] against
// the package sentinels — Get itself never blocks or denies.
func (l License) Get(ctx context.Context) (*LicenseSnapshot, error) {
	const op = "License.Get"

	c := l.o.c

	if snap, ok := c.licenses.fresh(l.o.id, c.licensePolicy.ttl()); ok {
		return snap, nil
	}

	fresh, err := retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationLicenseResponse, error) {
		resp, err := c.api.GetOrganizationLicenseWithResponse(ctx, c.productID, l.o.id)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
	if err != nil {
		if !c.licensePolicy.Strict && !errors.Is(err, ErrPermanent) {
			if snap, ok := c.licenses.stale(l.o.id); ok {
				return snap, nil
			}
		}
		return nil, err
	}

	return c.licenses.store(l.o.id, fresh), nil
}

// Invalidate clears the cached license read for this organization, so the
// next Get performs a live fetch. [License.Instantiate] and [License.Adjust]
// already refresh the cache automatically when the write happens through this
// client; call Invalidate when the license may have changed some other way —
// another service, another [Client] instance, the admin UI.
func (l License) Invalidate() { l.o.c.licenses.delete(l.o.id) }

// Instantiate creates the organization's license as a copy of templateID's
// current values. Anchor stamps template_id and instantiated_at as
// provenance; editing the template afterwards does not reach this
// organization. There is no re-instantiate route — moving an organization to
// a different template is a sequence of [License.Adjust] calls, one field at
// a time, not a single call.
func (l License) Instantiate(
	ctx context.Context,
	templateID string,
) (*nanoclient.OrganizationLicenseResponse, error) {
	const op = "License.Instantiate"

	c := l.o.c
	body := nanoclient.OrganizationLicenseInstantiateRequest{TemplateId: templateID}

	license, err := retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationLicenseResponse, error) {
		resp, err := c.api.InstantiateOrganizationLicenseWithResponse(ctx, c.productID, l.o.id, body)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
	if err != nil {
		return nil, err
	}

	c.licenses.store(l.o.id, license)
	return license, nil
}

// Adjust starts a bespoke change to one or more of this organization's
// license fields, for an arrangement that does not deserve a new template.
//
//	license, err := org.License().Adjust().
//	    Set("max_flows", 500).
//	    Do(ctx)
//
// Adjust merges into the license: a field set here replaces its value, and a
// field left unset keeps its current one. No field can be removed this way —
// every field the schema declares must stay set — and the merged result is
// validated against the schema exactly as a template write is.
func (l License) Adjust() *LicenseAdjustBuilder {
	return &LicenseAdjustBuilder{
		o:   l.o,
		req: nanoclient.OrganizationLicenseAdjustRequest{Values: nanoclient.LicenseTemplateValues{}},
	}
}

// LicenseAdjustBuilder accumulates a license adjustment. Setter methods
// chain; [LicenseAdjustBuilder.Do] sends. A builder is single-use and not
// safe for concurrent mutation.
type LicenseAdjustBuilder struct {
	o   *Org
	req nanoclient.OrganizationLicenseAdjustRequest
}

// Set changes a single license field.
func (b *LicenseAdjustBuilder) Set(key string, value any) *LicenseAdjustBuilder {
	b.req.Values[key] = value
	return b
}

// Values merges every entry from m into the adjustment. A nil or empty map is
// a no-op.
func (b *LicenseAdjustBuilder) Values(m map[string]any) *LicenseAdjustBuilder {
	maps.Copy(b.req.Values, m)
	return b
}

// Do applies the adjustment and returns the license as it now stands.
func (b *LicenseAdjustBuilder) Do(ctx context.Context) (*nanoclient.OrganizationLicenseResponse, error) {
	const op = "License.Adjust"

	c := b.o.c

	license, err := retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationLicenseResponse, error) {
		resp, err := c.api.AdjustOrganizationLicenseWithResponse(ctx, c.productID, b.o.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
	if err != nil {
		return nil, err
	}

	c.licenses.store(b.o.id, license)
	return license, nil
}

// Diff reports how this organization's license differs from the template it
// was instantiated from, one field at a time. The template always resolves,
// archived or deleted, so this answers for the life of the license.
func (l License) Diff(ctx context.Context) (*nanoclient.OrganizationLicenseDiffResponse, error) {
	const op = "License.Diff"

	c := l.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.OrganizationLicenseDiffResponse, error) {
		resp, err := c.api.GetOrganizationLicenseDiffWithResponse(ctx, c.productID, l.o.id)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON200)
	})
}

// ReportUsage starts reporting this organization's usage for a license field
// as an absolute snapshot — "this organization now has 37 flows" — never a
// delta. Report as often as you like; Anchor stores every report and never
// sums them, so a retried report cannot double-count and one that never
// arrived corrects itself on the next.
//
//	_, err := org.License().ReportUsage("max_flows", 37).Do(ctx)
//
//	_, err := org.License().ReportUsage("monthly_runs", 412).
//	    From(periodStart).
//	    Do(ctx)
//
// key must name a limit field the product's license schema declares — an
// unknown key or a non-limit field is refused, not silently dropped. A report
// is accepted even when value exceeds the field's own limit: refusing it
// would make the exceeded status unreachable, and Anchor would keep serving a
// value that reads within_limit for an organization genuinely past its
// ceiling.
//
// Reporting usage does not touch the [License.Get] cache: usage is recorded
// separately from a license's own field values, and today's license read
// carries no usage or status to invalidate (see [License]).
func (l License) ReportUsage(key string, value float64) *UsageReportBuilder {
	return &UsageReportBuilder{o: l.o, req: nanoclient.UsageReportRequest{Key: key, Value: value}}
}

// UsageReportBuilder accumulates a single usage report. Setter methods chain;
// [UsageReportBuilder.Do] sends. A builder is single-use and not safe for
// concurrent mutation.
type UsageReportBuilder struct {
	o   *Org
	req nanoclient.UsageReportRequest
}

// From starts a windowed counter's period: "412 runs since August 14."
// Without To, Anchor fills the end in as now and returns it, so a reader
// never meets half a window. Without From at all, the report is a gauge: "37
// flows exist right now," a value that rises and falls.
func (b *UsageReportBuilder) From(t time.Time) *UsageReportBuilder {
	b.req.From = new(t)
	return b
}

// To closes the windowed counter's period explicitly. Anchor refuses To sent
// without From — there is then nothing to run from — and refuses a window
// longer than a year.
func (b *UsageReportBuilder) To(t time.Time) *UsageReportBuilder {
	b.req.To = new(t)
	return b
}

// Do sends the usage report. Usage needs no license: an organization can
// report before it is on a tier and keeps reporting after one is withdrawn.
func (b *UsageReportBuilder) Do(ctx context.Context) (*nanoclient.UsageObservationResponse, error) {
	const op = "License.ReportUsage"

	c := b.o.c

	return retrying(ctx, c, func(ctx context.Context) (*nanoclient.UsageObservationResponse, error) {
		resp, err := c.api.ReportOrganizationUsageWithResponse(ctx, c.productID, b.o.id, b.req)
		if err != nil {
			return nil, transportError(op, err)
		}
		return decode(op, resp.StatusCode(), resp.Body, resp.JSON201)
	})
}
