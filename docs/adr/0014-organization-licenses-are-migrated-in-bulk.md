# ADR-0014: Moving organizations onto a license template is one Anchor operation

**Status:** Accepted

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

Only Anchor can restamp provenance, and restamping provenance is what the operation *is*. Everything else follows: the selection ("everyone currently on the tier we are retiring") is a query only Anchor can answer, the field-by-field preview is the diff Anchor already computes, and the record is the change history Anchor already keeps.

## Decision

**Anchor gains one product-scoped route that moves a set of organizations onto a license template.**

```
POST /v1/products/{product_id}/licensing/organization-licenses/migrate
```

Scoped `organization_license:migrate`, a new entry in the permission catalog alongside `email:send`.

### It takes a fresh copy, and stamps it

For each organization, the license's values are replaced wholesale by the target template's values, and `template_id` and `instantiated_at` are restamped to the target and to the moment of the run. This is instantiation happening a second time, so it behaves like instantiation: the copied values are not re-validated against the schema, for the reason `Instantiate` gives — a schema tightened since the template was last written must not block a move onto a tier that is still on sale.

A new repository method, `Restamp`, is the only path that writes `template_id`. `Update` keeps its exclusions, so "an adjustment cannot change which template a customer was sold" remains true as a property of the code and not only of the caller.

**This is not a re-sync, but it subsumes one.** Naming the template an organization already holds is allowed, and restamps it from that template, discarding differences. That is the out-of-scope operation — reachable only by naming the same template *and* asking to overwrite, since anything that differs from its own template is skipped by default. It is not a background sync, it is not implicit, and no license moves because a template moved.

### Selection is server-side

The caller supplies exactly one of:

- `organization_ids` — an explicit list.
- `from_template_id` — every organization in the product whose license names that template. An archived template is accepted here, because moving customers off a withdrawn tier is the main reason to run this at all.

Resolving `from_template_id` inside the request is not a convenience. A client that lists and then loops races every organization instantiated between the two calls.

### A run is bounded, and refuses rather than truncating

At most 500 organizations per call. A selection matching more is refused with `LICENSE_MIGRATION_TOO_LARGE` carrying the count, for a dry run exactly as for a real one, so the operator learns the size before committing either way. Nothing is ever silently truncated.

500 organizations is a few seconds of small transactions, which fits a synchronous request honestly. A larger cohort is the operator's loop over explicit lists, which is a thin client loop rather than a script carrying licensing semantics.

### A difference blocks the move by default

`on_difference` is `SKIP` or `OVERWRITE`, defaulting to `SKIP`. An organization whose license differs from the template it currently names is reported `SKIPPED` with reason `DIFFERS_FROM_TEMPLATE` and left alone.

The word is *difference*, not *deviation*, and `CONTEXT.md` explains why: a difference is either someone adjusting that customer or the template moving after the copy was taken, and the diff alone does not say which. So the default cannot be described as "protects bespoke arrangements" — it protects everything that differs, including a cohort whose only sin is that their tier was edited after they were stamped. That is what the dry run is for. The operator reads the differences, decides, and re-runs with `OVERWRITE` or handles those organizations one at a time.

An organization holding no license at all is `SKIPPED` with reason `NOT_LICENSED`, whatever the policy. Instantiation is a different verb with a different scope, and the two stay distinct.

### The dry run is the same code path

`dry_run: true` computes every outcome and writes nothing. It is not a second implementation: the run resolves the selection, computes each organization's outcome and its field-by-field changes, and only then decides whether to write. A dry run and the real run therefore cannot disagree about anything except a failure that only a write can produce.

### One transaction per organization

Not one per batch. An organization that fails is reported `FAILED` with its error code and the batch continues. A batch-wide transaction would make one bad row discard several hundred good ones, and there is no invariant spanning two organizations' licenses that a shared transaction would protect.

### The audit trail is the history that already exists

Every organization moved appends one entry to `organization_license_changes` with a new `change_type` of `MIGRATED`: `template_id` is the target, `previous_template_id` the tier they came from, `old_value` and `new_value` the whole sets on either side. `change_type` carries no CHECK constraint, so the new type needs no migration; `previous_template_id` is a new nullable column, which is one.

Every entry of one run shares a single `changed_at`, exactly as the entries of one adjustment already do. A run is therefore identifiable by that timestamp together with the target template, without a batch table or a batch identifier.

## Consequences

**Good.** "Which customers are on the tier we retired, and move them" is two calls, and the first one is a dry run.

**Good.** Provenance survives a tier move, so ADR-0010's promise — the record of what a customer was sold keeps resolving — holds for a customer who has been sold more than one thing.

**Good.** The copy model is untouched. A license is still a copy, still has no pointer to a template, and still cannot be changed by editing a template. What is new is a second stamping, requested explicitly.

**Good.** The default refuses to overwrite anything that differs, so the destructive reading of this operation requires the operator to ask for it twice: once in the dry run they read, once in the `OVERWRITE` they send.

**Cost.** `SKIP` cannot tell a bespoke arrangement from a template that moved, because nothing can. A product that edits its templates in place will see large skip lists and reach for `OVERWRITE`, which is exactly the moment the dry run matters most and exactly the moment an operator is most likely to skim it.

**Cost.** Migrating discards differences rather than carrying them forward. An organization on Beta with `flows` raised to 25 as a bespoke deal, moved to Pro, ends on Pro's `flows` and nothing else. Merging was considered and rejected: with no way to tell a deviation from a moved template, a merge would preserve stale values as often as bespoke ones, and would do it silently.

**Cost.** 500 is a cap, and a product with more customers on one tier has to batch its own calls. A cursor over the selection, or an asynchronous job with a stored batch record, is additive later and neither changes the contract's shape.

**Cost.** A run is recorded only as its organizations' history entries. "Show me the migration that ran on the third" is answerable by the shared `changed_at`, which is a join the caller writes rather than a route Anchor offers. A `license_migrations` table is additive later; it was not built because the per-organization record is the auditable unit and it already existed.

**Cost.** `instantiated_at` now moves. It was documented as the date the copy was taken and an adjustment still does not move it, but a reader who took it to mean "when this customer was first licensed" is now wrong. The first stamping is still in the history, as the `INSTANTIATED` entry.
