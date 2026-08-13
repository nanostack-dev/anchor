# ADR-0012: Usage shape is declared, not inferred

**Status:** Accepted

Amends [ADR-0003](0003-usage-reported-as-snapshots.md), which let a report's own window presence decide whether it was a gauge or a windowed counter.

## Context

[ADR-0003](0003-usage-reported-as-snapshots.md) gives a usage report two shapes distinguished by one field: a window present means a windowed counter, a window absent means a gauge. Nothing pins that shape to the license field itself — a report against `flows` could carry a window today and omit it tomorrow, and Anchor would store both exactly as authored.

Every comparable system that separates "what does this metric mean" from "what value was just reported" pins the meaning at declaration, not at the data point:

- Prometheus fixes a metric's type — counter, gauge, histogram — when it is registered. `http_requests_total` is always a counter; the client library's counter type has no `Set()` method, so a gauge-shaped write does not compile against it. The type travels with the metric name, never with an individual sample.
- OpenTelemetry's metrics API separates instrument kinds the same way, chosen at instrument-creation time: `system.memory.usage` is declared an `UpDownCounter` (gauge-like — rises and falls), `http.server.request.duration` a `Histogram`. Neither is decided per data point.
- Stripe's usage-based billing meters declare an aggregation formula on the meter itself when it is created — an `api_requests` meter declared `sum`. Every event reported against that meter is interpreted the declared way; the event itself carries no formula.
- Datadog metric types (`gauge`, `count`, `rate`) are fixed per metric name in its submission config — `system.disk.read.bytes` is always a `gauge` — not chosen anew by whichever integration happens to emit the next point.

None of them let shape ride along with the data. Anchor's own read side already assumes a series means one consistent thing throughout: [ADR-0005](0005-timescaledb-for-usage-history.md)'s continuous aggregates roll a series up by summing or averaging it, and ADR-0003 itself calls differencing consecutive observations "safe" — both only hold if every point in the series is the same shape. Only the write side left that question open, per report.

### Concretely, this is what stays silently wrong

A Product declares `flows` as a limit with no shape pinned (what today's code allows). A consuming service reports its first billing period the way ADR-0003 always allowed: `POST {key: "flows", value: 500, from: "2026-08-01", to: "2026-09-01"}` — a windowed counter. The report is accepted and stored. Three weeks later, a different code path in the same service reports `POST {key: "flows", value: 37}` — no window, meaning "37 flows exist right now" — a gauge. That report is accepted too; nothing compares it against the previous report's shape, because no shape was ever declared to compare against. The series for `flows` now mixes an accumulating count with a point-in-time reading, in the same key. Anything that reads the series and differences consecutive points to derive a rate — the exact operation ADR-0003 calls safe — computes a delta between a windowed total and an instantaneous count. The result is a number with no meaning, and nothing in the stored rows flags which one is the odd point out; it looks like ordinary usage history until someone tries to chart it.

## Decision

**A license field's usage shape — `GAUGE` or `WINDOWED_COUNTER` — is declared once, on the schema field, and every usage report against that field is checked against it.**

- `usage_shape` is a new column on `license_schema_fields`: mandatory when `type = LIMIT`, refused for every other type. Both directions are enforced in the service layer (`declareFields`), not a `CHECK` constraint — this repository keeps business rules out of SQL.
- A usage report's window presence must match its field's declared shape: `GAUGE` refuses a report that carries a window (`from`/`to` set), `WINDOWED_COUNTER` refuses one that omits it. Both are refused with `USAGE_SHAPE_MISMATCH`, naming the field and its declared shape.
- A limit declared before this ADR carries no `usage_shape` — the column has no backfill, because only the field's own owner knows which shape its existing history means. A report against such a field is refused with `LICENSE_FIELD_USAGE_SHAPE_UNDECLARED` rather than guessed at; redeclaring the field is what fills it in.

This still validates rather than gates ([ADR-0001](0001-anchor-validates-but-never-gates.md)): the check is about which *shape* a value takes, never about the value itself or about a limit being exceeded.

## Consequences

**Good.** A series for one key means one thing for its whole life, so the read side's assumptions — continuous aggregates rolling it up, differencing it for a rate — hold in fact, not only in the shapes that happened to be reported so far.

**Good.** A shape mismatch is caught at write time, against the one report that's wrong, rather than discovered later by whoever reads the series and gets a rate that makes no sense.

**Cost.** Declaring a limit now requires one more decision up front: gauge or windowed counter. For most limits this is obvious from what they measure — concurrent connections versus monthly API calls — so it is rarely a hard call, but it is one more field a schema author cannot skip.

**Cost.** A limit declared before this ADR needs a follow-up write — redeclaring the field with a shape — before it can accept usage again. There is no migration-time backfill, for the same reason [ADR-0009](0009-every-license-field-is-mandatory.md) leaves a schema widening as a two-step operator task rather than inferring a value only a human can actually know.
