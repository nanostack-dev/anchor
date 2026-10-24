package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"anchor/internal/telemetry"

	"github.com/nanostack-dev/nanostack-framework/pkg/tracing"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestExternalTransportRecordsClientSpanWithoutPropagating(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previousProvider, previousPropagator := otel.GetTracerProvider(), otel.GetTextMapPropagator()
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(tracing.Propagator())
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		otel.SetTextMapPropagator(previousPropagator)
	})

	received := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	ctx, parent := provider.Tracer("test").Start(context.Background(), "parent")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	resp, err := (&http.Client{Transport: telemetry.ExternalTransport(nil)}).Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	parent.End()

	headers := <-received
	require.Empty(t, headers.Get("Traceparent"))
	require.Empty(t, headers.Get("Tracestate"))
	require.Empty(t, headers.Get("Baggage"))

	spans := recorder.Ended()
	require.Len(t, spans, 2)
	client := spans[0]
	require.Equal(t, parent.SpanContext().TraceID(), client.SpanContext().TraceID())
	require.Equal(t, parent.SpanContext().SpanID(), client.Parent().SpanID())
}
