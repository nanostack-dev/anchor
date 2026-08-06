// Package security exposes the security requirements of the operation a request
// addresses, named in this application's terms.
//
// The context plumbing lives in the framework's pkg/apisec; this file only maps
// the scheme names this contract declares onto it, so the two applications that
// share the framework do not each carry a copy of the same context key.
package security

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
)

// Scheme names as declared under `components.securitySchemes` in
// cmd/http/openapi.yaml. They join the contract to the code enforcing it, so
// they are spelled once, here.
const (
	SchemePlatformBearerAuth = "platformBearerAuth"
	SchemeProductAPIKeyAuth  = "productApiKeyAuth"
)

// PlatformBearerScopes reports the scopes the operation demands of the platform
// bearer scheme, and whether the operation accepts it at all. It is false for a
// request that never passed through the auth middleware, so an unresolved
// request never looks permitted.
func PlatformBearerScopes(ctx context.Context) ([]string, bool) {
	return apisec.ScopesFromContext(ctx, SchemePlatformBearerAuth)
}

// ProductAPIKeyScopes reports the scopes the operation demands of the product
// API key scheme, and whether the operation accepts it at all.
func ProductAPIKeyScopes(ctx context.Context) ([]string, bool) {
	return apisec.ScopesFromContext(ctx, SchemeProductAPIKeyAuth)
}
