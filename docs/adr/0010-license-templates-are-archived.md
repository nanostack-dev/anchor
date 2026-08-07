# ADR-0010: License templates are archived, never deleted

**Status:** Accepted

Amends [ADR-0004](0004-license-schema-template-and-copy.md), which states that a template carries "no `DRAFT`/`PUBLISHED`/`ARCHIVED` lifecycle". One of those three now exists.

## Context

An Organization's license names the template it was instantiated from, in `template_id`, and the date in `instantiated_at`. Together they answer "what was this customer sold, and when" — epic #62's user story 17 — and they are what the template diff (story 18) is computed against.

Migration `000027` stored `template_id` as a plain column with no foreign key, and the reasoning was that a deleted template does not make it untrue that a customer was sold that tier on that date. That is correct about the *fact* and wrong about the *record*. Once the template row was gone:

- The identifier resolved to nothing. "Which tier is this account on" had no answer beyond an opaque KSUID.
- The diff route returned 404, so the accounts most likely to want the answer — the ones on a tier that was withdrawn — were exactly the ones that could not get it.
- Nothing in the database prevented the dangling reference, so the inconsistency was permanent and silent.

A template is small, and a Product offers few of them over its life. The cost of keeping every one is negligible next to a customer record that stops resolving.

## Decision

**A license template is withdrawn by archiving it. Its row is never deleted.**

`license_templates` gains `status`, which is `ACTIVE` or `ARCHIVED`. **The `DELETE` route is removed** rather than repurposed: a verb that says one thing and does another is worse than no verb. Withdrawal is `POST .../templates/{id}/archive`, which names the one irreversible act it performs and returns the archived template so a caller sees the new status without a second read. It is idempotent.

An archived template is refused wherever a caller would act as though the tier were still offered — instantiation and edits — and kept wherever the record is what is wanted. It still resolves by identifier, and it stays in the listing: a template is never deleted, so listing a Product's templates means all of them. An optional `status` narrows to one, which is how an operator finds the tiers they need to move customers off.

**Because the row is permanent, `organization_licenses.template_id` becomes a real foreign key**, composite with `product_id`, with no `ON DELETE` clause. Nothing has to decide what happens to the licenses that name a template, because nothing deletes one.

**The name is unique among a Product's active templates only.** A partial unique index replaces the table constraint. Archiving "Pro" frees the name, so a withdrawn tier does not block its own replacement.

This is not the lifecycle ADR-0004 ruled out. There is no draft to publish, no revisions, and no state a template moves back through. Read ADR-0004's sentence as ruling out *versioning*, which still does not exist and which the copy already makes unnecessary.

## Consequences

**Good.** Every tier a Product has ever offered stays listable and indexed, so "which customers are on the tier we retired last year" is an ordinary query rather than an archaeology exercise.

**Good.** Provenance always resolves. "Which tier is this account on, and how do they differ from it" is answerable for the whole life of the license, including after the tier is withdrawn — which is when an operator is most likely to ask.

**Good.** The database enforces it. A dangling `template_id` is now impossible rather than merely unlikely, and the diff has no missing-template branch to handle.

**Good.** Withdrawing a tier is reversible in the way that matters: the name is free, so the replacement is a new template with the old name, and every existing license keeps naming the row it was actually stamped from.

**Cost.** There is no way to remove a template created by mistake. It is archived like any other, and it stays in the unfiltered listing. Rows accumulate, bounded by the number of tiers a Product has ever offered.

**Cost.** There is no `DELETE` on a license template at all, which will read as an omission to anyone who has not found this ADR. The alternative — keeping `DELETE` and having it archive — trades that one-time surprise for a permanent lie in the contract, and a verb that quietly does not do what it says is the worse of the two.

**Cost.** `POST .../archive` is an action route in an otherwise resource-shaped API. `PUT .../status` would have been the resource-shaped option, but a status you can only ever set to one value is a worse description of an irreversible act than a verb is.

**Cost.** Archiving cannot be undone. An operator who archives the wrong tier recreates it, and the two rows are then distinguishable only by their identifiers and dates. This is deliberate — an un-archive would let a tier's history be rewritten, and the record is the whole reason the row is kept.

**Note.** Migration `000028` adds the foreign key over existing data. It fails if any license names a template that was hard-deleted while `000027` was live. Failing is correct: the alternative is discarding a customer's record of what they were sold.
