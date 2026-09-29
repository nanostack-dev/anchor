# ADR-0019: Organization invitations live in Anchor

**Status:** Accepted

## Context

EchoPoint needs a way for an admin to bring a new person into an Organization. Anchor already owns Organization, membership and role, but had no invitation. The alternative was to build invitations inside EchoPoint: it already decides who may invite, gates the member limit, owns its email templates and knows the signed-in person's verified email. Only one Product consumes Anchor today, so "other Products will reuse it" was a guess, not a need.

## Decision

Invitations live in Anchor, because Anchor is a product in its own right: Organization-as-a-Service for SaaS products Nanostack does not own. Inviting people into an Organization is the common SaaS foundation work Anchor exists to provide, and every consumer would otherwise rebuild it.

Anchor stores the invitation, its token and its lifecycle, and exposes them as a full API so a Product can drive its own flow. Anchor stays private: the invited person never talks to Anchor. The Product performs the accept. The policy stays with the Product: who may invite, whether the member limit allows it, and whether the person accepting is the person invited ([ADR-0001](0001-anchor-validates-but-never-gates.md)).

A Product that wants to run the whole flow itself must never be blocked by a rule Anchor made. Anchor only validates data integrity: one pending invitation per email per Organization, and no invitation for an existing member. Everything else the Product can change or bypass through the API: update the role or expiry, delete an invitation outright, create a new one. The email can go out two ways, chosen per Product in its invitation settings: Anchor sends it, or the Product does. Under Anchor delivery the send is part of the create call, so a failed send fails the call and is logged. Anchor stores no delivery state.

## Considered Options

- **Invitations in EchoPoint.** Faster and one repo, but every future Product rebuilds the same thing, and accepting spans an EchoPoint write plus an Anchor call instead of one transaction.
- **Clerk invitations.** Rejected: Clerk handles authentication only.

## Consequences

Accepting an invitation and creating the membership happen in one Anchor transaction. Each change to invitations crosses two repos (Anchor contract and SDK, then EchoPoint).
