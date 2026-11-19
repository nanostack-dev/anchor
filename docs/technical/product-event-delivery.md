# Product event delivery

How the worker in [`internal/events`](../../apps/anchor/internal/events/) delivers a queued product event to a Product's endpoint. The vocabulary (event, endpoint, delivery) is in [CONTEXT.md](../../CONTEXT.md#product-events); the protocol choice is [ADR-0017](../adr/0017-product-events-use-standard-webhooks.md); the original requirements are the [product events spec](../product-events-spec.md).

## Path

1. A product write calls `events.Emit` inside its transaction. The emitter enqueues one pgkit job on the `product-events` queue in that transaction ([emit.go](../../apps/anchor/internal/events/emit.go)).
2. The worker ([worker.go](../../apps/anchor/internal/events/worker.go)) reads the Product's endpoint for each attempt, so an attempt always goes to the URL saved at that moment. No endpoint, or an event type the endpoint does not subscribe to, completes the job with no request.
3. The worker signs the body with Standard Webhooks headers and POSTs it. It does not follow redirects. A 2xx completes the job and resets the endpoint's `delivery_status` to `succeeded` and `consecutive_failed_calls` to 0.
4. Any other answer, or a network error, sets `delivery_status` to `failed`, adds one to `consecutive_failed_calls` and retries the job.

## Retry schedule

One policy ([retry.go](../../apps/anchor/internal/events/retry.go)) sets both the attempts of a job (at enqueue) and the delay between them (the worker's `RetryDelay`).

| Environment | Attempts | First delay | Cap | First attempt to last |
| --- | --- | --- | --- | --- |
| Deployed (prod, dev, development) | 20 | 10 s | 2 h | 15.6 h to 20.8 h |
| Test suites (`core.environment: test`) | 6 | 25 ms | 1 s | 0.58 s to 0.78 s |

The delay doubles after each failed attempt up to the cap, so a deployed delivery waits 10 s, 20 s, 40 s and so on to 85 min, then 2 h for each remaining attempt. This follows the Standard Webhooks (Svix) schedule of about a day. Jitter takes at most a quarter off each delay, derived from a hash of the job id and attempt, so jobs that fail together spread out and the window keeps its floor. pgkit's default backoff jitters over the whole delay, which leaves no floor.

The test suites wait for a delivery to fail six times (`TestProductEventDeliveryStatusOnConfig`), so they keep six attempts within a second.

`consecutive_failed_calls` counts failed attempts across every job of the endpoint, so it can grow well past one job's attempts while an endpoint stays down.

## 410 Gone

Standard Webhooks asks a sender to stop after a 410. The worker records a 410 as a failed call like any other, then removes the endpoint only when both hold, checked in the delete statement itself (`DeleteGoneEndpointInternal`):

- the stored endpoint still has the URL that answered 410;
- it has at least 3 consecutive failed calls, the last one this 410.

Otherwise the job retries. So a single 410 from a proxy or a deploy keeps the endpoint, and a 410 from an old URL, in flight while an operator saved a new one, never removes the new URL. Removing an endpoint deletes its signing secret, and saving the endpoint again mints a new one. After removal the job completes, and later events find no endpoint.

Before this rule a single 410 removed the endpoint whatever its URL, so `GET` of the product answered without the endpoint after one 410. It now keeps answering with it, `delivery_status: failed`, until the third failed call in a row.

## Endpoint addresses

An endpoint may only reach public addresses, so a Product cannot point Anchor at its own network ([address.go](../../apps/anchor/internal/events/address.go)). The refused ranges are loopback, private (RFC 1918, IPv6 ULA), shared address space (100.64.0.0/10, which the tailnet uses), link-local (including 169.254.169.254, cloud metadata), unspecified, multicast, documentation, benchmarking and reserved ranges, and the IPv6 prefixes that embed an IPv4 address (NAT64, 6to4, Teredo). An IPv4 address written as IPv6 (`::ffff:127.0.0.1`) is checked as IPv4.

Two checks apply:

1. **On save.** An IP literal in a refused range, `localhost` or a `*.localhost` name answers `400` with code `INVALID_EVENT_ENDPOINT_URL` (the existing code) and the message that the URL must point to a public address. This is early feedback only: names, other IP spellings (decimal, hex) and DNS changes are not visible here.
2. **On every delivery.** The worker's dialer has a `Control` hook that checks each address it actually connects to, after DNS ([transport.go](../../apps/anchor/internal/events/transport.go)). A refused address fails the attempt before any byte is sent, counts as a failed call and retries on the schedule. This is the security boundary; it also covers a name that changes to a private address after the endpoint was saved (DNS rebinding). The client uses no proxy, so the hook sees the endpoint's address and not a proxy's, and it never follows a redirect.

`core.events.allow_private_endpoints` (`EVENTS_ALLOW_PRIVATE_ENDPOINTS`, default `false`) turns both checks off. Only the test suites (`cmd/it/*/application.yaml`, whose endpoints are httptest servers on 127.0.0.1), the e2e runtime and a local run set it. A deployment never does.

## Known limitations

- A job that fails every attempt becomes a failed pgkit job and the worker logs `product event delivery permanently failed`. There is no dead-letter view or replay endpoint yet; failed jobs stay in `pgqueue_jobs` for 30 days ([architecture](architecture.md)).
- Each replica's pgkit worker runs claimed jobs one at a time (`Worker.scan`). An endpoint that hangs until the 15 s delivery timeout delays every other Product's deliveries on that replica, and the day-long schedule retries it up to 20 times per event.
- `Retry-After` on 429 and 503 is not honoured yet; those answers follow the schedule above.
- The 410 threshold counts every failed call in a row, not only 410s: two 503s and then one 410 remove the endpoint, and so do three queued events that each get one 410. A `disabled` state that stops delivery but keeps the signing secret needs a new `ProductEventDeliveryStatus` value or column.
