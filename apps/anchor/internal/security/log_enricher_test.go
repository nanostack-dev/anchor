package security_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/pkg/httputil/requestlog"
	"github.com/nanostack-dev/nanostack-framework/pkg/log"
	"github.com/rs/zerolog"
)

// The behaviour anchor did not have before: a line emitted during a request
// carries the correlation id and the authenticated caller without the call site
// naming either.
//
// This also pins the middleware order the design depends on — Contextualize
// must run before the Bind that follows authentication. A future reorder would
// otherwise regress silently, since nothing else asserts it.
func TestBoundRequestLineCarriesIdentity(t *testing.T) {
	var sink capture
	base := zerolog.New(&sink).Level(zerolog.DebugLevel)
	binder := log.NewBinder(base, requestlog.NewLogEnricher(), security.NewLogEnricher())

	handler := requestlog.Contextualize(base)(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			// Stands in for the auth middleware: resolve the caller, then bind.
			ctx := security.SetCurrentUserID(r.Context(), "usr_4mHt9pLz")
			ctx = security.SetTenantID(ctx, "tnt_92xQ")
			ctx = binder.Bind(ctx)

			log.Ctx(ctx).Info().Msg("workspace loaded")
		},
	))

	request := httptest.NewRequest(http.MethodGet, "/tenants/abc/workspaces", nil)
	request.Header.Set(requestlog.RequestIDHeader, "req_fixed")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	line := sink.decode(t)
	for key, want := range map[string]string{
		"request_id": "req_fixed",
		"method":     http.MethodGet,
		"path":       "/tenants/abc/workspaces",
		"user_id":    "usr_4mHt9pLz",
		"tenant_id":  "tnt_92xQ",
		"message":    "workspace loaded",
	} {
		if line[key] != want {
			t.Errorf("%s = %v, want %v", key, line[key], want)
		}
	}
}

// An unauthenticated request is normal, not an error: the enricher must add
// nothing rather than emitting empty fields or failing the way
// GetCurrentUserID/GetTenantID would.
func TestEnricherAddsNothingWithoutACaller(t *testing.T) {
	var sink capture
	base := zerolog.New(&sink).Level(zerolog.DebugLevel)

	ctx := log.NewBinder(base, security.NewLogEnricher()).Bind(context.Background())
	log.Ctx(ctx).Info().Msg("anonymous")

	line := sink.decode(t)
	for _, key := range []string{"user_id", "tenant_id"} {
		if _, ok := line[key]; ok {
			t.Errorf("expected no %q for an unauthenticated context, got %v", key, line[key])
		}
	}
}

// Product API key auth sets a tenant but no user; the enricher must publish the
// half that exists.
func TestEnricherHandlesTenantWithoutUser(t *testing.T) {
	var sink capture
	base := zerolog.New(&sink).Level(zerolog.DebugLevel)

	ctx := security.SetTenantID(context.Background(), "tnt_platform")
	ctx = log.NewBinder(base, security.NewLogEnricher()).Bind(ctx)
	log.Ctx(ctx).Info().Msg("api key call")

	line := sink.decode(t)
	if line["tenant_id"] != "tnt_platform" {
		t.Errorf("tenant_id = %v, want tnt_platform", line["tenant_id"])
	}
	if _, ok := line["user_id"]; ok {
		t.Errorf("expected no user_id for API key auth, got %v", line["user_id"])
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
