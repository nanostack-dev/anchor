# Product event delivery

How the worker in [`internal/events`](../../apps/anchor/internal/events/) delivers a queued product event to a Product's endpoint. The vocabulary (event, endpoint, delivery) is in [CONTEXT.md](../../CONTEXT.md#product-events); the protocol choice is [ADR-0017](../adr/0017-product-events-use-standard-webhooks.md); the retry schedule, the endpoint breaker, the 410 rule and the address refusal are [ADR-0024](../adr/0024-event-delivery-retries-for-a-day-and-retires-endpoints-slowly.md); the original requirements are the [product events spec](../product-events-spec.md).

## Path

1. A product write calls `events.Emit` inside its transaction. The emitter enqueues one pgkit job on the `product-events` queue in that transaction ([emit.go](../../apps/anchor/internal/events/emit.go)).
2. The worker ([worker.go](../../apps/anchor/internal/events/worker.go)) reads the Product's endpoint for each attempt, so an attempt always goes to the URL saved at that moment. No endpoint, or an event type the endpoint does not subscribe to, completes the job with no request.
3. While the endpoint's breaker is open (see [Failing endpoints](#failing-endpoints)), the attempt ends here with no request and the job retries.
4. The worker signs the body with Standard Webhooks headers and POSTs it. It does not follow redirects. A 2xx completes the job, sets the endpoint's `delivery_status` to `succeeded`, `consecutive_failed_calls` to 0 and clears `failing_since` and `last_failed_at`.
5. Any other answer, or a network error, sets `delivery_status` to `failed`, adds one to `consecutive_failed_calls`, sets `last_failed_at` to now and `failing_since` to now if it was empty, then retries the job. A refused address (see [Endpoint addresses](#endpoint-addresses)) fails the job at once instead.

`failing_since` and `last_failed_at` (migration 000076) are internal: the API does not return them. Saving the endpoint clears them with the counter.

## Retry schedule

One policy ([retry.go](../../apps/anchor/internal/events/retry.go)) sets both the attempts of a job (at enqueue) and the delay between them (the worker's `RetryDelay`).

| Environment | Attempts | First delay | Cap | First attempt to last |
| --- | --- | --- | --- | --- |
| Deployed (prod, dev, development) | 20 | 10 s | 2 h | 15.6 h to 20.8 h |
| Test suites (`core.environment: test`) | 6 | 25 ms | 1 s | 0.58 s to 0.78 s |

The delay doubles after each failed attempt up to the cap, so a deployed delivery waits 10 s, 20 s, 40 s and so on to 85 min, then 2 h for each remaining attempt. This follows the Standard Webhooks (Svix) schedule of about a day. Jitter takes at most a quarter off each delay, derived from a hash of the job id and attempt, so jobs that fail together spread out and the window keeps its floor. pgkit's default backoff jitters over the whole delay, which leaves no floor.

The test suites wait for a delivery to fail six times (`TestProductEventDeliveryStatusOnConfig`), so they keep six attempts within a second.

`consecutive_failed_calls` counts failed attempts across every job of the endpoint, so it can grow well past one job's attempts while an endpoint stays down.

## Failing endpoints

Each replica's pgkit worker runs claimed jobs one at a time (`Worker.scan`), so an endpoint that hangs until the 15 s delivery timeout holds every other Product's deliveries on that replica. A deployment therefore stops dialing an endpoint that keeps failing (`breakerWait` in [retry.go](../../apps/anchor/internal/events/retry.go)):

- From 5 failed calls in a row, each failed call opens a cooldown counted from `last_failed_at`: 10 s after the fifth, doubled per further failure, capped at 2 h, the same curve as the retry schedule.
- During the cooldown an attempt makes no request, is not counted as a failed call and returns a retryable error (`endpoint is cooling down`). The job spends that attempt and retries on its own schedule.
- The first attempt after the cooldown calls the endpoint. A success resets the streak and closes the breaker for every queued job; a failure opens a longer cooldown.

So a dead or hanging endpoint costs each replica about one call per cooldown instead of one per queued event and attempt. The test suites (`core.environment: test`) turn the breaker off: they assert every failed call.

## 410 Gone

Standard Webhooks asks a sender to stop after a 410. The worker records a 410 as a failed call like any other, then removes the endpoint only when all hold, checked in the delete statement itself against the database clock (`DeleteGoneEndpointInternal`):

- the stored endpoint still has the URL that answered 410;
- it has at least 3 consecutive failed calls, the last one this 410;
- its failure streak began at least 12 h ago (`failing_since`). A row with no `failing_since`, such as one that was already failing before migration 000076, is never removed until it fails again.

Otherwise the job retries. So a 410 from a proxy or a deploy, or a short burst of 410s, keeps the endpoint, and a 410 from an old URL, in flight while an operator saved a new one, never removes the new URL. 12 h is shorter than the shortest retry window (15.6 h), so the retries of a single event can still retire a gone endpoint. Removing an endpoint deletes its signing secret, and saving the endpoint again mints a new one. After removal the job completes, and later events find no endpoint.

Before this rule a single 410 removed the endpoint whatever its URL, so `GET` of the product answered without the endpoint after one 410. It now keeps answering with it, `delivery_status: failed`, until a 410 ends a streak of at least 3 failed calls that began 12 h earlier. The test suites keep the count and drop the age (`TestProductEventEndpointGone` expects removal after three 410s).

## Endpoint addresses

An endpoint may only reach public addresses, so a Product cannot point Anchor at its own network ([address.go](../../apps/anchor/internal/events/address.go)). The refused ranges are loopback, private (RFC 1918, IPv6 ULA), shared address space (100.64.0.0/10, which the tailnet uses), link-local (including 169.254.169.254, cloud metadata), unspecified, multicast, documentation, benchmarking and reserved ranges, and the IPv6 prefixes that embed an IPv4 address (NAT64, 6to4, Teredo). An IPv4 address written as IPv6 (`::ffff:127.0.0.1`) is checked as IPv4.

Two checks apply:

1. **On save.** An IP literal in a refused range, `localhost` or a `*.localhost` name answers `400` with code `INVALID_EVENT_ENDPOINT_URL` (the existing code) and the message that the URL must point to a public address. This is early feedback only: names, other IP spellings (decimal, hex) and DNS changes are not visible here.
2. **On every delivery.** The worker's dialer has a `Control` hook that checks each address it actually connects to, after DNS ([transport.go](../../apps/anchor/internal/events/transport.go)). A refused address fails the attempt before any byte is sent and counts as a failed call. The job then fails for good (`queue.NonRetryable`) instead of spending a day of attempts: waiting does not make an address public. A name that resolves to a private address for a moment loses the events delivered in that moment. This is the security boundary; it also covers a name that changes to a private address after the endpoint was saved (DNS rebinding). The client uses no proxy, so the hook sees the endpoint's address and not a proxy's, and it never follows a redirect.

`core.events.allow_private_endpoints` (`EVENTS_ALLOW_PRIVATE_ENDPOINTS`, default `false`) turns both checks off. Only the test suites (`cmd/it/*/application.yaml`, whose endpoints are httptest servers on 127.0.0.1), the e2e runtime and a local run set it. A deployment never does.

## Known limitations

- A job that fails every attempt becomes a failed pgkit job and the worker logs `product event delivery permanently failed`. There is no dead-letter view or replay endpoint yet; failed jobs stay in `pgqueue_jobs` for 30 days ([architecture](architecture.md)). The product event log and replay are tracked in [#131](https://github.com/nanostack-dev/anchor/issues/131).
- The breaker bounds the cost of an endpoint that fails. An endpoint that succeeds slowly (close to the 15 s timeout) still holds the replica's other deliveries; per-endpoint concurrency in the worker would remove that.
- While the breaker is open, queued jobs spend attempts without a request. A job whose last attempts fall between the endpoint's recovery and the end of its cooldown (at most 2 h) is lost.
- `Retry-After` on 429 and 503 is not honoured yet; those answers follow the schedule above.
- The 410 rule counts every failed call in a row, not only 410s, once the streak is 12 h old: an endpoint that answered 503 for 12 h and then one 410 is removed. Removal deletes the signing secret. A `disabled` state that stops delivery but keeps the secret needs a new `ProductEventDeliveryStatus` value, a change to the API contract ([ADR-0024](../adr/0024-event-delivery-retries-for-a-day-and-retires-endpoints-slowly.md)).
