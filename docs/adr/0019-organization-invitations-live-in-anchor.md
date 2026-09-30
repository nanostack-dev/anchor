# ADR-0019: Organization invitations live in Anchor

**Status:** Accepted

## Context

EchoPoint needs a way for an admin to bring a new person into an Organization. Anchor already owns Organization, membership and role, but had no invitation. The alternative was to build invitations inside EchoPoint: it already decides who may invite, gates the member limit, owns its email templates and knows the signed-in person's verified email. Only one Product consumes Anchor today, so "other Products will reuse it" was a guess, not a need.

## Decision

Invitations live in Anchor, because Anchor is a product in its own right: Organization-as-a-Service for SaaS products Nanostack does not own. Inviting people into an Organization is the common SaaS foundation work Anchor exists to provide, and every consumer would otherwise rebuild it.

Anchor stores the invitation, its token and its lifecycle, and exposes them as a full API so a Product can drive its own flow. Anchor stays private: the invited person never talks to Anchor. The Product performs the accept. The policy stays with the Product: who may invite, whether the member limit allows it, and whether the person accepting is the person invited ([ADR-0001](0001-anchor-validates-but-never-gates.md)).

A Product that wants to run the whole flow itself must never be blocked by a rule Anchor made. Anchor only validates data integrity: one pending invitation per email per Organization, and no invitation for an existing member. Everything else the Product can change or bypass through the API: update the role or expiry, delete an invitation outright, create a new one.

Anchor never sends the invitation email. Create and resend return the invitation token once (Anchor stores only its SHA-256 hash, and the token only travels in request bodies, never in a URL), and the Product builds its own link and sends its own email (through Anchor's email sending if it wants). There are no per-Product invitation settings: the default expiry is 7 days, and a create call can set another.

## Considered Options

- **Invitations in EchoPoint.** Faster and one repo, but every future Product rebuilds the same thing, and accepting spans an EchoPoint write plus an Anchor call instead of one transaction.
- **Clerk invitations.** Rejected: Clerk handles authentication only.
- **Anchor sends the email (Anchor delivery).** Built and then removed before merge: per-Product settings chose Anchor or Product delivery, with a template and an accept URL template. Rejected because it made Anchor know each Product's URLs and email wording, added settings and a failure mode (a failed send failing the create), and gave the Product less control over the one message a new user reads first. A Product that wants Anchor's SMTP can still call the email API itself.

## Consequences

Accepting an invitation and creating the membership happen in one Anchor transaction. A role stays in use while a member or a pending invitation names it, so deleting it is refused; accepted and expired invitations that name it are deleted with their events. Each change to invitations crosses two repos (Anchor contract and SDK, then EchoPoint).
