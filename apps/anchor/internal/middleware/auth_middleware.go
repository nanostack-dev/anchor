package middleware

import (
	"errors"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"github.com/rs/zerolog"

	"anchor/internal/domain/product"
	"anchor/internal/service"

	"github.com/go-chi/chi/v5"

	"anchor/internal/domain/product/apikey"

	"anchor/internal/security"
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

			if auth.handleProductAPIKeyAuth(w, r, next) {
				return
			}

			if auth.handlePlatformBearerAuth(w, r, next) {
				return
			}

			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		},
	)
}

func (auth *AuthMiddleware) handleProductAPIKeyAuth(
	w http.ResponseWriter, r *http.Request, next http.Handler,
) bool {
	values, allowed := security.ProductAPIKeyScopes(r.Context())
	if !allowed {
		return false
	}

	apiKeyValue := r.Header.Get(ProductAPIKeyHeader)
	if apiKeyValue == "" {
		return false
	}

	productIDPath := chi.URLParam(r, "product_id")
	if len(values) == 0 {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return true
	}

	_, err := auth.productAPIKeyKeyService.ValidateAPIKeyAndScopes(
		r.Context(), apikey.ValidateAPIKeyScopesInput{
			ProductID:   productIDPath,
			Scopes:      values,
			APIKeyValue: apiKeyValue,
		},
	)
	if err != nil {
		if frameworkErr, hasFrameworkErr := fault.As(err); hasFrameworkErr {
			writeAPIError(w, frameworkErr)
		} else {
			writeAPIError(w, fault.ErrUnexpected)
		}
		return true
	}
	prod, err := auth.productService.GetInternal(r.Context(), productIDPath)
	if err != nil {
		auth.logger.Error().Err(err).Str("product_id", productIDPath).
			Msg("failed to resolve product tenant for API key auth")
		http.Error(w, "Unexpected Error", http.StatusInternalServerError)
		return true
	}
	if prod == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return true
	}

	ctx := security.SetTenantID(r.Context(), prod.PlatformTenantID)
	ctx = security.SetProductScope(ctx, security.ProductScope{ProductID: productIDPath})
	next.ServeHTTP(w, r.WithContext(ctx))
	return true
}

func (auth *AuthMiddleware) handlePlatformBearerAuth(
	w http.ResponseWriter, r *http.Request, next http.Handler,
) bool {
	if _, allowed := security.PlatformBearerScopes(r.Context()); !allowed {
		return false
	}

	token, err := auth.extractAndValidateToken(w, r)
	if err != nil {
		return true
	}

	if accessErr := auth.validateProductAccess(w, r, token); accessErr != nil {
		return true
	}

	ctx := security.SetCurrentUserID(r.Context(), token.UserID)
	ctx = security.SetTenantID(ctx, token.TenantID)
	if productIDPath := chi.URLParam(r, "product_id"); productIDPath != "" {
		ctx = security.SetProductScope(ctx, security.ProductScope{ProductID: productIDPath})
	}
	next.ServeHTTP(w, r.WithContext(ctx))
	return true
}

func (auth *AuthMiddleware) extractAndValidateToken(
	w http.ResponseWriter, r *http.Request,
) (*service.AuthClaims, error) {
	bearer := r.Header.Get("Authorization")
	if bearer == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, errors.New("missing authorization header")
	}

	bearerLen := len("Bearer ")
	if len(bearer) < bearerLen {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, errors.New("invalid bearer format")
	}

	token, err := auth.jwtHelper.ValidateAccessToken(bearer[bearerLen:])
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return nil, err
	}

	return token, nil
}

func (auth *AuthMiddleware) validateProductAccess(
	w http.ResponseWriter, r *http.Request, token *service.AuthClaims,
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
		http.Error(w, "Unexpected Error", http.StatusInternalServerError)
		return err
	}

	if find == nil {
		auth.logger.Debug().Str("tenant_id", token.TenantID).Str("product_id", productIDPath).
			Msg("Product not found for tenant")
		http.Error(w, "Product not found", http.StatusNotFound)
		return errors.New("product not found")
	}

	return nil
}

func writeAPIError(w http.ResponseWriter, err *fault.Error) {
	fault.WriteJSON(w, err)
}
