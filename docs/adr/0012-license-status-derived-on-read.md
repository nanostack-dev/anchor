# ADR-0012: License status is derived on read, cached separately from usage

**Status:** Accepted

## Context

[Issue #62](https://github.com/nanostack-dev/anchor/issues/62) already decided the shape: the license read carries, per limit field, the latest reported usage and a derived status — `within_limit`, `at_limit`, `exceeded`, `stale` — computed on read and stored nowhere. This ADR records the two decisions that issue left open: what `stale` means when there is no usage to compare, and how a cached read stays consistent with usage that was never meant to be cached.

## Decision: a limit that has never reported is `stale`

Comparing the latest observation against the limit only works when a latest observation exists. Nothing in [issue #62](https://github.com/nanostack-dev/anchor/issues/62) says what a limit with zero observations should read as.

**A limit with no observation on record reads `stale`, with `usage: null` and `last_reported_at: null`, whether or not the field declares an expected reporting interval.** Staleness from *age* — the latest report is older than the field's declared interval — is only reachable when that interval is declared, since Anchor declares what it expects and never pulls a report on its own. Staleness from *absence* needs no such declaration: "no current number exists" is knowable without ever having been told to expect one, and it is the strongest form of staleness there is.

The alternative — inventing a default (`within_limit`, since 0 ≤ any non-negative limit) — would misrepresent silence as compliance. A product asking "is this organization within its limit" deserves to know the honest answer is "nothing has told us yet," not a manufactured `within_limit` that looks identical to a real, recent, compliant report.

## Decision: usage is derived fresh on every read; only the license record is cached

[Issue #62](https://github.com/nanostack-dev/anchor/issues/62) asks for two things that look like they pull in opposite directions: the read is cached with the framework cache module and evicted on license write (the same discipline as the API key permission cache, `docs/api-key-permission-cache.md`), and separately, usage arriving must not require a license write to become visible on the next read.

Both hold if only the `OrganizationLicense` row — the values a template's copy carries — is the cached value. `OrganizationLicenseService.GetLicense` reads that from cache (`cache.Cache[license.OrganizationLicense]`, keyed by product and organization, TTL matching the API key cache), evicted by `Instantiate` and `AdjustValues`. Usage — the latest observation per key and the status derived from it — is computed after the cache lookup, every call, straight from `UsageObservationRepository.LatestPerKey` and the schema's fields. It is never part of the cached value and nothing evicts it, because there is nothing to evict: a usage report is visible to the very next read regardless of the license record's cache state.

This also means a schema edit (adding, removing, or changing a field's expected reporting interval) is visible immediately, since the schema itself is not part of what is cached here either. Only the license values follow the eviction-on-write discipline; deriving status from them is deliberately as fresh as the rest of the licensing subsystem's writes already are.

## Consequences

**Good.** Both halves of the issue's requirement hold without a special case: caching gives the hot-path read its latency win on the part that changes rarely (a license's own values), and the part that changes on every report (usage) is never stale from caching, because it was never cached.

**Good.** `stale` from absence needs no schema change to become visible — instantiating a license onto a limit field immediately reads as `stale` rather than an invented `within_limit`, which is the honest state.

**Cost.** Every license read now performs one schema read and one usage query in addition to the (possibly cached) license read. Both are indexed, single-organization lookups (`license_schema_fields` by schema id, `usage_observations` by `(organization_id, key, observed_at desc)`), and neither is on the write path, but this is a real cost paid on every call, cached or not.

**Note.** A future SDK facade that caches the license read client-side (per [issue #62](https://github.com/nanostack-dev/anchor/issues/62)'s SDK section) inherits this same shape: it should default to fail-open on `stale`, exactly because `stale` already means "nothing to trust yet or the data is old," not "something is broken."
