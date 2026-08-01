package security

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"
)

// NewLogEnricher returns the log.Enricher that publishes the authenticated
// caller onto every log line emitted under a bound context.
//
// Anchor stores identity as two independent context values rather than one
// struct, which is why this cannot live in the framework: the framework has no
// way to know the shape. Registering it through group:"log.enrichers" is what
// lets the shared package stay ignorant of it.
//
// A context with no authenticated caller is normal — an unauthenticated
// request, a background pass — so absent values are skipped rather than
// treated as an error. GetCurrentUserID and GetTenantID return a fault when
// the value is missing, which is the right behaviour for callers that require
// it and the wrong one here, so this reads the context directly.
func NewLogEnricher() log.Enricher {
	return log.EnricherFunc(func(ctx context.Context, c zerolog.Context) zerolog.Context {
		if userID, ok := ctx.Value(currentUserIDKey).(string); ok && userID != "" {
			c = c.Str("user_id", userID)
		}
		if tenantID, ok := ctx.Value(tenantIDKey).(string); ok && tenantID != "" {
			c = c.Str("tenant_id", tenantID)
		}
		return c
	})
}
