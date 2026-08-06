# ADR-0005: Usage history is stored and aggregated by TimescaleDB

**Status:** Accepted

## Context

Usage observations accumulate quickly. At five-minute granularity, one key for one organization produces roughly 105,000 rows a year; a handful of keys across a few hundred organizations reaches hundreds of millions. History is an explicit requirement, so plain TTL deletion is not acceptable — the data has to be kept, just more coarsely as it ages.

The obvious hand-rolled design is a scheduled pass under `pglock` that thins old rows down to one per coarser bucket, plus a configured bucket granularity applied on write.

Two facts made that unnecessary:

- Anchor's database **is already TimescaleDB**. `docker-compose.yml` runs the `timescale/timescaledb` image, and infra provisions it on the shared data node. Anchor calls no Timescale function today — the extension sits idle.
- echopoint **already runs this exact pattern**. Its `000001_init.up.sql` creates a `request_metrics` hypertable and a cascading continuous-aggregate chain, `1 minute` → `1 hour` → `1 day`.

This collides with a standing repository invariant: *business rules live in the service layer, never in SQL.*

## Decision

Usage observations go into a **hypertable**. Granularity comes from a **cascading continuous aggregate** chain, mirroring echopoint's `request_metrics`. Retention and storage are handled by `add_retention_policy` and `add_compression_policy` rather than a custom pass.

Aggregates use `last(value, observed_at)` per bucket — never `sum` or `avg`. This follows directly from [ADR-0003](0003-usage-reported-as-snapshots.md): the values are snapshots, so the last observation in a bucket *is* the bucket's value. This is also what makes dropping raw chunks safe — the coarser aggregate already holds the data.

**The invariant gains one explicit exception:**

> Business rules live in the service layer, never in SQL — **except time-series aggregation, which is delegated to TimescaleDB.** `time_bucket`, continuous aggregates, retention and compression policies are storage concerns, in the same category as an index. Reimplementing them in Go would be strictly worse than using the engine built for them.

Deriving license status from usage remains in Go. The line is: *shaping and storing the series* is SQL's job; *interpreting it against a limit* is the service layer's.

## Consequences

**Good.** Most of the hand-rolled design disappears: no configured bucket-on-write, no thinning job, no `pglock` scheduling, no rollup table.

**Good.** Compression on time-series data is substantial, and it comes from a policy rather than code.

**Good.** The pattern is already proven in-house by echopoint, on the same extension and the same team.

**Cost.** The invariant now has an exception, and exceptions erode. Any future proposal to put logic in SQL must justify itself against *this* ADR, not treat it as precedent for a second exception.

**Cost.** Plain PostgreSQL no longer runs Anchor. This was already effectively true — the project's own compose file ships the Timescale image — but it is now load-bearing rather than incidental, and should be stated in the deployment documentation.

**Verify during implementation.** How go-jet handles continuous-aggregate views, and whether reads require raw SQL for `time_bucket`. Neither is expected to block, but neither has precedent in this repository.

**Related risk.** `anchor/docker-compose.yml` pins `timescale/timescaledb:2.16.1-pg16` while production runs `2.23.0-pg18`. Integration tests would validate this feature against a different engine than deploys use. That drift must be closed before this work lands.
