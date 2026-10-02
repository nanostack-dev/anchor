# ADR-0019: Organization invitations live in Anchor

**Status:** Accepted, amended 2026-10-01: the invitation token is removed

The first version gave each invitation a secret token, returned once on create and resend, and accepted by token. It is removed: a Product finds the invitations of a signed-in person by searching for their verified email addresses, and accepts one by its id. See "Finding an invitation" below.

## Context

EchoPoint needs a way for an admin to bring a new person into an Organization. Anchor already owns Organization, membership and role, but had no invitation. The alternative was to build invitations inside EchoPoint: it already decides who may invite, gates the member limit, owns its email templates and knows the signed-in person's verified email. Only one Product consumes Anchor today, so "other Products will reuse it" was a guess, not a need.

## Decision

Invitations live in Anchor, because Anchor is a product in its own right: Organization-as-a-Service for SaaS products Nanostack does not own. Inviting people into an Organization is the common SaaS foundation work Anchor exists to provide, and every consumer would otherwise rebuild it.

Anchor stores the invitation and its lifecycle, and exposes them as a full API so a Product can drive its own flow. Anchor stays private: the invited person never talks to Anchor. The Product performs the accept. The policy stays with the Product: who may invite, whether the member limit allows it, and whether the person accepting is the person invited ([ADR-0001](0001-anchor-validates-but-never-gates.md)).

A Product that wants to run the whole flow itself must never be blocked by a rule Anchor made. Anchor only validates data integrity: one pending invitation per email per Organization, and no invitation for an existing member. Everything else the Product can change or bypass through the API: update the role or expiry (which is also how an invitation gets more time), delete an invitation outright, create a new one.

Anchor never sends the invitation email. The Product sends its own email (through Anchor's email sending if it wants), with a plain link to its app that carries no secret. There are no per-Product invitation settings: the default expiry is 7 days, and a create call can set another.

### Finding an invitation

There is no token. When a person signs up or signs in, the Product searches the invitations of every Organization of the Product for that person's **verified** email addresses (`POST /v1/products/{id}/invitations/search` with `emails` and the `pending` status), shows them, and accepts the one the person picks by its id (`POST .../organizations/{org}/invitations/{id}/accept`). The Product re-checks on accept that the invitation email is one of the person's verified addresses.

The token proved only that someone had read the email. A verified email address proves that the person controls the mailbox, which is the same thing, and the Product already had to check it so that a forwarded link could not be accepted by someone else. With that check in place the token added no protection, and it cost a lot: a secret to rotate on every resend, a secret that the Product must never store and must handle with care while it sends the email, and a lookup endpoint. Without it an invitation also reaches a person who lost the email: they sign in and see it.

## Considered Options

- **Invitations in EchoPoint.** Faster and one repo, but every future Product rebuilds the same thing, and accepting spans an EchoPoint write plus an Anchor call instead of one transaction.
- **Clerk invitations.** Rejected: Clerk handles authentication only.
- **A token in the email link.** Shipped first, then removed (see the amendment above): it duplicated the verified-email check the Product must make anyway.
- **Anchor sends the email (Anchor delivery).** Built and then removed before merge: per-Product settings chose Anchor or Product delivery, with a template and an accept URL template. Rejected because it made Anchor know each Product's URLs and email wording, added settings and a failure mode (a failed send failing the create), and gave the Product less control over the one message a new user reads first. A Product that wants Anchor's SMTP can still call the email API itself.

## Consequences

Accepting an invitation and creating the membership happen in one Anchor transaction. Anchor compares email addresses without regard to letter case, and leaves the question of which addresses are verified to the Product, which owns the identity provider. A Product that cannot verify email addresses cannot use invitations safely. A role stays in use while a member or a pending invitation names it, so deleting it is refused; accepted and expired invitations that name it are deleted with their events. Each change to invitations crosses two repos (Anchor contract and SDK, then EchoPoint).
