# ADR-0020: Self-rescheduling jobs collapse duplicates after enqueue

**Status:** Accepted, 2026-10-08

## Context

The Clerk reconcile scheduler is a pgqueue job that inserts its own successor on every run ([Clerk reconcile scheduler](../technical/clerk-reconcile-scheduler.md)). pgqueue has no unique jobs, and the successor insert and the worker's ack are separate statements. A run whose ack is lost is reaped after the visibility timeout and runs again, inserting a second successor. From then on two chains reschedule themselves forever. Dev ran six chains for 14+ days, and two reconciles of the same instance raced on a new user ([postmortem](../postmortems/2026-10-07-duplicate-clerk-reconcile.md)).

## Decision

After inserting its successor, every run of a self-rescheduling job deletes all other pending copies of itself except the one with the lowest id. Every replica keeps the same id, so concurrent runs converge on one chain without a lock. Each deletion logs a warning, so a fork is visible.

A job that works on one resource takes a `pglock` advisory lock keyed on that resource and skips when another run holds it, because a fork, a retry or a reaped rerun can still run a copy in parallel.

## Considered Options

- **Lock at enqueue.** One function, shared by the startup seed, the first-key start and the re-enqueue, takes a blocking advisory lock and inserts a successor only when none is pending. It prevents the fork rather than removing it, and it also closes the race between the startup seed and a running scheduler. Not chosen: `pglock` offers only try-locks, and a re-enqueue that gives up on contention can leave zero chains (the seed sees the run as `processing` and skips too). It needs a new blocking primitive, while the collapse converges with the primitives that exist and was already covered by a component test.
- **Unique jobs in pgkit**, as in River (`UniqueOpts`), Oban (`unique`), graphile-worker (`job_key`) and pg-boss (`singletonKey`). A partial unique index on pending jobs breaks the reaper: `ReapStuckJobs` is one bulk `UPDATE` back to `pending`, and a reaped run colliding with its pending successor would fail the statement for every queue. It works only with a key per time window, unique across all statuses, and costs a pgkit schema migration and release. Revisit when a second self-rescheduling job appears.

## Consequences

A fork still exists briefly: the run that created it has already done its work once more before its successor is deleted. Per-resource work must therefore tolerate a duplicate run, which the per-resource lock provides. A new self-rescheduling job must call the same collapse step after enqueueing its successor ([implementation rules](../development/agent-rules.md)).
