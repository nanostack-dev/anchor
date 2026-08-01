package middleware_test

import (
	"context"
	"encoding/json"
	"testing"

	"anchor/internal/middleware"
	"anchor/internal/security"
	"anchor/internal/service"

	"github.com/nanostack-dev/nanostack-framework/modules/logging"
	"github.com/nanostack-dev/nanostack-framework/pkg/apisec"
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

	ctx := security.SetCurrentUserID(context.Background(), "usr_wired")
	ctx = security.SetTenantID(ctx, "tnt_wired")
	log.Ctx(binder.Bind(ctx)).Info().Msg("wired")

	line := sink.decode(t)
	if line["user_id"] != "usr_wired" {
		t.Errorf("user_id = %v — the security enricher is not reaching the Binder", line["user_id"])
	}
	if line["tenant_id"] != "tnt_wired" {
		t.Errorf("tenant_id = %v — the security enricher is not reaching the Binder", line["tenant_id"])
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
