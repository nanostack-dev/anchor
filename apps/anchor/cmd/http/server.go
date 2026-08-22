package httpserver

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"anchor/internal/api"
	"anchor/internal/buildinfo"
	"anchor/internal/middleware"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	sharedhealth "github.com/nanostack-dev/nanostack-framework/pkg/health"
	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/requestlog"
	"github.com/nanostack-dev/pgkit/adminui"
	"github.com/nanostack-dev/pgkit/queue"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors" // Import the cors middleware
	nethttpmiddleware "github.com/oapi-codegen/nethttp-middleware"
	"github.com/rs/zerolog"
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
	Logger         zerolog.Logger
	API            *api.AnchorAPI
	Queue          *queue.Client
	AuthMiddleware *middleware.AuthMiddleware
	ServerConfig   *ServerConfig
}

func RegisterServer(params ServerParams) {
	var dashboard *adminui.UI

	params.Lifecycle.Append(
		fx.Hook{
			OnStart: func(_ context.Context) error {
				if err := startHTTPServer(params); err != nil {
					return err
				}

				var dashErr error
				dashboard, dashErr = startQueueDashboard(params)
				return dashErr
			},
			OnStop: func(_ context.Context) error {
				params.Logger.Info().Msg("Stopping HTTP server")
				if dashboard != nil {
					shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout*time.Second)
					defer cancel()
					if err := dashboard.Shutdown(shutdownCtx); err != nil {
						params.Logger.Error().Err(err).Msg("pgkit admin UI shutdown failed")
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
func startQueueDashboard(params ServerParams) (*adminui.UI, error) {
	logger := params.Logger.With().Str("component", "pgkit_adminui").Logger()
	addr := os.Getenv("PGKIT_DASHBOARD_ADDR")
	if addr == "" {
		return nil, nil
	}
	if strings.TrimSpace(os.Getenv("PGKIT_DASHBOARD_TOKEN")) == "" {
		return nil, errors.New("PGKIT_DASHBOARD_TOKEN is required when PGKIT_DASHBOARD_ADDR is set")
	}

	dashboard, err := adminui.NewFromEnv(params.Queue, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize pgkit admin UI")
		return nil, err
	}

	go func() {
		if serveErr := dashboard.ListenAndServe(addr); serveErr != nil {
			logger.Error().Err(serveErr).Msg("pgkit admin UI server failed")
		}
	}()

	logger.Info().
		Str("addr", addr).
		Msg("pgkit admin UI started")

	return dashboard, nil
}

func startHTTPServer(params ServerParams) error {
	params.Logger.Info().Msgf(
		"Starting HTTP server on port %d with allowed origin %s",
		params.ServerConfig.Port, params.ServerConfig.AllowedOrigin,
	)

	router := setupRouter(params)
	server := createHTTPServer(params, router)

	logRoutes(router, params.Logger)
	startServerGoroutines(params, server)

	return nil
}

func setupRouter(params ServerParams) *chi.Mux {
	router := chi.NewRouter()

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
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           corsMaxAge,
		},
	)
	router.Use(corsMiddleware.Handler)

	// Establishes the per-request correlation id and the request-scoped logger
	// every later stage builds on. It has to run before the access log and
	// before auth binds the caller onto the logger.
	router.Use(requestlog.Contextualize(params.Logger))

	swagger, err := openapi3.NewLoader().LoadFromData(OpenAPI)
	if err != nil {
		params.Logger.Fatal().Err(err).Msg("Failed to load OpenAPI specification")
		return nil
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

	return router
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
	errorMiddleware := middleware.NewErrorMiddleware(params.Logger)

	return &http.Server{
		Addr: ":" + strconv.Itoa(params.ServerConfig.Port),
		Handler: createHandler(
			router, params.API, errorMiddleware, []api.MiddlewareFunc{
				middleware.NewRequestLoggingMiddleware(params.Logger),
				authMiddleware.Create,
			},
		),
		ReadHeaderTimeout: readHeaderTimeout * time.Second,
		ReadTimeout:       readTimeout * time.Second,
		WriteTimeout:      writeTimeout * time.Second,
		IdleTimeout:       idleTimeout * time.Second,
	}
}

func logRoutes(router *chi.Mux, logger zerolog.Logger) {
	walkFunc := func(
		method, route string, _ http.Handler,
		_ ...func(http.Handler) http.Handler,
	) error {
		logger.Info().Msgf("[%s] %s", method, route)
		return nil
	}
	if err := chi.Walk(router, walkFunc); err != nil {
		logger.Error().Err(err).Msg("Failed to walk routes")
	}
}

func startServerGoroutines(params ServerParams, server *http.Server) {
	port := params.ServerConfig.Port

	go func() {
		params.Logger.Info().Msgf("Server running on port %d", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(
			err, http.ErrServerClosed,
		) {
			params.Logger.Fatal().Err(err).Msg("HTTP server encountered an error")
		}
	}()

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		params.Logger.Info().Msg("Shutting down server...")
		ctxShutdown, cancel := context.WithTimeout(
			context.Background(), shutdownTimeout*time.Second,
		)
		defer cancel()

		if err := server.Shutdown(ctxShutdown); err != nil {
			params.Logger.Error().Err(err).Msg("Forced shutdown")
		}

		params.Logger.Info().Msg("Server exited cleanly")
	}()
}

func createHandler(
	router *chi.Mux, anchorAPI *api.AnchorAPI,
	errorMiddleware *middleware.ErrorMiddleware, middlewares []api.MiddlewareFunc,
) http.Handler {
	return api.HandlerWithOptions(
		api.NewStrictHandlerWithOptions(
			anchorAPI, []api.StrictMiddlewareFunc{
				api.NewWebhookHeadersMiddleware(),
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
