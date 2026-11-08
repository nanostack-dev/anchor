package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
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
	"go.uber.org/fx"
)

const (
	corsMaxAge        = 300 // 5 minutes
	readHeaderTimeout = 30  // seconds
	readTimeout       = 60  // seconds
	writeTimeout      = 60  // seconds
	idleTimeout       = 120 // seconds
	shutdownTimeout   = 5   // seconds
)

type ServerParams struct {
	fx.In

	Lifecycle      fx.Lifecycle
	Logger         *slog.Logger
	ScopedLogger   applog.Scoped
	API            *api.AnchorAPI
	Queue          *queue.Client
	AuthMiddleware *middleware.AuthMiddleware
	ServerConfig   *ServerConfig
	AuthzReplayer  *fgasync.Syncer
}

func RegisterServer(params ServerParams) {
	var dashboard *adminui.UI

	params.Lifecycle.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				if err := startHTTPServer(ctx, params); err != nil {
					return err
				}

				var dashErr error
				dashboard, dashErr = startQueueDashboard(ctx, params)
				return dashErr
			},
			OnStop: func(ctx context.Context) error {
				params.Logger.InfoContext(ctx, "Stopping HTTP server")
				if dashboard != nil {
					shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout*time.Second)
					defer cancel()
					if err := dashboard.Shutdown(shutdownCtx); err != nil {
						params.Logger.ErrorContext(ctx, "pgkit admin UI shutdown failed", log.Err(err))
					}
				}
				return nil
			},
		},
	)
}

// startQueueDashboard boots pgkit's embedded admin UI. Anchor registers no
// pgkit workflows, so the workflow module is nil and the UI serves queue views
// only.
func startQueueDashboard(ctx context.Context, params ServerParams) (*adminui.UI, error) {
	logger := params.Logger.With(slog.String(log.KeyComponent, "pgkit_adminui"))
	addr := os.Getenv("PGKIT_DASHBOARD_ADDR")
	if addr == "" {
		return nil, nil
	}
	if strings.TrimSpace(os.Getenv("PGKIT_DASHBOARD_TOKEN")) == "" {
		return nil, errors.New("PGKIT_DASHBOARD_TOKEN is required when PGKIT_DASHBOARD_ADDR is set")
	}

	dashboard, err := adminui.NewFromEnv(params.Queue, nil)
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize pgkit admin UI", log.Err(err))
		return nil, err
	}

	go func() {
		if serveErr := dashboard.ListenAndServe(addr); serveErr != nil {
			logger.ErrorContext(ctx, "pgkit admin UI server failed", log.Err(serveErr))
		}
	}()

	logger.InfoContext(ctx, "pgkit admin UI started", slog.String("addr", addr))

	return dashboard, nil
}

func startHTTPServer(ctx context.Context, params ServerParams) error {
	params.Logger.InfoContext(ctx, "Starting HTTP server",
		slog.Int("port", params.ServerConfig.Port),
		slog.String("allowed_origin", params.ServerConfig.AllowedOrigin),
	)

	router, err := setupRouter(params)
	if err != nil {
		return err
	}
	server := createHTTPServer(params, router)

	logRoutes(ctx, router, params.Logger)
	startServerGoroutines(ctx, params, server)

	return nil
}

func setupRouter(params ServerParams) (*chi.Mux, error) {
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
			AllowedOrigins: parseAllowedOrigins(params.ServerConfig.AllowedOrigin),
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
		Logger:      params.ScopedLogger.Logger,
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
	params.API.RegisterDocsRoutes(router, OpenAPI)
	router.Get(sharedhealth.DefaultPath, func(w http.ResponseWriter, r *http.Request) {
		isTenantInit, isTenantInitErr := params.API.TenantService.IsTenantInit(r.Context())
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

func createHTTPServer(params ServerParams, router *chi.Mux) *http.Server {
	authMiddleware := params.AuthMiddleware
	errorMiddleware := middleware.NewErrorMiddleware(params.ScopedLogger)

	return &http.Server{
		Addr: ":" + strconv.Itoa(params.ServerConfig.Port),
		// The server span wraps everything, continuing an incoming W3C
		// traceparent, so the request logger and DB queries join it.
		Handler: tracing.Middleware()(createHandler(
			router, params.API, errorMiddleware, params.AuthzReplayer, []api.MiddlewareFunc{
				middleware.NewRequestLoggingMiddleware(params.ScopedLogger),
				authMiddleware.Create,
			},
		)),
		ReadHeaderTimeout: readHeaderTimeout * time.Second,
		ReadTimeout:       readTimeout * time.Second,
		WriteTimeout:      writeTimeout * time.Second,
		IdleTimeout:       idleTimeout * time.Second,
	}
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

func startServerGoroutines(ctx context.Context, params ServerParams, server *http.Server) {
	port := params.ServerConfig.Port
	// The goroutines outlive the OnStart hook whose context ctx is.
	ctx = context.WithoutCancel(ctx)

	go func() {
		params.Logger.InfoContext(ctx, "Server running", slog.Int("port", port))
		if err := server.ListenAndServe(); err != nil && !errors.Is(
			err, http.ErrServerClosed,
		) {
			// slog has no fatal level: log at Error and exit, as zerolog's
			// Fatal did.
			params.Logger.ErrorContext(ctx, "HTTP server encountered an error", log.Err(err))
			os.Exit(1)
		}
	}()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		params.Logger.InfoContext(ctx, "Shutting down server...")
		ctxShutdown, cancel := context.WithTimeout(ctx, shutdownTimeout*time.Second)
		defer cancel()

		if err := server.Shutdown(ctxShutdown); err != nil {
			params.Logger.ErrorContext(ctx, "Forced shutdown", log.Err(err))
		}

		params.Logger.InfoContext(ctx, "Server exited cleanly")
	}()
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
