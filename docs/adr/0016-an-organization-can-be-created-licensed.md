# ADR-0016: An organization can be created licensed, on the organization scope

**Status:** Accepted

## Context

Onboarding a customer is two writes: create the organization, then instantiate its license. A consuming product made both calls in sequence. Between them there is an organization holding no license, and if the second call fails there is an organization that stays that way. Echopoint's `/init` carried exactly this pair, and its rollback story was "the organization is already there, try the license again".

Anchor already collapses a second onboarding write into the create call: `founding_member` creates the organization and its first membership in one transaction, for the same reason.

## Decision

`POST /v1/products/{product_id}/organizations` takes an optional `license`, holding the same `template_id` the license route takes. When present, the template is read and then copied onto the new organization, both in the transaction that created it. A refused template fails the whole call and leaves no organization behind.

**A template this product does not have is a 400, not a 404.** The template is resolved before the organization row is written, and its absence answers `ORGANIZATION_LICENSE_TEMPLATE_NOT_FOUND` at 400. The caller addressed the organization collection, which exists, and named the template in the body — a 404 would say the collection was missing. The license route keeps answering 404 for the same template, because there the template is what the request addresses. A template that exists but belongs to another product is the same answer, so the create route never says whether the identifier is real.

Whether the tier is still offered stays the license service's answer: an archived template is refused as `LICENSE_TEMPLATE_ARCHIVED` at 400, by the create route and the license route alike.

The organization itself carries the license, in an optional `license` field on `ProductOrganizationResponse` and on the domain object behind it. The 201 response fills it in. A read fills it in when the caller asks with `?include=license`, following the `include` convention the user-organizations routes already use — a comma-separated array of an enum, `style: form`, `explode: false`.

**Absent is not "has none".** A read that did not ask leaves the field out. Only a read that asked and came back without one says the organization holds no license.

**The include carries no usage.** Usage is derived on every read of the license route and never stored. An organization read is not that route, so the included license is the record alone.

**One statement per included resource, never one per organization.** A search including the license reads every license in the page with a single statement keyed by organization ID.

**The `organization:create` scope covers it.** The license route demands `organization_license:create`, and the create route keeps demanding only `organization:create`. Scopes are enforced per operation from the contract, so demanding both would refuse every existing key that lacks the license scope, including on calls asking for no license. Creating an organization is already a product-administrative act, and the license it can stamp is a copy of a template the same product authored.

**Nothing is stamped on the idempotent path.** `founding_member` returns the organization the user already belongs to rather than creating one. That call creates nothing, so it licenses nothing, and answers no license.

## Consequences

**Good.** A product onboards a customer in one call, and a failure leaves no half-built organization.

**Good.** Licensing writes join the transaction the context already carries, and begin one otherwise, through one `inTx` helper the subsystem shares. `Instantiate` stays a single method: a caller composing it into a larger unit calls the same thing as a caller that is not. The framework transactor always begins its own transaction, which would have put the Organization row out of reach of the license insert.

**Cost.** The template is read twice per licensed create: once to resolve it, once inside the instantiation. Handing the resolved template to the license service would remove the second read and widen its interface for one caller.

**Cost.** A key holding `organization:create` alone can now stamp a license. Separating the two would need a per-field scope check the middleware does not express today.

**Cost.** `ProductOrganizationResponse` carries a field that only a create call ever fills. The alternative, a second response schema, duplicates every organization field and drifts.
