# Distributed tracing

Anchor emits OpenTelemetry traces through the framework's [`modules/tracing`](https://github.com/nanostack-dev/nanostack-framework/tree/main/modules/tracing) and [`pkg/tracing`](https://github.com/nanostack-dev/nanostack-framework/tree/main/pkg/tracing) (v0.19.0+). Wiring lives in [`cmd/app/app.go`](../../apps/anchor/cmd/app/app.go) and [`cmd/http/server.go`](../../apps/anchor/cmd/http/server.go).

## Configuration

| Variable | Effect |
| --- | --- |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Enables OTLP/HTTP export (deployments use `http://otel-collector:4318`). Unset means a no-op provider; nothing connects anywhere. |
| `OTEL_SERVICE_NAME` | `service.name` resource attribute (`anchor`). |
| `OTEL_TRACES_SAMPLER_ARG` | Ratio for the parent-based sampler; default `1.0`. A sampled incoming parent is always kept. |
| `OTEL_SDK_DISABLED=true` | Forces the no-op provider. |

The W3C TraceContext + Baggage propagator is installed even without an endpoint, so an incoming `traceparent` still sets `trace_id` on log lines.

## Inbound requests

- `tracing.Middleware()` wraps the whole HTTP handler. It starts the server span, continuing a caller's `traceparent` (Echopoint sends one), and skips `/health`.
- `tracing.RouteMiddleware` is the first router middleware. It names the span `METHOD /route/{pattern}` (for example `GET /v1/products/{product_id}`) and sets `http.route`. A request rejected before routing, such as an OpenAPI path-parameter validation failure, keeps the method-only name.
- `requestlog.Contextualize` and the framework log enricher add `trace_id`/`span_id` to request and service log lines, once per line.
- `modules/postgres` records queries as child spans (`sql.conn.query`, `sql.conn.exec`, ...). Queries without a span on their context (migrations, pollers, lock heartbeats) start no trace.
- pgkit (v0.0.44+) stores the enqueuer's W3C context in `pgqueue_jobs.trace_context` (queue schema v3, applied by `EnsureSchema` at startup). Workers run each job under a `pgkit.queue <queue>` consumer span that is a child of that context, so integration-event processing and webhook delivery join the request trace that enqueued them.

## Outbound requests

Calls to hosts outside Nanostack use [`telemetry.ExternalTransport`](../../apps/anchor/internal/telemetry/external.go): each call gets a client span, but no `traceparent`, `tracestate` or `baggage` header is sent.

| Client | Host |
| --- | --- |
| Clerk reconcile ([`clerk/provider.go`](../../apps/anchor/internal/integration/provider/clerk/provider.go)) | Clerk Backend API |
| Product event delivery ([`events/worker.go`](../../apps/anchor/internal/events/worker.go)) | Customer webhook endpoints |

A service-to-service client that calls another Nanostack service should use `tracing.Client`/`tracing.Transport` instead, so the trace continues. Anchor has none today. Build every outbound request with the caller's context (`http.NewRequestWithContext`).

## Local verification

Run a collector with an OTLP/HTTP receiver on `:4318` and a `debug` exporter (`otel/opentelemetry-collector-contrib`), start Anchor with `OTEL_EXPORTER_OTLP_ENDPOINT=http://127.0.0.1:4318`, then send a request with `traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`. The collector prints a `Server` span with that trace id, parent `00f067aa0ba902b7` and the route name, plus `sql.conn.query` children.
