# ADR-0008: One license schema per Product, under a `licensing/` namespace

**Status:** Accepted

## Context

[ADR-0004](0004-license-schema-template-and-copy.md) settled the three layers but not how many schemas a Product has, nor where the routes live. Both surfaced while building the first slice, and both are cheap now and breaking later — once the Go SDK and the Terraform provider ship, a route is a published contract and a second schema row is a migration with live data behind it.

The obvious alternative to a singleton was a collection with one schema marked active, which would give staged evolution and a rollback flip.

## Decision

**A Product has exactly one license schema.** `UNIQUE (product_id)` on `license_schemas`, and the routes address it by product with no schema identifier.

The active-pointer model was rejected because it does not buy what it appears to. Anchor validates on write only, so adding a required field breaks nothing at read time — no existing license is re-validated and no read starts failing; it constrains the next write and nothing else. Flipping to a new version does not backfill existing licenses either, so the same migration work remains in both models. What the pointer does add is a state machine with genuinely hard questions attached — may a schema be activated that existing templates violate, and what does the usage path do mid-flip — plus a resolution hop on a read [ADR-0005](0005-timescaledb-for-usage-history.md) wants small and cacheable.

Review and rollback already exist elsewhere. [ADR-0006](0006-license-write-surfaces.md) puts the schema in Terraform, changing "rarely, and by us"; `terraform plan` is the diff, the review gate and the revert, and an activation flag would duplicate that while forcing the provider to order activations.

The direction is deliberately the reversible one. Singleton → collection is additive: drop the unique constraint, add an active pointer, keep today's routes as "the active one". Collection → singleton is not, because the inactive rows would need a meaning.

**Routes sit under `licensing/`, not `license/`.** Licensing is a subsystem with several resources, and `email/` is the in-repo precedent for that shape — a namespace segment that is not itself addressable, holding `email/templates` and `email/sends`.

The namespace is *not* named `license`, because `CONTEXT.md` spends that noun on one Organization's own copy of a template's values. Under `license/` the segment would mean the subsystem at product level and the organization's grant one level down — one word, two concepts, which is the failure the glossary's struck-words section exists to prevent. `licensing` is free, and matches the OpenAPI tag already on these operations:

```
/products/{p}/licensing/schema                  the singleton declaration
/products/{p}/licensing/templates               named sets of values
/products/{p}/organizations/{o}/license         the Organization's grant
/products/{p}/organizations/{o}/license/usage   usage against that grant
```

## Consequences

**Good.** No lifecycle to build, document or test. A schema edit is an edit, reviewed as a Terraform diff like the permission catalog it sits beside.

**Good.** Every path segment carries one meaning, so `license/usage` reads as usage against that organization's license rather than as a second namespace.

**Cost.** No schema history. "What did this Product's schema look like in January?" is unanswerable. Buy it back the way [ADR-0004](0004-license-schema-template-and-copy.md) buys back template history — an append-only change log following the `integration_audit_logs` idiom. That is additive and needs no state machine.

**Cost.** A Product cannot validate two populations differently at the same time — legacy customers against an old field set, new ones against a new one. Per-organization *divergence* is already covered, because each license is its own copy and holds whatever it was stamped with; only differing `required` rules for new writes are out of reach. If that becomes real, it is the trigger to revisit, and the additive path above is the route.
