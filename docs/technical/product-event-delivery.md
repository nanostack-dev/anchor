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

## Known limitations

- A job that fails every attempt becomes a failed pgkit job and the worker logs `product event delivery permanently failed`. There is no dead-letter view or replay endpoint yet; failed jobs stay in `pgqueue_jobs` for 30 days ([architecture](architecture.md)).
- `Retry-After` on 429 and 503 is not honoured yet; those answers follow the schedule above.
