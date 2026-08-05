# ADR-0007: Licensing is Anchor's first feature slice

**Status:** Accepted

## Context

Anchor's code is laid out flat: `internal/service/*.go`, `internal/api/*.go`, with domain types in `internal/domain/<entity>/`. echopoint uses vertical feature slices — `internal/feature/<plural>/` holding `module.go`, `handler.go`, `service.go`, `repository.go`, mappers, and errors together.

The shared engineering rules name echopoint as canonical and Anchor as lagging: *"echopoint = canonical reference; anchor lags — match echopoint, don't copy anchor's older patterns."*

Licensing is the largest new subsystem Anchor has taken on. Building it flat would entrench the pattern our own rules call outdated, in the newest code in the repository.

## Decision

Licensing is built as `internal/feature/licenses/`, following echopoint's slice layout:

```
internal/feature/licenses/
├── module.go              fx wiring
├── handler.go
├── service.go             validation, status derivation
├── repository.go          go-jet, hypertable reads
├── api_mapper.go
├── db_mapper.go
├── errors.go
├── validation_errors.go
└── rules/                 validation-rule evaluator, import-clean
```

The `rules/` subpackage holds the structured-rule evaluator from [ADR-0004](0004-license-schema-template-and-copy.md) and imports nothing licensing-specific. It is a candidate for later extraction into `nanostack-framework` — but only once a second real caller exists. Today the email subsystem has no validation rules, so extracting now would produce "shared" code with exactly one caller, moved somewhere harder to change.

## Consequences

**Good.** The migration Anchor's rules already call for gets a first step, on greenfield code where it costs nothing. There is no existing service to untangle, no behaviour to preserve, no risk.

**Good.** The slice gives `rules/` a natural home. The flat layout has nowhere obvious to put an import-clean subpackage.

**Cost.** Anchor carries two layouts until the rest migrates. A newcomer sees `internal/service/organization_service.go` beside `internal/feature/licenses/service.go` and has to learn both. This is a genuine cost and not worth pretending away.

**Risk.** The migration stalls at one slice and Anchor sits mixed indefinitely. Mitigate by recording the intended direction in `AGENTS.md` so it is a stated trajectory rather than an unexplained inconsistency — not by avoiding the first step.
