package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

type webhookHeadersKey struct{}
type webhookPayloadKey struct{}

const maxWebhookPayloadBytes = 1 << 20

func WebhookPayloadMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost ||
			!strings.HasPrefix(r.URL.Path, "/v1/products/") ||
			!strings.Contains(r.URL.Path, "/integrations/webhooks/") {
			next.ServeHTTP(w, r)
			return
		}
		defer r.Body.Close()
		payload, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxWebhookPayloadBytes))
		if err != nil {
			http.Error(w, "Cannot read webhook body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(payload))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), webhookPayloadKey{}, payload)))
	})
}

func webhookPayloadFromContext(ctx context.Context) ([]byte, bool) {
	payload, ok := ctx.Value(webhookPayloadKey{}).([]byte)
	return payload, ok
}

// WebhookHeadersFromContext retrieves HTTP headers stored in context by the
// webhook headers middleware. Returns an empty map if no headers are present.
func WebhookHeadersFromContext(ctx context.Context) map[string]string {
	if v, ok := ctx.Value(webhookHeadersKey{}).(map[string]string); ok {
		return v
	}
	return map[string]string{}
}

// webhookHeaderPrefixes returns header name prefixes that are forwarded into
// context for webhook signature validation (e.g. svix-id, svix-timestamp,
// svix-signature).
func webhookHeaderPrefixes() []string { return []string{"svix-", "webhook-"} }

// NewWebhookHeadersMiddleware returns a StrictMiddlewareFunc that, for the
// IngestWebhook operation, copies relevant HTTP headers into the request
// context so the handler can access them for signature validation.
func NewWebhookHeadersMiddleware() StrictMiddlewareFunc {
	return func(f StrictHandlerFunc, operationID string) StrictHandlerFunc {
		return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
			if operationID == "IngestWebhook" {
				headers := extractWebhookHeaders(r)
				ctx = context.WithValue(ctx, webhookHeadersKey{}, headers)
			}
			return f(ctx, w, r, request)
		}
	}
}

// extractWebhookHeaders copies headers relevant to webhook signature
// validation from the HTTP request. Header names are lowercased.
func extractWebhookHeaders(r *http.Request) map[string]string {
	headers := make(map[string]string)
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		for _, prefix := range webhookHeaderPrefixes() {
			if strings.HasPrefix(lower, prefix) && len(values) > 0 {
				headers[lower] = values[0]
				break
			}
		}
	}
	return headers
}
