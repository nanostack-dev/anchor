package middleware_test

import (
	"encoding/json"
	"testing"

	"net/http"
	"net/http/httptest"

	"anchor/internal/middleware"
	"anchor/internal/security"
	"anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/requestlog"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

// The enricher registrations in NewModule are the single point of failure for
// request-scoped logging: an fx value group accepts being empty, so deleting
// either fx.Annotate leaves the app building and starting normally while every
// log line silently loses the caller's identity. Nothing else would notice.
//
// This builds the real module through fx and asserts the Binder it produces
// actually publishes identity, so a deleted registration fails the build.
func TestModuleRegistersLogEnrichers(t *testing.T) {
	var sink capture
	var binder *log.Binder

	app := fx.New(
		fx.Supply(logging.Config{Level: zerolog.DebugLevel}),
		fx.Provide(func() zerolog.Logger { return zerolog.New(&sink).Level(zerolog.DebugLevel) }),
		fx.Provide(logging.NewBinder),
		middleware.NewModule(),
		// AuthMiddleware is not under test; it only has to be constructible for
		// the module that carries the enrichers to be usable.
		fx.Provide(
			func() service.JWTHelper { return nil },
			func() service.ProductAPIKeyService { return nil },
			func() service.ProductService { return nil },
			func() *apisec.Resolver { return nil },
		),
		fx.Populate(&binder),
		fx.NopLogger,
	)
	if err := app.Err(); err != nil {
		t.Fatalf("building the middleware module failed: %v", err)
	}

	// Both registered enrichers must be asserted. Checking only the security
	// fields would leave the requestlog registration unpinned: deleting it keeps
	// this green while production silently loses request_id from every bound
	// line. So the Bind runs inside a real Contextualize.
	var line map[string]any
	handler := requestlog.Contextualize(zerolog.New(&sink))(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			ctx := security.SetCurrentUserID(r.Context(), "usr_wired")
			ctx = security.SetTenantID(ctx, "tnt_wired")
			log.Ctx(binder.Bind(ctx)).Info().Msg("wired")
			line = sink.decode(t)
		},
	))

	request := httptest.NewRequest(http.MethodGet, "/tenants/abc", nil)
	request.Header.Set(requestlog.RequestIDHeader, "req_wired")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	for key, want := range map[string]string{
		"user_id":    "usr_wired", // security enricher
		"tenant_id":  "tnt_wired", // security enricher
		"request_id": "req_wired", // requestlog enricher
		"path":       "/tenants/abc",
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v — an enricher is not reaching the Binder", key, line[key], want)
		}
	}
}

type capture struct{ last []byte }

func (c *capture) Write(p []byte) (int, error) {
	c.last = append(c.last[:0], p...)
	return len(p), nil
}

func (c *capture) decode(t *testing.T) map[string]any {
	t.Helper()
	if len(c.last) == 0 {
		t.Fatal("no log line was written")
	}
	var line map[string]any
	if err := json.Unmarshal(c.last, &line); err != nil {
		t.Fatalf("decode log line %q: %v", c.last, err)
	}
	return line
}
