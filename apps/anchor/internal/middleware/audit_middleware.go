package middleware

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"

	"anchor/internal/domain/audit"
	"anchor/internal/security"
	"anchor/internal/service"
)

// AuditMiddleware is the safety net under the service-level audit hooks:
// it observes every mutating request and records a generic audit entry when
//   - no service hook wrote one (a mutation nobody instrumented), or
//   - the request failed (service hooks only run after success, so failed
//     attempts are only ever captured here).
//
// Service hooks stay the primary source: they carry exact action names,
// display-name snapshots and previous-value deltas. Entries written here are
// derived from the route and marked with the request route/status metadata.
type AuditMiddleware struct {
	auditLogService service.AuditLogService
	logger          zerolog.Logger
}

func NewAuditMiddleware(
	auditLogService service.AuditLogService, logger zerolog.Logger,
) *AuditMiddleware {
	return &AuditMiddleware{
		auditLogService: auditLogService,
		logger:          logger.With().Str("component", "audit_middleware").Logger(),
	}
}

const (
	paramProductID      = "product_id"
	paramOrganizationID = "organization_id"
)

// skippedPathSuffixes returns route suffixes the fallback net ignores:
// POSTs that read rather than mutate (/search, /validate, /introspect,
// /preview) and data-plane operations whose volume doesn't belong in a
// management audit log (/sends — one row per transactional email).
func skippedPathSuffixes() []string {
	return []string{"/search", "/validate", "/introspect", "/preview", "/sends"}
}

func (a *AuditMiddleware) Create(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if !isMutatingMethod(r.Method) || isSkippedPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			ctx, trail := security.WithAuditTrail(r.Context())
			ww := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// A panicking mutation handler is exactly the failed attempt the
			// net exists to capture: record it, then let the panic continue.
			defer func() {
				if rec := recover(); rec != nil {
					a.recordFallback(r.WithContext(ctx), trail, http.StatusInternalServerError)
					panic(rec)
				}
			}()

			next.ServeHTTP(ww, r.WithContext(ctx))

			a.recordFallback(r.WithContext(ctx), trail, ww.Status())
		},
	)
}

func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isSkippedPath(path string) bool {
	for _, suffix := range skippedPathSuffixes() {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func (a *AuditMiddleware) recordFallback(r *http.Request, trail *security.AuditTrail, status int) {
	// A service hook already produced the authoritative entry for this
	// request; even a late failure (e.g. response encoding) happened after
	// the recorded mutation committed. Never write a second, contradictory row.
	if trail.Recorded() {
		return
	}
	// Unauthenticated requests carry no tenant/actor context worth recording.
	if status == http.StatusUnauthorized {
		return
	}
	// Not-found failures mutate nothing and are trivially generated in bulk
	// (id guessing); recording each would let one client flood the table.
	if status == http.StatusNotFound {
		return
	}
	success := status < http.StatusBadRequest

	ctx := r.Context()
	if _, err := security.GetTenantID(ctx); err != nil {
		return
	}

	routeCtx := chi.RouteContext(ctx)
	if routeCtx == nil {
		return
	}
	productID := routeCtx.URLParam(paramProductID)
	if productID == "" {
		// Platform-plane routes are out of scope for the product audit log.
		return
	}

	action, targetType := deriveRouteAction(r.Method, routeCtx.RoutePattern())
	if action == "" {
		return
	}

	entry := audit.Log{
		ProductID:  productID,
		Action:     audit.Action(action),
		TargetType: targetType,
		MetadataJSON: audit.Metadata(map[string]any{
			"route":    routeCtx.RoutePattern(),
			"method":   r.Method,
			"status":   status,
			"fallback": true,
		}),
	}
	if !success {
		entry.Outcome = audit.OutcomeFailure
	}
	if orgID := routeCtx.URLParam(paramOrganizationID); orgID != "" {
		entry.OrganizationID = &orgID
	}
	if targetID := lastPathParamValue(routeCtx); targetID != "" {
		entry.TargetID = &targetID
	}

	a.auditLogService.Record(ctx, entry)
}

// deriveRouteAction builds a generic dotted action from the matched route:
// the resource is the last static path segment, the verb comes from the
// HTTP method — e.g. DELETE /v1/products/{product_id}/organizations/
// {organization_id}/api-keys/{api_key_id} -> "api_keys.deleted".
func deriveRouteAction(method, routePattern string) (string, string) {
	resource := ""
	for segment := range strings.SplitSeq(routePattern, "/") {
		if segment == "" || segment == "v1" || strings.HasPrefix(segment, "{") {
			continue
		}
		resource = segment
	}
	if resource == "" {
		return "", ""
	}
	resource = strings.ReplaceAll(resource, "-", "_")

	var verb string
	switch method {
	case http.MethodPost:
		verb = "created"
	case http.MethodPut, http.MethodPatch:
		verb = "updated"
	case http.MethodDelete:
		verb = "deleted"
	default:
		return "", ""
	}

	return resource + "." + verb, resource
}

// lastPathParamValue returns the value of the final path parameter when the
// route ends in one (the target of an update/delete), excluding the scoping
// product_id/organization_id parameters.
func lastPathParamValue(routeCtx *chi.Context) string {
	if len(routeCtx.URLParams.Keys) == 0 {
		return ""
	}
	lastKey := routeCtx.URLParams.Keys[len(routeCtx.URLParams.Keys)-1]
	if lastKey == paramProductID || lastKey == paramOrganizationID {
		return ""
	}
	return routeCtx.URLParam(lastKey)
}
