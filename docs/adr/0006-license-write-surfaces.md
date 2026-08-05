# ADR-0006: Schemas and templates in Terraform, licenses via API

**Status:** Accepted

## Context

The three licensing layers change at very different rates. A license schema changes when the shape of what can be licensed changes — rarely, and by us. A template changes when pricing changes — occasionally, and by us. An organization's license changes whenever a customer signs up, upgrades, or gets a bespoke adjustment — constantly.

Anchor already draws this line for RBAC: `anchor_product_permission` and `anchor_product_role` are Terraform resources in `infra/terraform/deploy/anchor/{dev,prod}/main.tf`, while role *assignments* are runtime API calls. The permission catalog is reviewed infrastructure; assignments are data.

## Decision

| layer | write surface |
| --- | --- |
| license schema | Terraform (`anchor_license_schema`) **and** the admin UI |
| license template | Terraform (`anchor_license_template`) **and** the admin UI |
| organization license | API and SDK only |

Schemas and templates are declarative configuration in the same category as the permission catalog: a change to "Pro gets 500 flows" is a pricing change and deserves a reviewed diff and a dev→prod promotion.

They are **also editable in the admin UI**. Both writers go through the same API and therefore the same validation.

**No ownership marker.** There is no `managed_by` field and no read-only mode in the UI for Terraform-managed records. Conflicts are resolved the way every Terraform user already resolves them: `terraform plan` reports drift, and the operator decides. This is how AWS and every other mature provider behaves, and inventing a per-record ownership flag would mean a new field to maintain, an immutability rule to police, and a mental model nobody else uses.

**Terraform must not own the organization license.** Per-customer deviation is runtime data ([ADR-0004](0004-license-schema-template-and-copy.md)), so a Terraform-managed license would revert every bespoke adjustment on the next apply.

## Consequences

**Good.** Pricing changes land as reviewed pull requests with visible diffs, and promote through the existing dev→prod pipeline.

**Good.** A future billing integration writes organization licenses over the API and never touches schemas or templates — it has no business inventing plans.

**Cost.** Drift is real and possible. A template edited in the UI will show as a change on the next `terraform plan`, and an operator who applies without reading will revert it. This is accepted as the operator's responsibility, consistent with how Terraform is used everywhere else.

**Cost.** A schema or template created in the UI is not in the repository. Someone creating a one-off enterprise template should understand it lives only in the database until someone imports it.
