package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"anchor/internal/authz/fgasync"

	"anchor/internal/api"
	"anchor/internal/applog"
	"anchor/internal/buildinfo"
	"anchor/internal/middleware"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	sharedhealth "github.com/nanostack-dev/nanostack-framework/pkg/health"
	"github.com/nanostack-dev/nanostack-framework/pkg/tracing"
	"github.com/nanostack-dev/pgkit/adminui"
	"github.com/nanostack-dev/pgkit/queue"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import the cors middleware
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

const (
	corsMaxAge = 300 // 5 minutes
	// dashboardShutdownTimeout bounds the drain of the pgkit admin UI.
	dashboardShutdownTimeout = 5 * time.Second
)

// handlerDeps is what the router and the strict handler take.
type handlerDeps struct {
	logger        *slog.Logger
	scopedLogger  applog.Scoped
	api           *api.AnchorAPI
	auth          *middleware.AuthMiddleware
	allowedOrigin string
	replayer      *fgasync.Syncer
}

// NewHandler builds anchor's HTTP handler: the chi router with CORS, the
// request scope, the OpenAPI validator, the docs and /health routes, then the
// strict handler with the request summary and auth, all inside the server
// span. nanoserver serves it.
func NewHandler(
	ctx context.Context,
	cfg Config,
	logger *slog.Logger,
	scopedLogger applog.Scoped,
	anchorAPI *api.AnchorAPI,
	auth *middleware.AuthMiddleware,
	replayer *fgasync.Syncer,
) (http.Handler, error) {
	deps := handlerDeps{
		logger: logger, scopedLogger: scopedLogger, api: anchorAPI, auth: auth,
		allowedOrigin: cfg.AllowedOrigin, replayer: replayer,
	}
	logger.InfoContext(ctx, "Starting HTTP server",
		slog.Int("port", cfg.Port),
		slog.String("allowed_origin", cfg.AllowedOrigin),
	)

	router, err := setupRouter(deps)
	if err != nil {
		return nil, err
	}
	logRoutes(ctx, router, logger)

	errorMiddleware := middleware.NewErrorMiddleware(scopedLogger)
	// The server span wraps everything, continuing an incoming W3C
	// traceparent, so the request logger and DB queries join it.
	return tracing.Middleware()(createHandler(
		router, anchorAPI, errorMiddleware, replayer, []api.MiddlewareFunc{
			middleware.NewRequestLoggingMiddleware(scopedLogger),
			auth.Create,
		},
	)), nil
}

// QueueDashboard serves pgkit's embedded admin UI on PGKIT_DASHBOARD_ADDR
// while the App runs. Anchor registers no pgkit workflows, so the UI serves
// queue views only. Without the address it serves nothing.
type QueueDashboard struct {
	ui     *adminui.UI
	addr   string
	logger *slog.Logger
}

// NewQueueDashboard builds the admin UI when PGKIT_DASHBOARD_ADDR is set.
// pgkit reads its own settings (PGKIT_DASHBOARD_*) from the environment.
func NewQueueDashboard(queueClient *queue.Client, logger *slog.Logger) (*QueueDashboard, error) {
	dashboard := &QueueDashboard{
		addr:   os.Getenv("PGKIT_DASHBOARD_ADDR"),
		logger: logger.With(slog.String(log.KeyComponent, "pgkit_adminui")),
	}
	if dashboard.addr == "" {
		return dashboard, nil
	}
	if strings.TrimSpace(os.Getenv("PGKIT_DASHBOARD_TOKEN")) == "" {
		return nil, errors.New("PGKIT_DASHBOARD_TOKEN is required when PGKIT_DASHBOARD_ADDR is set")
	}
	ui, err := adminui.NewFromEnv(queueClient, nil)
	if err != nil {
		return nil, fmt.Errorf("initialize pgkit admin UI: %w", err)
	}
	dashboard.ui = ui
	return dashboard, nil
}

// Run serves the admin UI until the App stops. A listen failure is logged
// and leaves the API serving: the UI is an operator tool, not a dependency.
func (d *QueueDashboard) Run(ctx context.Context) error {
	if d.ui == nil {
		<-ctx.Done()
		return nil
	}
	go func() {
		if serveErr := d.ui.ListenAndServe(d.addr); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			d.logger.ErrorContext(ctx, "pgkit admin UI server failed", log.Err(serveErr))
		}
	}()
	d.logger.InfoContext(ctx, "pgkit admin UI started", slog.String("addr", d.addr))

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), dashboardShutdownTimeout)
	defer cancel()
	if err := d.ui.Shutdown(shutdownCtx); err != nil {
		d.logger.ErrorContext(ctx, "pgkit admin UI shutdown failed", log.Err(err))
	}
	return nil
}

func setupRouter(params handlerDeps) (*chi.Mux, error) {
	router := chi.NewRouter()
	// Names the server span after the matched chi route; it must be the first
	// router middleware so it reads the final pattern after serving.
	router.Use(tracing.RouteMiddleware)

	corsMiddleware := cors.New(
		cors.Options{
			// Origins come entirely from config. Entries may be exact
			// (https://app.tryanchor.dev) or wildcard (https://*.tryanchor.dev);
			// go-chi/cors matches wildcards and reflects the matched origin so
			// AllowCredentials works. Prod lists exact origins; dev/preview add a
			// wildcard. No domain literals in source.
			AllowedOrigins: parseAllowedOrigins(params.allowedOrigin),
			AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders: []string{
				"Accept",
				"Authorization",
				"Content-Type",
				"X-CSRF-Token",
				"Baggage",
				"Traceparent",
				"Tracestate",
				"X-Request-Id",
				"X-Client-Version",
			},
			ExposedHeaders:   []string{"Link", "Deprecation"},
			AllowCredentials: true,
			MaxAge:           corsMaxAge,
		},
	)
	router.Use(corsMiddleware.Handler)
	router.Use(api.WebhookPayloadMiddleware)

	// Establishes the per-request correlation id (echoed in X-Request-Id) and
	// the log scope (request_id, method, path) every later stage builds on. It
	// has to run before the request summary and before auth adds the caller
	// to the scope. Anchor writes its own summary line inside auth
	// (middleware.NewRequestLoggingMiddleware), so this one writes none; it
	// still logs a handler panic.
	router.Use(log.HTTPMiddleware(log.HTTPOptions{
		Logger:      params.scopedLogger.Logger,
		SkipSummary: func(*http.Request) bool { return true },
	}))

	swagger, err := openapi3.NewLoader().LoadFromData(OpenAPI)
	if err != nil {
		return nil, fmt.Errorf("load OpenAPI specification: %w", err)
	}

	apiValidator := nethttpmiddleware.OapiRequestValidatorWithOptions(
		swagger, &nethttpmiddleware.Options{
			Options: openapi3filter.Options{
				ExcludeRequestBody:    true,
				ExcludeResponseBody:   true,
				IncludeResponseStatus: true,
				MultiError:            true,
				AuthenticationFunc: func(
					_ context.Context, _ *openapi3filter.AuthenticationInput,
				) error {
					return nil
				},
			},
			SilenceServersWarning: false,
		},
	)

	// Custom middleware to skip OpenAPI validation for docs routes
	router.Use(
		func(next http.Handler) http.Handler {
			return http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					// Skip OpenAPI validation for documentation and mounted utility routes.
					if r.URL.Path == "/docs" || r.URL.Path == "/openapi.yaml" || r.URL.Path == "/health" {
						next.ServeHTTP(w, r)
						return
					}
					// Apply OpenAPI validation for all other routes
					apiValidator(next).ServeHTTP(w, r)
				},
			)
		},
	)

	// Register documentation routes after all middlewares
	params.api.RegisterDocsRoutes(router, OpenAPI)
	router.Get(sharedhealth.DefaultPath, func(w http.ResponseWriter, r *http.Request) {
		isTenantInit, isTenantInitErr := params.api.TenantService.IsTenantInit(r.Context())
		if isTenantInitErr != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		sharedhealth.NewHandler(sharedhealth.Config{
			Service:   "anchor",
			Version:   buildinfo.Version,
			CommitSHA: buildinfo.CommitSHA,
			BuildDate: buildinfo.BuildDate,
			Extra: map[string]any{
				"tenant_initialized": isTenantInit,
			},
		}).ServeHTTP(w, r)
	})

	return router, nil
}

// parseAllowedOrigins splits a comma-separated origin list, trimming blanks.
// Entries may include go-chi/cors wildcards (e.g. https://*.tryanchor.dev).
func parseAllowedOrigins(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	return functional.Slice(parts).FilterMap(
		func(p string) bool { return strings.TrimSpace(p) != "" },
		strings.TrimSpace,
	)
}

func logRoutes(ctx context.Context, router *chi.Mux, logger *slog.Logger) {
	walkFunc := func(
		method, route string, _ http.Handler,
		_ ...func(http.Handler) http.Handler,
	) error {
		logger.InfoContext(ctx, fmt.Sprintf("[%s] %s", method, route))
		return nil
	}
	if err := chi.Walk(router, walkFunc); err != nil {
		logger.ErrorContext(ctx, "Failed to walk routes", log.Err(err))
	}
}

func createHandler(
	router *chi.Mux, anchorAPI *api.AnchorAPI,
	errorMiddleware *middleware.ErrorMiddleware, replayer api.AuthzReplayer,
	middlewares []api.MiddlewareFunc,
) http.Handler {
	return api.HandlerWithOptions(
		api.NewStrictHandlerWithOptions(
			anchorAPI, []api.StrictMiddlewareFunc{
				api.NewWebhookHeadersMiddleware(),
				api.NewLogoutAccessTokenMiddleware(),
				api.NewAuthzReplayMiddleware(replayer),
			},
			api.StrictHTTPServerOptions{
				RequestErrorHandlerFunc:  errorMiddleware.HandleRequestError,
				ResponseErrorHandlerFunc: errorMiddleware.HandleResponseError,
			},
		),
		api.ChiServerOptions{
			BaseRouter:  router,
			Middlewares: middlewares,
		},
	)
}
