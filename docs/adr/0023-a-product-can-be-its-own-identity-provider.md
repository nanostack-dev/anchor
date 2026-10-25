# ADR-0023: A product can be its own identity provider

**Status:** Accepted, 2026-10-25

## Context

Access evaluation names a user by its external ID: the product user's ID in its identity provider ([access evaluation](../technical/access-evaluation.md)). Until this decision only Clerk ingest wrote `product_users.external_id`. `POST /v1/products/{product_id}/product-users` had no external ID, so a user that a product created through the API could not be named by access evaluation at all.

Some products authenticate their users themselves. Atelier signs users in with an email and a password and has no Clerk instance. To use access evaluation it must give each product user its own account ID.

The column was ready: `VARCHAR(255)`, nullable, with the partial unique index `idx_product_users_external_lookup` on `(product_id, external_id)` (migration 000005). It compares IDs exactly, with letter case.

## Decision

A product user can be created with `external_id`, the user's ID in the product's own identity provider.

- **Create only.** The ID is set on create and no operation changes it. Anchor exposes no update for a product user, and this decision keeps it that way for the external ID. Access decisions follow the ID, so a changed ID would give the access of one person to another, with no record of the change. To correct an ID, the product deletes the user and creates it again, which also removes its memberships.
- **Unique within the product, compared exactly.** The same rule as the column and the Clerk IDs already stored in it. A duplicate answers 409 `PRODUCT_USER_EXTERNAL_ID_ALREADY_EXISTS`, the same shape as `PRODUCT_USER_EMAIL_ALREADY_EXISTS`. The create maps a violation of `idx_product_users_external_lookup` to the same 409, so concurrent creates do not answer 500.
- **One set of IDs per product, shared with Clerk.** A Clerk integration of the same product matches its users on the same column. A user created with a Clerk user ID is the user that Clerk ingest then updates (email, name, active status) and deletes. Clerk ingest never writes a different external ID on an existing user, so it cannot point an ID at another person.
- **Optional and additive.** Released clients (EchoPoint, the CLI, the Terraform provider) do not send the field and behave as before. A user created without it has no external ID until Clerk syncs one.

## Considered Options

- **Let an update set the external ID.** Rejected. It needs a new update route, and every write to the ID silently changes who gets which access decisions.
- **Set it once, on a separate "link identity" route.** It would allow linking after create, but a product that is its own identity provider knows the ID before it creates the user. One call is enough; a second route adds a state (created, not yet linked) that nothing needs.
- **An identity table with a provider column**, so that Clerk IDs and a product's IDs cannot collide. Rejected for now. A collision needs a product to send an ID that its own Clerk instance also issued, and that is the case where sharing is the wanted behaviour: the product creates the user before Clerk sends it. The table can be added later without a change to the contract.
- **Compare IDs without letter case**, as product and role names are compared ([case-insensitive identifiers](../case-insensitive-identifiers.md)). Rejected. Identity providers issue case-sensitive IDs, Clerk IDs are already stored that way, and the index is case-sensitive.

## Consequences

- Access evaluation works for a product that is its own identity provider, with no change to the evaluator or its contract.
- The SDK has `Users().Create(email).ExternalID(id)`. The Terraform provider has no product user resource; one added later must replace the user when the ID changes.
- A Clerk user whose email belongs to another product user still cannot be ingested: the insert fails on `product_users_product_id_email_key`, the webhook event retries and then fails, and the reconcile batch rolls back. This was already true for a user created without an external ID. Creating the user with its Clerk user ID avoids it.
- A product that sends a Clerk user ID for a different person gives that person's Clerk updates and deletion to the user it created. Anchor cannot detect this; the product owns the IDs it sends.
