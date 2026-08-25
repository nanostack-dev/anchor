# ADR-0017: A license follows its template, except on adjusted fields

**Status:** Accepted

Reverses the copy-not-pointer decision of [ADR-0004](0004-license-schema-template-and-copy.md): an organization's license is still stored as its own copy of a template's values, but the copy now **follows** the template. Amends [ADR-0014](0014-organization-licenses-are-migrated-in-bulk.md) (what a migration does to the new adjusted-field record, and what `CARRY_FORWARD` carries in practice) and touches [ADR-0009](0009-every-license-field-is-mandatory.md) (a schema field removal now cascades instead of leaving stale keys behind).

## Context

ADR-0004 chose copy semantics so that "editing a template cannot break a live organization". What shipped proved the opposite failure mode is the one that actually happens: editing a template silently **strands** live organizations. The echopoint organization's license sat on 6 of its 7 fields for weeks while its template carried all 7, because a Terraform edit to the template never reached it. Nothing was broken loudly; the consumer's own `Default()` filled the missing field, so the drift was invisible on both sides.

The only propagation path was the bulk migrate route (ADR-0014) — an operation a human must run, capped at 500 organizations, designed for tier *moves* rather than for keeping a tier's customers on the tier's current values. "Everyone on Pro gets what Pro says today" required someone to notice the drift and run a migration naming Pro against Pro with `DISCARD` — which also destroys every bespoke adjustment, because nothing on the license row recorded which fields were bespoke. That provenance existed only in the append-only history.

## Decision

**A license follows the template it names.** A template value update is written through to every organization license instantiated from that template. **A field the organization adjusted for itself is never overwritten** — its bespoke value survives every later template update. **A schema change that changes template values cascades the same way**: schema → template → organization license.

### Adjusted fields are recorded on the license row

`organization_licenses.adjusted_fields` (JSONB array of license field names, migration 000035) records which fields are bespoke, explicitly, rather than replaying history at propagation time.

- **Instantiate** sets it empty: a fresh copy follows its template on every field.
- **Adjust** adds every field the adjustment actually moved — the same fields that get `ADJUSTED` history entries. A field written back at its current value moves nothing and pins nothing.
- **Migrate with `DISCARD`** clears it: the operator asked for the target whole, so nothing stays pinned. This keeps the ADR-0014 re-sync reading intact — naming the tier an organization already holds with `DISCARD` resets it to that tier *and* un-pins it.
- **Migrate with `CARRY_FORWARD`** keeps it, minus the fields the target template does not declare, matching what the migration carries in values.

There is deliberately no un-pin verb short of a `DISCARD` migrate. An operator who adjusts a field back to the template's current value has still adjusted it; guessing that they meant "release it back to the template" would turn a coincidence of values into a semantic change.

### Propagation is durable, bounded, asynchronous, idempotent

A template value update enqueues one `license_template_sync` job on `pgkit/pgqueue`, **in the same transaction** as the template write — an edit that commits is an edit every license will follow, restart or not. A product can hold far more than the migrate route's 500-organization cap, so the request transaction never re-stamps licenses inline.

The worker reads the template **at processing time** (rapid edits coalesce onto the final state), pages through the organizations naming it (100 per job execution, each organization in its own transaction, a continuation job re-enqueued with a cursor), and for each license:

- resolves the synced values — the template whole, except adjusted fields keep the held value; an adjusted field the template no longer declares is dropped with the field, exactly as `MigratedValues` refuses to resurrect undeclared fields;
- skips the write entirely when nothing changes — no update, no history entry — which is what makes a re-delivered or re-run job a no-op, matching `OutcomeUnchanged`;
- validates the merged set via `ValidateValues`, exactly as an adjustment is validated. **A license whose merged values no longer satisfy the schema is refused whole and reported** (warn log naming the organization); it is never partially applied and its adjustment is never silently dropped. It keeps what it holds until an operator resolves it — the diff route shows the gap;
- appends a `TEMPLATE_SYNCED` history entry (new `LicenseChangeType`) carrying the whole set on both sides, so a reader can tell an automatic follow from an operator's `SET`;
- evicts that organization's license cache entry after the write commits.

Status and usage need nothing: ADR-0012 derives them on read, so a propagated limit recomputes on the next read for free.

### The schema cascade

A schema update that **removes** a field prunes that key from every template of the product, in the schema update's own transaction, and enqueues a sync for each template actually changed. That is the schema → template leg; the template → license leg is the same job as above. A schema update that adds a field or changes rules changes no template value and cascades nothing — ADR-0009's two-step widening (declare the field, then set it on each template) is unchanged, and the template edit that sets the new value is what propagates it.

## Consequences

**Good.** The drift class this reverses is gone: "everyone on Beta holds what Beta says" becomes an invariant maintained by the system rather than an operator's chore. The proven failure — a Terraform template edit never reaching a live customer — cannot recur silently.

**Good.** Bespoke deals finally have first-class provenance. `adjusted_fields` is readable on the license, the diff route's `changed` entries now usually *mean* "adjusted", and `CARRY_FORWARD`'s sharpest cost in ADR-0014 — carrying stale values it cannot tell from bespoke ones — shrinks to the propagation-in-flight window, because a synced license no longer differs from its own tier except where adjusted.

**Lost, and stated plainly:** the guarantee ADR-0004 bought — **a live customer can no longer be insulated from an operator's template edit**. Editing Pro now changes what every un-adjusted Pro customer holds, within seconds, with no per-customer confirmation. A fat-fingered template edit propagates as readily as an intended one; the mitigation is the same one ADR-0014 prescribed — templates managed as reviewed configuration (ADR-0006) — plus the `TEMPLATE_SYNCED` history entries, which make the blast radius auditable per organization and correctable by a second edit, which propagates the same way.

**Cost.** A refused sync (adjusted value violating a tightened schema) leaves that organization behind silently from the API's point of view: a warn log and a lingering diff, not a stored failure record. A stored per-organization sync-failure surface is additive later.

**Cost.** Propagation is eventually consistent. A read between the template write and the worker's pass returns the old values; the component tests poll for convergence, and consumers must not assume a template edit is visible synchronously.

**Cost.** `ADJUSTED` history entries written before migration 000035 did not populate `adjusted_fields`; a license adjusted before this ships follows its template on those fields unless re-adjusted. The one known production deviation of this kind was audited (the echopoint organization holds no bespoke values), so no backfill is run.
