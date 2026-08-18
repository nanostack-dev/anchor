# ADR-0014: An organization can be created licensed, on the organization scope

**Status:** Accepted

## Context

Onboarding a customer is two writes: create the organization, then instantiate its license. A consuming product made both calls in sequence. Between them there is an organization holding no license, and if the second call fails there is an organization that stays that way. Echopoint's `/init` carried exactly this pair, and its rollback story was "the organization is already there, try the license again".

Anchor already collapses a second onboarding write into the create call: `founding_member` creates the organization and its first membership in one transaction, for the same reason.

## Decision

`POST /v1/products/{product_id}/organizations` takes an optional `license`, holding the same `template_id` the license route takes. When present, the template is copied onto the new organization in the transaction that created it. A refused template — unknown, archived, or another product's — fails the whole call and leaves no organization behind.

The 201 response carries the license it stamped, under `license`. A read of an organization never carries one: the license route is where an organization's license is read, usage and all.

**The `organization:create` scope covers it.** The license route demands `organization_license:create`, and the create route keeps demanding only `organization:create`. Scopes are enforced per operation from the contract, so demanding both would refuse every existing key that lacks the license scope, including on calls asking for no license. Creating an organization is already a product-administrative act, and the license it can stamp is a copy of a template the same product authored.

**Nothing is stamped on the idempotent path.** `founding_member` returns the organization the user already belongs to rather than creating one. That call creates nothing, so it licenses nothing, and answers no license.

## Consequences

**Good.** A product onboards a customer in one call, and a failure leaves no half-built organization.

**Good.** The license service gained `InstantiateInTx`, the same work joined to the caller's transaction. The transaction runner starts a new transaction rather than joining an ambient one, so a service composing another service's write has to say so.

**Cost.** A key holding `organization:create` alone can now stamp a license. Separating the two would need a per-field scope check the middleware does not express today.

**Cost.** `ProductOrganizationResponse` carries a field that only a create call ever fills. The alternative, a second response schema, duplicates every organization field and drifts.
