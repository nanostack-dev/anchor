package security

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
)

// Security requirements used to reach the middleware through oapi-codegen's
// generated `<Scheme>Scopes` context keys. That mechanism is deprecated
// upstream (oapi-codegen#1524) because it flattens the OpenAPI `security`
// block: it records which schemes an operation mentions and the scopes each
// carries, but not how they combine, so alternative schemes (OR), combined
// schemes (AND) and anonymous alternatives all look alike once written into the
// context. The requirements now come from the OpenAPI document, resolved per
// request by the auth middleware and carried here with their structure intact.

// Scheme names as declared under `components.securitySchemes` in
// cmd/http/openapi.yaml.
const (
	SchemePlatformBearerAuth = "platformBearerAuth"
	SchemeProductAPIKeyAuth  = "productApiKeyAuth"
)

type requirementsKey struct{}

// WithRequirements returns a context carrying the security requirements of the
// operation being served.
func WithRequirements(ctx context.Context, reqs apisec.Requirements) context.Context {
	return context.WithValue(ctx, requirementsKey{}, reqs)
}

// RequirementsFrom returns the requirements attached to ctx. ok is false when
// none were attached, meaning the request never passed through the auth
// middleware; that must read as "not authorised" rather than "no requirements",
// because an empty requirement set means a public operation.
func RequirementsFrom(ctx context.Context) (apisec.Requirements, bool) {
	reqs, ok := ctx.Value(requirementsKey{}).(apisec.Requirements)
	return reqs, ok
}

// PlatformBearerScopes reports the scopes the operation demands of the platform
// bearer scheme, and whether the operation accepts it at all.
func PlatformBearerScopes(ctx context.Context) ([]string, bool) {
	return scopesFor(ctx, SchemePlatformBearerAuth)
}

// ProductAPIKeyScopes reports the scopes the operation demands of the product
// API key scheme, and whether the operation accepts it at all.
func ProductAPIKeyScopes(ctx context.Context) ([]string, bool) {
	return scopesFor(ctx, SchemeProductAPIKeyAuth)
}

func scopesFor(ctx context.Context, scheme string) ([]string, bool) {
	reqs, ok := RequirementsFrom(ctx)
	if !ok {
		return nil, false
	}
	return reqs.ScopesFor(scheme)
}
