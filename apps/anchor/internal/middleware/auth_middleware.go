package middleware

import (
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"github.com/rs/zerolog"

	"anchor/internal/domain/product"
	"anchor/internal/service"

	"github.com/go-chi/chi/v5"
)

const (
	ProductAPIKeyHeader = "X-Product-Api-Key" //nolint:gosec // This is a header name, not credentials
)

type AuthMiddleware struct {
	jwtHelper               service.JWTHelper
	productAPIKeyKeyService service.ProductAPIKeyService
	productService          service.ProductService
	logger                  zerolog.Logger
	// resolver reads each operation's security requirements from the OpenAPI
	// document, replacing oapi-codegen's generated `<Scheme>Scopes` context
	// keys, which flattened them and could not represent alternative or
	// combined schemes (oapi-codegen#1524).
	resolver *apisec.Resolver
}

// NewAuthMiddleware creates a new AuthMiddleware instance.
func NewAuthMiddleware(
	jwtHelper service.JWTHelper, productAPIKeyKeyService service.ProductAPIKeyService,
	productService service.ProductService, logger zerolog.Logger,
	resolver *apisec.Resolver,
) *AuthMiddleware {
	return &AuthMiddleware{
		jwtHelper:               jwtHelper,
		productAPIKeyKeyService: productAPIKeyKeyService,
		productService:          productService,
		logger:                  logger.With().Str("component", "auth_middleware").Logger(),
		resolver:                resolver,
	}
}

func (auth *AuthMiddleware) Create(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Attach the operation's security requirements before attempting
			// any authentication; every decision below reads them. A request
			// matching no documented operation is refused rather than treated
			// as unrestricted — an unresolvable route must never fall through
			// with an empty requirement set, which would read as "public".
			requirements, resolved := auth.resolver.For(r)
			if !resolved {
				auth.logger.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Msg("no OpenAPI operation matched; refusing the request")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			r = r.WithContext(apisec.WithRequirements(r.Context(), requirements))

			if requirements.Public() {
				next.ServeHTTP(w, r)
				return
			}

			// The disjunction between product API key and platform bearer auth
			// comes from the contract: Evaluate tries each alternative the
			// document declares and returns the context the satisfied one
			// produced.
			authorized, err := requirements.Evaluate(r.Context(), r, auth.authenticateScheme)
			if err != nil {
				auth.renderAuthFailure(w, err)
				return
			}

			next.ServeHTTP(w, r.WithContext(authorized))
		},
	)
}

// validateAccessToken parses and verifies the platform access token. It reports
// failure as an error rather than writing a response, because under a
// disjunction a later alternative may still authorise the request.
func (auth *AuthMiddleware) validateAccessToken(r *http.Request) (*service.AuthClaims, error) {
	const bearerPrefix = "Bearer "

	bearer := r.Header.Get("Authorization")
	if len(bearer) < len(bearerPrefix) {
		return nil, unauthorized("invalid bearer format")
	}

	token, err := auth.jwtHelper.ValidateAccessToken(bearer[len(bearerPrefix):])
	if err != nil {
		return nil, unauthorized("access token rejected")
	}
	return token, nil
}

// authorizeProductAccess checks the caller's tenant owns the product named in
// the path, when the route names one.
func (auth *AuthMiddleware) authorizeProductAccess(
	r *http.Request, token *service.AuthClaims,
) error {
	productIDPath := chi.URLParam(r, "product_id")
	if productIDPath == "" {
		return nil
	}

	find, err := auth.productService.GetWithCache(
		r.Context(), product.GetProductInput{
			TenantID:  token.TenantID,
			ProductID: productIDPath,
		},
	)
	if err != nil {
		auth.logger.Warn().Err(err).Str("tenant_id", token.TenantID).Msgf(
			"Failed to find product %s for tenant %s", productIDPath, token.TenantID,
		)
		return &schemeError{
			status:  http.StatusInternalServerError,
			message: "Unexpected Error",
			reason:  "product lookup failed",
		}
	}

	if find == nil {
		auth.logger.Debug().Str("tenant_id", token.TenantID).Str("product_id", productIDPath).
			Msg("Product not found for tenant")
		return &schemeError{
			status:  http.StatusNotFound,
			message: "Product not found",
			reason:  "product not found for tenant",
		}
	}

	return nil
}

func writeAPIError(w http.ResponseWriter, err *fault.Error) {
	fault.WriteJSON(w, err)
}
