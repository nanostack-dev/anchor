# Coding style

Practices this repo has learned the hard way, during code review. Read before writing or reviewing Go code here. Anchor-specific — unlike `docs/engineering-best-practices.md`, this file is not synced with echopoint, so it's the place for lessons that don't need to travel there too.

## Reuse before you build

Before writing a small generic helper — map a slice, coalesce a pointer to a default, diff two collections — check `nanostack-framework/pkg/` first. `slicex`, `ptr`, `search`, and `transactor` already cover most of what a handler or mapper needs. Twice in one PR, a hand-rolled helper turned out to duplicate a framework one already in use elsewhere in this codebase:

- `mapItems[T, R any](items []T, mapper func(T) R) []R` reinvented `slicex.Map` — already the convention across `internal/api` (a prior PR had migrated every other handler to it).
- `valueOr[T any](ptr *T, fallback T) T` reinvented `ptr.DerefOr` — already used elsewhere in this repo.

Before adding a one-off generic helper: `grep -r` the shape you're about to write against `slicex`/`ptr`/`search` first, and skim `nanostack-framework/pkg/` if it looks generic enough that a shared package might already own it.

## Verify behavior before swapping in a replacement

Reuse is not find-and-replace. `mapItems` always allocated `make([]R, 0, len(items))`, turning a nil input into a non-nil empty slice; `slicex.Map` deliberately preserves a nil input as nil. At one call site — `mapLicenseDiffToResponse` — that difference mattered: `DiffValues` returns nil, on purpose, for "nothing differs," and the OpenAPI contract's `differences` field is a required, non-nullable array. A blind swap would have turned `"differences": []` into `"differences": null` for that case.

Before replacing hand-rolled code with a library or framework equivalent, trace every call site's input back to where it's produced and check whether nil, zero-value, or empty-vs-absent behavior differs between the two. Where the difference is load-bearing, keep it explicit — a guard plus a short comment — rather than losing it silently.

## Comments carry the why, sized to what actually needs explaining

A comment earns its place by telling a reader something the code can't say on its own — not by restating what's already visible. A four-line function doesn't need a five-line doc comment; a genuinely non-obvious invariant does (the nil-guard above is a real example). When in doubt: write it, then reread asking "would someone who can already read Go learn something new here?" If not, cut it.

## Component-test coverage, and assert what actually distinguishes the states

Every new validation branch — both declare-time and use-time — needs an end-to-end component test through the real HTTP API, not just unit coverage of the function implementing it.

Check that the assertion you write can actually fail the way you intend it to. `assert.Empty(t, diff.Differences)` passes identically whether the JSON was `[]` or `null` — it only checks length. Proving "this field reads back as `[]`, never `null`" needs `assert.NotNil` alongside it, since the generated field is a plain, non-pointer slice and only that distinguishes the two JSON shapes once decoded into Go.

## Factor test setup only where it's actually duplicated

Turn repeated raw setup into a named fixture — a method on a `*World`/handle type, matching this repo's existing pattern — once it's genuinely repeated, not preemptively. `licenseWorld.InsertGaugeObservation`/`InsertWindowedObservation` replaced a dozen near-identical `insertRawObservation(t, w.tenantID, w.productID(), w.OrganizationID(), ...)` calls across one test file. A pagination-literal pattern in the same file, by contrast, appeared exactly once — building a fixture for it would have been abstraction with no reuse, so it stayed inline, just tidied with `new(v)`.

The same logic decides which package owns a fixture. The cross-package test DSL (`cmd/it/shared/dsl`) is deliberately API-only: every builder step arranges state through the generated HTTP client, never raw SQL. A fixture that needs direct DB access — to place `observed_at` in the past, say — belongs local to the CT package that needs it, not bolted onto the shared DSL.

The HTTP plumbing around an act belongs in the DSL too — the credential, the request build, and the `require.NoError` / status / `NotNil` triplet, plus a `*Raw` twin for the tests whose subject is a refusal. `itdsl.OrganizationClient` is the shape to copy. See `docs/engineering-best-practices.md`.
