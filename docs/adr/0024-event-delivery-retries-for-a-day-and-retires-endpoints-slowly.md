# ADR-0024: Event delivery retries for a day and retires endpoints slowly

**Status:** Accepted, 2026-11-19

## Context

[ADR-0017](0017-product-events-use-standard-webhooks.md) chose Standard Webhooks for product events. The 2026-11-19 audit found four faults in the delivery worker ([`internal/events`](../../apps/anchor/internal/events/)):

- A delivery gave up after 6 attempts in 31 s. An endpoint that was down for a deploy lost every event of that half minute.
- A single 410 Gone deleted the Product's endpoint, whatever URL had answered. Deleting the endpoint deletes its signing secret, so a 410 from a proxy or a deploy broke delivery until an operator saved the endpoint again and changed the secret on the receiver.
- An endpoint could point at loopback, a private network, the tailnet or cloud metadata, so a Product could make Anchor call its own network (SSRF).
- Each replica's pgkit worker runs claimed jobs one at a time (`Worker.scan`). One endpoint that hangs until the 15 s timeout holds every other Product's deliveries. Longer retries make this worse: 20 attempts of 15 s for each queued event.

[Product event delivery](../technical/product-event-delivery.md) describes the resulting behaviour.

## Decision

1. **Retry for about a day.** A deployment gives a delivery 20 attempts. The delay starts at 10 s and doubles to a 2 h cap, with at most a quarter taken off as jitter from a hash of the job and attempt. The first attempt to the last spans 15.6 h to 20.8 h, close to the Standard Webhooks (Svix) schedule. The test suites keep 6 attempts within a second. One `deliveryRetryPolicy` sets both the attempts (at enqueue) and the delay (the worker's `RetryDelay`).
2. **Stop dialing an endpoint that keeps failing.** From 5 failed calls in a row, each failed call opens a cooldown from `last_failed_at` of half the retry curve (5 s, doubled per further failure, capped at 1 h). Half keeps every cooldown below the shortest jittered retry delay, so the breaker never skips a single job's own retries; it only skips the calls of the other queued jobs. During the cooldown an attempt makes no request, does not count as a failed call and retries. The first attempt after the cooldown is the probe: a success closes the breaker for every queued job. The state is two internal columns, `failing_since` and `last_failed_at` (migration 000076).
3. **Retire a gone endpoint slowly, and only its URL.** A 410 is a failed call. It deletes the endpoint only when the stored endpoint still has the URL that answered, has at least 3 failed calls in a row, and its failure streak began at least 12 h ago. One conditional `DELETE` checks all three against the database clock. 12 h is longer than any deploy or proxy fault we expect, and shorter than one event's retry window (15.6 h), so a lone event can still retire a dead endpoint.
4. **Refuse non-public addresses, with an opt-out for test stacks.** Saving an IP literal in a refused range, `localhost` or `*.localhost` answers `400 INVALID_EVENT_ENDPOINT_URL` (the existing code). The delivery dialer checks every address it connects to after DNS; a refused address fails the job at once (`queue.NonRetryable`), as waiting does not make it public. `core.events.allow_private_endpoints` turns both checks off; only the test suites, the e2e runtime and a local run set it.

## Considered Options

- **Keep 6 quick attempts and add a dead-letter replay instead.** Rejected for now: replay needs a product event log, a list API and a replay API ([#131](https://github.com/nanostack-dev/anchor/issues/131)). A day of retries covers the common outage with no new surface. The log and replay stay the follow-up for deliveries that fail for longer.
- **Deliver concurrently with a per-endpoint limit.** It would also remove the cost of an endpoint that succeeds slowly, which the breaker does not. It needs pgkit worker concurrency or a dispatcher in Anchor: a larger change, and a later one.
- **Snooze an endpoint's jobs (`SnoozeTx`) while its breaker is open.** A snooze refunds the attempt, so jobs would never use up their 20 attempts while an endpoint stays dead. That needs a separate age limit, a transaction and a queue client in the deliverer. Failing the attempt fast keeps the schedule bounded, at the cost that a job can spend some attempts without a request.
- **Disable the endpoint after repeated 410s and keep its secret.** Svix does this, and the audit asked for it. It needs a new `delivery_status` value in the API contract (a wire change for the Go SDK, the UI and released clients) and a way to enable the endpoint again. Rejected for this change. The 12 h streak makes a wrong deletion unlikely; the `disabled` state can still be added later.
- **Count only 410s toward retirement.** A receiver that answered 503 for 12 h and then 410 has also gone. Counting every failure keeps one counter, which the API already returns.
- **Keep an accept path for private addresses on deployed environments.** The workspace rule keeps a server-side default for requests that released clients send. Here an accept path would keep the SSRF open, so the refusal applies with no fallback. No released client was found saving a private-address endpoint; the opt-out covers test stacks.
- **Refuse only on save.** Names, other IP spellings and DNS changes (rebinding) pass a save-time check. The dial check is the security boundary; the save check is early feedback.

## Consequences

- An outage shorter than about 15 h loses no event that is alone in the queue for its endpoint. With other events queued, a job can lose its last attempts to a cooldown (below). A delivery that fails for longer is still lost; the worker logs `product event delivery permanently failed`, and the pgkit job stays failed for 30 days.
- A dead or hanging endpoint costs each replica about one call per cooldown instead of one per queued event and attempt. An endpoint that succeeds slowly still holds the replica's other deliveries.
- While the breaker is open, queued jobs spend attempts without a request. A job whose last attempts fall between the endpoint's recovery and the end of the current cooldown (at most 1 h) is lost.
- `GET /v1/products/{id}` keeps returning an endpoint that answers 410, with `delivery_status: failed` and a growing `consecutive_failed_calls`, until a 410 ends a streak that is 12 h old. `consecutive_failed_calls` grows past 20 while an endpoint stays down.
- Removing an endpoint still deletes its signing secret.
- An endpoint that resolves to a private address, even for a moment, fails its deliveries at once and loses those events.
- The test suites drop the 12 h age rule and the breaker, so they assert every failed call and a removal after three 410s. Unit tests cover both deployed rules.
