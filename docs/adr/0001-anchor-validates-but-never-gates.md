# ADR-0001: Anchor validates but never gates

**Status:** Accepted

## Context

Anchor is adding a licensing subsystem: Products declare what their customers are allowed, and Anchor records it. The obvious next step — having Anchor also refuse operations that exceed a limit — is the one we are deliberately not taking.

Anchor cannot see the thing being limited. A flow run happens in echopoint, a document is created in some other Product. Anchor knows the ceiling; only the Product knows the current reality and the exact moment an action is attempted.

An earlier framing used the single word "enforce" for two different things, and the ambiguity produced real confusion during design: *enforcing a limit* (blocking an action) and *enforcing a rule* (rejecting a malformed write) have opposite answers here.

## Decision

Anchor **validates** and never **gates**.

- **validate** — reject a malformed write. Anchor always does this. A limit that violates its schema's validation rules is rejected at the license write.
- **gate** — block an action because a limit is reached. Anchor never does this. Every allow/deny decision belongs to the consumer Product.

The most Anchor exposes is a derived `status` per limit — `within_limit`, `at_limit`, `exceeded`, `stale` — computed on read from the latest usage and the current limit. It is advice, never a verdict.

The SDK ([ADR-0004](0004-license-schema-template-and-copy.md) covers its shape) may compute that status client-side, but returns a *decision value*, never a blocking call and never an error meaning "denied". The `if` statement that blocks a user lives in the consumer's own repository. This is testable: grep for the gate and it is in echopoint.

## Consequences

**Good.** Anchor's usage writes need no transactional exactness, because no decision is made from them at write time. That removes a lock per usage event, removes the need for exact counting, and makes a stale cache harmless rather than dangerous.

**Good.** Consumers keep control of their own failure modes — including deciding that a `stale` license means *allow*, which is the recommended default. A licensing system that blocks paying customers because a cache expired is worse than one that lets a few extra flows through.

**Cost.** Limits are advisory. A Product that ignores the status is unconstrained, and Anchor cannot stop it. This is accepted: Anchor's consumers are first-party or trusted, and a Product that wants to cheat its own licensing already controls its own code.

**Cost.** There is no central kill switch. Cutting off an organization means every Product doing it, not one flag in Anchor.

**Watch for.** The temptation to ship `sdk.MustBeWithinLimit()` or a middleware that rejects. That quietly relocates gating logic into Anchor's repository, versioned by Anchor and debugged by Anchor's maintainers for consumers they did not write. If it ever seems necessary, reopen this ADR rather than working around it.
