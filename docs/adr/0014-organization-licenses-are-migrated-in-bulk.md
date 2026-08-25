# ADR-0014: Moving organizations onto a license template is one Anchor operation

**Status:** Accepted, partially superseded by [ADR-0015](0015-migrate-grants-a-first-license.md), amended by [ADR-0017](0017-license-follows-its-template.md)

[ADR-0015](0015-migrate-grants-a-first-license.md) reverses the "an organization holding no license is `SKIPPED` with reason `NOT_LICENSED`" clause in this ADR's Decision, and the corresponding `MIGRATED`/`SKIPPED` naming in Consequences. Everything else below — restamping provenance, `CARRY_FORWARD`/`DISCARD`, the 500 cap, one transaction per organization, the search route — is unchanged and still governs.

[ADR-0017](0017-license-follows-its-template.md) adds what a migration does to the license's adjusted-field record (`DISCARD` clears it, `CARRY_FORWARD` keeps the fields the target declares), and shrinks the "carries stale values it cannot tell from bespoke ones" cost below: a license now follows its template, so outside a propagation still in flight, an un-adjusted license does not differ from its own tier.

Extends [ADR-0004](0004-license-schema-template-and-copy.md), which says an organization's license is a copy taken at instantiation. It stays a copy. This adds the operation that takes a *second* copy, from a different template, and says so in the record.

## Context

Epic #62 lists "re-syncing an organization's license from its template" under Out of Scope. This is not that operation, and the distinction is the whole reason this ADR exists.

Re-syncing means: recompute a license from the template it *already names*, discarding whatever was adjusted. It answers "make this customer match their tier again". Nobody has asked for it.

Moving a group of organizations onto a *different* template is a commercial event that happens on an ordinary week: a tier is withdrawn and its customers go somewhere, an early-access cohort graduates onto the paid tier, a pricing experiment ends. [ADR-0010](0010-license-templates-are-archived.md) already assumes this operation exists — it argues the archived-template listing is "how an operator finds the tiers they need to move customers off" — and then there is no route that moves them.

### The consumer-side loop does not work

The obvious answer is to leave Anchor alone and loop `PATCH .../license` over the organizations, once per field. It fails on four counts, and the first is fatal.

**Provenance would lie.** `OrganizationLicenseRepository.Update` deliberately excludes `template_id` and `instantiated_at` from its updatable columns, so an adjustment cannot change which template a customer was sold. After a loop of adjustments moving every field to Pro's values, the license still reads "instantiated from Beta, on the day they signed up". ADR-0010 keeps a template row forever precisely so that answer stays true. A loop of adjustments makes it false.

**The diff would become noise.** User story 18 wants the diff to find accounts with bespoke arrangements. An organization adjusted field-by-field onto Pro's values diffs against Beta on every field, so the one route for finding bespoke accounts reports the whole migrated cohort.

**The history would misdescribe what happened.** User story 20 wants a support engineer to explain what changed. A tier move would read back as a handful of unrelated field adjustments that happen to share a timestamp.

**The archived-template guard would not fire.** `Instantiate` refuses an archived template because a withdrawn tier cannot be sold to anyone new. `AdjustValues` holds no template repository at all — by design, so an adjustment cannot drift from what a template write is held to — so a loop of adjustments can put a customer on the values of a withdrawn tier with nothing to stop it.

Three of those are inconveniences. The first is a correctness claim ADR-0010 makes in writing.

### So the capability belongs to Anchor

Only Anchor can restamp provenance, and restamping provenance is what the operation *is*. Everything else follows: the selection ("everyone currently on the tier we are retiring") is a query only Anchor can answer, and the record is the change history Anchor already keeps.

## Decision

**Anchor gains one product-scoped route that moves a set of organizations onto a license template.**

```
POST /v1/products/{product_id}/licensing/organization-licenses/migrate
```

Scoped `organization_license:migrate`, a new entry in the permission catalog alongside `email:send`.

### It takes a fresh copy, and stamps it

For each organization, the license takes the target template's values, and `template_id` and `instantiated_at` are restamped to the target and to the moment of the run. This is instantiation happening a second time, so it behaves like instantiation: the copied values are not re-validated against the schema, for the reason `Instantiate` gives — a schema tightened since the template was last written must not block a move onto a tier that is still on sale.

A new repository method, `Restamp`, is the only path that writes `template_id`. `Update` keeps its exclusions, so "an adjustment cannot change which template a customer was sold" remains true as a property of the code and not only of the caller.

**This is not a re-sync, but it subsumes one.** Naming the template an organization already holds is allowed. With `on_difference: DISCARD` it resets that organization to its own tier — the out-of-scope operation, reachable only by naming the same template *and* asking to discard. It is not a background sync, it is not implicit, and no license moves because a template moved.

### Selection is server-side, and the book is searchable

`POST /v1/products/{product_id}/licensing/organization-licenses/search` reads the customer book a page at a time: every organization and the license it holds, filtered by `license_template_ids`, by `licensed`, or by a match on the organization name. An organization holding no license is a row with no license rather than a missing row, because "who is on no tier" is half of what the question is.

That search is what the admin UI lists and what an operator selects from, and its template filter is the reason `organization_licenses` is left-joined onto `organizations` rather than read separately. Usage is not derived per row: a page would otherwise cost as many usage derivations as it has rows.

For the run itself the caller supplies exactly one of:

- `organization_ids` — an explicit list.
- `from_template_id` — every organization in the product whose license names that template. An archived template is accepted here, because moving customers off a withdrawn tier is the main reason to run this at all.

Resolving `from_template_id` inside the request is not a convenience. A client that lists and then loops races every organization instantiated between the two calls.

### A run is bounded, and refuses rather than truncating

At most 500 organizations per call. A selection matching more is refused with `LICENSE_MIGRATION_TOO_LARGE` carrying the count. Nothing is ever silently truncated.

500 organizations is a few seconds of small transactions, which fits a synchronous request honestly. A larger cohort is the operator's loop over explicit lists, which is a thin client loop rather than a script carrying licensing semantics.

### A difference is carried forward by default

`on_difference` is `CARRY_FORWARD` or `DISCARD`, defaulting to `CARRY_FORWARD`. A license field whose value differs from the template the organization currently holds keeps that value on the migrated license; every other field takes the target's. `DISCARD` takes the target whole.

The default is the one that survives contact with a real customer book. An enterprise account given +50 flows and then moved from Pro to Enterprise should not silently lose the +50 because the tier changed — the deal outlived the tier, which is the whole reason a per-organization deviation exists at all.

The word is *difference*, not *deviation*, and `CONTEXT.md` explains why: a difference is either someone adjusting that customer or the template moving after the copy was taken, and the difference alone does not say which. So the default cannot be described as "preserves bespoke arrangements" — it preserves everything that differs, including a value that is merely stale because the tier was edited after the copy was stamped. **This is the real cost of the default, and it is silent.** What an operator has instead is the comparison between the two templates, which the admin UI renders before the run, and the per-organization `changes` the run reports afterwards.

Carrying forward is bounded by the target's own declaration: only a license field the target template names can keep its value. A value the target does not name belongs to a field the schema no longer declares, and carrying it would resurrect a field nothing validates.

An organization holding no license at all is `SKIPPED` with reason `NOT_LICENSED`. Instantiation is a different verb with a different scope, and the two stay distinct.

### One transaction per organization

Not one per batch. An organization that fails is reported `FAILED` with its error code and the batch continues. A batch-wide transaction would make one bad row discard several hundred good ones, and there is no invariant spanning two organizations' licenses that a shared transaction would protect.

### The audit trail is the history that already exists

Every organization moved appends one entry to `organization_license_changes` with a new `change_type` of `MIGRATED`: `template_id` is the target, `previous_template_id` the tier they came from, `old_value` and `new_value` the whole sets on either side. `change_type` carries no CHECK constraint, so the new type needs no migration; `previous_template_id` is a new nullable column, which is one.

Every entry of one run shares a single `changed_at`, exactly as the entries of one adjustment already do. A run is therefore identifiable by that timestamp together with the target template, without a batch table or a batch identifier.

## Consequences

**Good.** "Which customers are on the tier we retired, and move them" is two calls: search by that template, then migrate the selection.

**Good.** Provenance survives a tier move, so ADR-0010's promise — the record of what a customer was sold keeps resolving — holds for a customer who has been sold more than one thing.

**Good.** The copy model is untouched. A license is still a copy, still has no pointer to a template, and still cannot be changed by editing a template. What is new is a second stamping, requested explicitly.

**Good.** The destructive reading of this operation — taking the tier whole and dropping what a customer was individually given — requires asking for it by name, in `on_difference: DISCARD`.

**Cost, and the sharpest one.** `CARRY_FORWARD` cannot tell a bespoke arrangement from a template that moved, because nothing can. A product that edits its templates in place will carry stale values onto the new tier and say nothing about it, and the organizations affected are exactly the ones nobody deliberately touched. The mitigation is external to Anchor: manage templates as reviewed configuration (which [ADR-0006](0006-license-write-surfaces.md) already prescribes) so that in-place edits are rare, and read the template comparison before running.

**Cost.** There is no server-side preview. An earlier draft of this ADR specified a `dry_run` that computed every outcome and wrote nothing; it was removed as unearned complexity, because the question an operator actually asks before a tier move — "how do these two tiers differ" — is answered by the two templates, which any client already has. What a dry run would have added over that is the per-organization carry-forward resolution, and that is recoverable from the run's own `changes`. A migration that goes wrong is corrected by another migration, not by an undo.

**Cost.** 500 is a cap, and a product with more customers on one tier has to batch its own calls. A cursor over the selection, or an asynchronous job with a stored batch record, is additive later and neither changes the contract's shape.

**Cost.** A run is recorded only as its organizations' history entries. "Show me the migration that ran on the third" is answerable by the shared `changed_at`, which is a join the caller writes rather than a route Anchor offers. A `license_migrations` table is additive later; it was not built because the per-organization record is the auditable unit and it already existed.

**Cost.** `instantiated_at` now moves. It was documented as the date the copy was taken and an adjustment still does not move it, but a reader who took it to mean "when this customer was first licensed" is now wrong. The first stamping is still in the history, as the `INSTANTIATED` entry.
