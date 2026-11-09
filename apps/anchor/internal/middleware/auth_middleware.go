package middleware

import (
	"log/slog"
	"net/http"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/applog"
	"anchor/internal/domain/product"
	"anchor/internal/repository"
	"anchor/internal/security"
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
	members                 repository.PlatformTenantUserRepository
	logger                  *slog.Logger
	// identity is the service identity, added to the request scope with the
	// caller once auth succeeds.
	identity applog.Identity
	// resolver reads each operation's security requirements from the OpenAPI
	// document, replacing oapi-codegen's generated `<Scheme>Scopes` context
	// keys, which flattened them and could not represent alternative or
	// combined schemes (oapi-codegen#1524).
	resolver *apisec.Resolver
}

// NewAuthMiddleware creates a new AuthMiddleware instance.
func NewAuthMiddleware(
	jwtHelper service.JWTHelper, productAPIKeyKeyService service.ProductAPIKeyService,
	productService service.ProductService, members repository.PlatformTenantUserRepository, logger *slog.Logger,
	identity applog.Identity, resolver *apisec.Resolver,
) *AuthMiddleware {
	return &AuthMiddleware{
		jwtHelper:               jwtHelper,
		productAPIKeyKeyService: productAPIKeyKeyService,
		productService:          productService,
		members:                 members,
		logger:                  logger.With(slog.String("component", "auth_middleware")),
		identity:                identity,
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
				auth.logger.WarnContext(r.Context(), "no OpenAPI operation matched; refusing the request",
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				writeAPIError(w, fault.ErrUnauthorized)
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

			// Add the caller and the service identity to the request scope, so
			// every later line of the request, the summary line included,
			// carries them. A refused request and a public route get neither.
			log.SetAttrs(authorized, append(slices.Clone(auth.identity), security.LogAttrs(authorized)...)...)

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
	// A deleted platform user keeps a signed token until it expires; the token must stop with the user.
	member, err := auth.members.FindByPlatformTenantIDAndUserID(r.Context(), token.TenantID, token.UserID)
	if err != nil {
		return nil, &schemeError{fault: fault.ErrUnexpected, reason: "platform user lookup failed"}
	}
	if member.IsAbsent() {
		return nil, unauthorized("platform user no longer exists")
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
		auth.logger.WarnContext(r.Context(), "Failed to find product for tenant",
			log.Err(err),
			slog.String("tenant_id", token.TenantID),
			slog.String("product_id", productIDPath),
		)
		return &schemeError{fault: fault.ErrUnexpected, reason: "product lookup failed"}
	}

	if find == nil {
		auth.logger.DebugContext(r.Context(), "Product not found for tenant",
			slog.String("tenant_id", token.TenantID),
			slog.String("product_id", productIDPath),
		)
		return &schemeError{fault: product.ErrProductNotFound, reason: "product not found for tenant"}
	}

	return nil
}

func writeAPIError(w http.ResponseWriter, err *fault.Error) {
	fault.WriteJSON(w, err)
}
