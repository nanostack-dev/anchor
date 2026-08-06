# ADR-0003: Usage is reported as absolute snapshots, not deltas

**Status:** Accepted

## Context

Consumers report usage to Anchor so it can be shown, kept as history, and compared against limits. Two shapes were available: deltas (`+1 flow run`) that Anchor sums, or absolute snapshots (`this organization now has 37 flows`) that replace the previous value.

Because Anchor never gates ([ADR-0001](0001-anchor-validates-but-never-gates.md)), exactness at write time buys nothing — no decision is made from the number at the moment it lands.

## Decision

Consumers report **absolute snapshots**. Anchor never sums.

A usage report carries a key, a value, and an optional window (`window_start` / `window_end`, half-open) distinguishing a gauge from a windowed counter:

- **gauge** — window absent. "37 flows exist right now." Rises and falls.
- **windowed counter** — window present. "412 runs in the period 2026-08-14 to 2026-09-14." Resets by starting a new window.

The window is carried explicitly rather than inferred from a formatted string, because real billing periods follow the subscription anniversary rather than the calendar. "August 14 to September 14" is not expressible as `"2026-08"`, and a string cannot carry a timezone.

Reports are stored as immutable observations, appended — never one row per window. Granularity of history is a separate concern from the window a limit applies to; see [ADR-0005](0005-timescaledb-for-usage-history.md).

## Consequences

**Good.** Retries are harmless. The same value landing twice changes nothing, so no idempotency key is needed on the hot path.

**Good.** Drift cannot accumulate. A missed report self-heals on the next one, because every report is a full resync. With deltas, one lost write means permanent skew and a reconciliation job.

**Good.** Rate at any granularity is still derivable, by differencing consecutive observations. This is safe *because* the window is explicit — a counter reset is a new window, never an ambiguous cliff within one series. It is also what makes downsampling by thinning correct rather than lossy.

**Cost.** Usage-based billing cannot be built on this data. Reconstructing exact event counts from sampled snapshots is not possible. If per-unit billing is ever wanted, it needs a real metering pipeline with idempotency keys and a dedup window — not this table doing double duty.

**Cost.** Consumers must be able to compute their own absolute usage. In practice this is trivial (`SELECT count(*) … WHERE organization_id = $1`) and more accurate than anything Anchor could accumulate.
