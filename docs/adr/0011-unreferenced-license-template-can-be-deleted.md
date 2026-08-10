# ADR-0011: An unreferenced license template can be deleted

**Status:** Accepted

Amends [ADR-0010](0010-license-templates-are-archived.md), which removed `DELETE` from the license template route entirely, on the grounds that a template's row must outlive it being offered because an Organization's license names it.

## Context

ADR-0010 reasoned correctly about a template that has customers: deleting it would strand every license that names it, so withdrawal has to keep the row. But it reasoned about that case only, and then removed `DELETE` unconditionally. The cost it named explicitly:

> There is no way to remove a template created by mistake. It is archived like any other, and it stays in the unfiltered listing. Rows accumulate, bounded by the number of tiers a Product has ever offered.

That cost falls hardest on exactly the workflow this repository's own Terraform provider adds: a template created and destroyed inside a CI acceptance test, or a tier drafted, never sold, and abandoned. None of those templates is named by any Organization's license — the foreign key ADR-0010 introduced guarantees that precisely — so nothing is lost by removing the row, and nothing about ADR-0010's reasoning applies to them.

## Decision

**`DELETE /v1/products/{product_id}/licensing/templates/{license_template_id}` is reintroduced.** It removes the row outright, and is refused with `400 LICENSE_TEMPLATE_IN_USE` if any Organization license still names the template — checked before the write, the same way a template name conflict is checked before its unique index would otherwise decide it (`licenseTemplateService.CreateTemplate`). The check and the write are not atomic against a concurrent instantiation; the foreign key `fk_organization_licenses_template` (migration `000028`) is what actually guarantees the invariant, and a race that slips past the check fails at the constraint instead, mapped to the same error.

Archiving is unchanged and remains the only withdrawal for a template that might have customers. A template that could be referenced is in practice never deletable, because the foreign key forbids it — delete is only ever reachable for a template nobody was ever licensed from. **The admin UI keeps using archive**: an operator withdrawing a tier from the UI cannot generally know whether some Organization already holds it, so archive is the one verb that is always safe to offer there. The API and Terraform caller that just created a template it is about to tear down in the same test, or the same `terraform destroy`, is in a different position, and delete exists for that caller.

**Archive's scope moves from `license_template:delete` to `license_template:update`.** Before this ADR, `license_template:delete` meant "call archive" and nothing stronger existed under that name, so sharing it cost nothing. Now it means "permanently remove the row," and a key an operator granted expecting only the reversible-in-listing, row-preserving act of archiving would, unchanged, have gained the ability to destroy a row outright — a capability expansion nobody consented to. Archive is an edit to the row's `status` column, not a removal of it, which is what `license_template:update` already means for every other field on a template.

Status is not part of the guard. An `ARCHIVED`-but-unreferenced template can also be deleted: archiving a template by mistake is exactly the case ADR-0010 named as unrecoverable, and the reference check is sufficient on its own to keep the provenance guarantee — nothing about being archived makes a template more or less safe to remove.

## Consequences

**Good.** A template no Organization was ever licensed from — the common case for a tier drafted, tested, and torn down, or an acceptance test's own fixture — is actually removed rather than accumulating as a permanently archived row.

**Good.** ADR-0010's named cost, "there is no way to remove a template created by mistake," now has an answer for the one case it is safe to answer: nobody has been sold it.

**Good.** The guarantee ADR-0010 built — a license's `template_id` always resolves — is unweakened. Delete cannot orphan a license: the same foreign key that made `template_id` a real reference is what the delete's own write would fail against if the check above raced and lost.

**Cost.** Two ways to remove a template now exist, and a caller has to know which applies. `LICENSE_TEMPLATE_IN_USE` on a delete attempt is the signal to archive instead, and the error message says so.

**Good.** A key scoped to `license_template:update` — the ordinary "let this integration manage its own templates" grant — can withdraw a tier by archiving it without also being trusted to permanently destroy a row. `license_template:delete` now means only the one thing its name says.

**Cost.** A key that already held `license_template:delete` before this ADR, expecting it to mean "call archive," silently loses that ability the moment this ships, and needs `license_template:update` added to keep working. This is a one-time break in an API surface nothing external consumes yet — the Terraform provider and the admin UI are both unbuilt or unreleased against it — so it is taken now rather than after either exists.
