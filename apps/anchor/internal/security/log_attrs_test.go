package security_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/security"
)

func attrMap(attrs []slog.Attr) map[string]string {
	out := make(map[string]string, len(attrs))
	for _, a := range attrs {
		out[a.Key] = a.Value.String()
	}
	return out
}

// A platform user's request carries the user and the tenant.
func TestLogAttrsPublishThePlatformUser(t *testing.T) {
	ctx := security.SetCurrentUserID(context.Background(), "usr_4mHt9pLz")
	ctx = security.SetTenantID(ctx, "tnt_92xQ")

	assert.Equal(t, map[string]string{
		"principal_type": "user",
		"user_id":        "usr_4mHt9pLz",
		"tenant_id":      "tnt_92xQ",
	}, attrMap(security.LogAttrs(ctx)))
}

// An unauthenticated request is normal, not an error: nothing is added rather
// than empty fields or the fault GetCurrentUserID/GetTenantID would return.
func TestLogAttrsAddNothingWithoutACaller(t *testing.T) {
	assert.Empty(t, security.LogAttrs(context.Background()))
}

// Product API key auth sets a tenant and the key but no user; the key is the
// caller.
func TestLogAttrsPublishTheProductAPIKey(t *testing.T) {
	ctx := security.SetTenantID(context.Background(), "tnt_platform")
	ctx = security.SetProductAPIKeyCaller(ctx, security.ProductAPIKeyCaller{ID: "papk_7Qx", Name: "echopoint"})

	assert.Equal(t, map[string]string{
		"principal_type": "api_key",
		"api_key_id":     "papk_7Qx",
		"api_key_name":   "echopoint",
		"tenant_id":      "tnt_platform",
	}, attrMap(security.LogAttrs(ctx)))
}
