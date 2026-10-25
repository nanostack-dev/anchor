// Package telemetry holds anchor's tracing helpers on top of the framework's
// pkg/tracing.
package telemetry

import (
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/tracing"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
)

// ExternalTransport wraps base (http.DefaultTransport when nil) for calls to
// hosts outside Nanostack, such as Clerk or customer webhook endpoints. Each
// call gets a client span, but no traceparent, tracestate or baggage header is
// sent, so internal trace ids never leave the platform.
func ExternalTransport(base http.RoundTripper) http.RoundTripper {
	return tracing.Transport(base, otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator()))
}
