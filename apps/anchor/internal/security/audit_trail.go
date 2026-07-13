package security

import "context"

type auditTrailKey string

const requestAuditTrailKey auditTrailKey = "request_audit_trail"

// AuditTrail tracks whether an audit log entry was recorded during the
// current request, so the audit middleware can fall back to a generic
// entry for mutations no service hook covered. Requests are handled by a
// single goroutine, so a plain bool is sufficient.
type AuditTrail struct {
	recorded bool
}

// Recorded reports whether a service-level audit entry was written.
func (t *AuditTrail) Recorded() bool { return t.recorded }

// WithAuditTrail attaches a fresh AuditTrail to the context.
func WithAuditTrail(ctx context.Context) (context.Context, *AuditTrail) {
	trail := &AuditTrail{}
	return context.WithValue(ctx, requestAuditTrailKey, trail), trail
}

// MarkAuditRecorded flags the request's AuditTrail, if any, as covered by a
// service-level audit entry. Safe to call with contexts that carry no trail.
func MarkAuditRecorded(ctx context.Context) {
	if trail, ok := ctx.Value(requestAuditTrailKey).(*AuditTrail); ok {
		trail.recorded = true
	}
}
