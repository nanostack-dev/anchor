# ADR-0007: Licensing is Anchor's first feature slice

**Status:** Accepted

## Context

Anchor's code is laid out flat: `internal/service/*.go`, `internal/api/*.go`, with domain types in `internal/domain/<entity>/`. echopoint uses vertical feature slices — `internal/feature/<plural>/` holding `module.go`, `handler.go`, `service.go`, `repository.go`, mappers, and errors together.

The shared engineering rules name echopoint as canonical and Anchor as lagging: *"echopoint = canonical reference; anchor lags — match echopoint, don't copy anchor's older patterns."*

Licensing is the largest new subsystem Anchor has taken on. Building it flat would entrench the pattern our own rules call outdated, in the newest code in the repository.

## Decision

Licensing is built as a self-contained package tree with its own fx module, following the shape `internal/email/` already established in this repository:

```
internal/license/
├── module.go              fx wiring
├── service/               validation, status derivation
├── repository/            go-jet, hypertable reads
└── rules/                 validation-rule evaluator, import-clean
```

**Amended after implementation began.** This ADR originally specified `internal/feature/licenses/` with `handler.go` inside the slice, copying echopoint verbatim. That is not achievable here: anchor's generated API is a single `StrictServerInterface` implemented by one `AnchorAPI` struct, so no feature package can own its own HTTP handler without regenerating the API surface into per-tag interfaces — a change well outside this work.

`internal/email/` is the real in-repo precedent and already achieves the goal: a subsystem's layers co-located in its own tree with its own module, rather than smeared across the flat `internal/service` and `internal/repository` packages. Matching it introduces **one** new pattern to anchor rather than two.

API handler methods therefore stay in `internal/api`, alongside the existing email handlers, and delegate to the licensing service. That is the seam the generated interface forces.

The `rules/` subpackage holds the structured-rule evaluator from [ADR-0004](0004-license-schema-template-and-copy.md). It is pure logic — a rule set and a value in, a violation out — with no state and no database, which is what lets its combinatorial matrix be table-tested directly instead of through a hundred HTTP round-trips. That testability is why it is a separate package. It is not staged for extraction into `nanostack-framework`; it is licensing's, and it stays here.

## Consequences

**Good.** The migration Anchor's rules already call for gets a first step, on greenfield code where it costs nothing. There is no existing service to untangle, no behaviour to preserve, no risk.

**Good.** The slice gives `rules/` a natural home. The flat layout has nowhere obvious to put an import-clean subpackage.

**Cost.** Anchor carries two layouts until the rest migrates. A newcomer sees `internal/service/organization_service.go` beside `internal/feature/licenses/service.go` and has to learn both. This is a genuine cost and not worth pretending away.

**Risk.** The migration stalls at one slice and Anchor sits mixed indefinitely. Mitigate by recording the intended direction in `AGENTS.md` so it is a stated trajectory rather than an unexplained inconsistency — not by avoiding the first step.
