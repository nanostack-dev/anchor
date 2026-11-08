package security

import (
	"context"
	"log/slog"
)

// LogAttrs returns the authenticated caller that ctx carries, as log
// attributes: principal_type with user_id for a platform user, principal_type
// with api_key_id and api_key_name for a product API key, and tenant_id. Auth
// adds them to the request scope with log.SetAttrs, so every later line of the
// request carries the caller without the call site naming it.
//
// Anchor stores identity as independent context values rather than one
// struct, which is why this cannot live in the framework.
//
// A context with no authenticated caller is normal (an unauthenticated
// request, a background pass), so absent values are skipped rather than
// treated as an error. GetCurrentUserID and GetTenantID return a fault when
// the value is missing, which is the right behaviour for callers that require
// it and the wrong one here, so this reads the context directly.
func LogAttrs(ctx context.Context) []slog.Attr {
	var attrs []slog.Attr
	if userID, ok := ctx.Value(currentUserIDKey).(string); ok && userID != "" {
		attrs = append(attrs, slog.String("principal_type", "user"), slog.String("user_id", userID))
	}
	if key, ok := ctx.Value(productAPIKeyKey).(ProductAPIKeyCaller); ok && key.ID != "" {
		attrs = append(attrs,
			slog.String("principal_type", "api_key"),
			slog.String("api_key_id", key.ID),
			slog.String("api_key_name", key.Name),
		)
	}
	if tenantID, ok := ctx.Value(tenantIDKey).(string); ok && tenantID != "" {
		attrs = append(attrs, slog.String("tenant_id", tenantID))
	}
	return attrs
}
