package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"

	"anchor/internal/domain/product/apikey"
	"anchor/internal/security"
)

// authenticateScheme verifies one security scheme named by the contract.
//
// It is the apisec.Authenticator the middleware hands to Requirements.Evaluate,
// so the disjunction between product API key and platform bearer auth is read
// from the document rather than reproduced here.
func (auth *AuthMiddleware) authenticateScheme(
	ctx context.Context, r *http.Request, scheme apisec.SchemeRequirement,
) (context.Context, error) {
	switch scheme.Name {
	case security.SchemeProductAPIKeyAuth:
		// Evaluate tries every alternative, so a scheme whose credential is
		// absent must say so cheaply — before any service call.
		if r.Header.Get(ProductAPIKeyHeader) == "" {
			return nil, apisec.ErrSchemeNotAttempted
		}
		return auth.authenticateProductAPIKey(ctx, r, scheme.Scopes)

	case security.SchemePlatformBearerAuth:
		if r.Header.Get("Authorization") == "" {
			return nil, apisec.ErrSchemeNotAttempted
		}
		return auth.authenticatePlatformBearer(ctx, r)

	default:
		// The contract names a scheme this code does not implement. Failing is
		// the safe reading: the alternative cannot be satisfied.
		return nil, fmt.Errorf("unsupported security scheme %q", scheme.Name)
	}
}

// authenticateProductAPIKey validates the product API key against the scopes the
// operation demands and returns the tenant and product scope it resolves.
func (auth *AuthMiddleware) authenticateProductAPIKey(
	ctx context.Context, r *http.Request, requiredScopes []string,
) (context.Context, error) {
	if len(requiredScopes) == 0 {
		return nil, unauthorized("operation declares no scopes for the product API key scheme")
	}

	productIDPath := chi.URLParam(r, "product_id")
	if _, err := auth.productAPIKeyKeyService.ValidateAPIKeyAndScopes(
		ctx, apikey.ValidateAPIKeyScopesInput{
			ProductID:   productIDPath,
			Scopes:      requiredScopes,
			APIKeyValue: r.Header.Get(ProductAPIKeyHeader),
		},
	); err != nil {
		if frameworkErr, ok := fault.As(err); ok {
			return nil, &schemeError{fault: frameworkErr, reason: "api key rejected"}
		}
		return nil, &schemeError{fault: fault.ErrUnexpected, reason: "api key validation failed"}
	}

	prod, err := auth.productService.GetInternal(ctx, productIDPath)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Str("product_id", productIDPath).
			Msg("failed to resolve product tenant for API key auth")
		return nil, &schemeError{
			status:  http.StatusInternalServerError,
			message: "Unexpected Error",
			reason:  "product lookup failed",
		}
	}
	if prod == nil {
		return nil, unauthorized("product not found for API key")
	}

	authorized := security.SetTenantID(ctx, prod.PlatformTenantID)
	return security.SetProductScope(
		authorized, security.ProductScope{ProductID: productIDPath},
	), nil
}

// authenticatePlatformBearer validates the platform access token and returns the
// caller identity and tenant it resolves.
func (auth *AuthMiddleware) authenticatePlatformBearer(
	ctx context.Context, r *http.Request,
) (context.Context, error) {
	token, err := auth.validateAccessToken(r)
	if err != nil {
		return nil, err
	}
	if accessErr := auth.authorizeProductAccess(r, token); accessErr != nil {
		return nil, accessErr
	}

	authorized := security.SetCurrentUserID(ctx, token.UserID)
	authorized = security.SetTenantID(authorized, token.TenantID)
	if productIDPath := chi.URLParam(r, "product_id"); productIDPath != "" {
		authorized = security.SetProductScope(
			authorized, security.ProductScope{ProductID: productIDPath},
		)
	}
	return authorized, nil
}

// renderAuthFailure writes the response for a failed evaluation.
//
// A scheme that examined the request described its own rejection; anything else
// means no credential the contract accepts was presented.
func (auth *AuthMiddleware) renderAuthFailure(w http.ResponseWriter, err error) {
	var failure *schemeError
	if !errors.As(err, &failure) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	if failure.fault != nil {
		writeAPIError(w, failure.fault)
		return
	}
	http.Error(w, failure.message, failure.status)
}

// schemeError describes how a scheme rejected the request, so the middleware can
// render it once evaluation gives up. Under a disjunction a scheme cannot write
// the response itself — a later alternative may still authorise the request.
type schemeError struct {
	status  int
	message string
	fault   *fault.Error
	reason  string
}

func (e *schemeError) Error() string { return e.reason }

func unauthorized(reason string) *schemeError {
	return &schemeError{
		status:  http.StatusUnauthorized,
		message: "Unauthorized",
		reason:  reason,
	}
}
