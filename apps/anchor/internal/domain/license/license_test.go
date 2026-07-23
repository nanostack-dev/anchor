package license_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/license"
	"anchor/internal/domain/plan"
)

func TestGraceBoundary(t *testing.T) {
	t.Parallel()

	expires := time.Now().Add(24 * time.Hour)
	grace := expires.Add(72 * time.Hour)

	tests := []struct {
		name    string
		license license.License
		want    *time.Time
	}{
		{name: "no expiry", license: license.License{}, want: nil},
		{
			name:    "expiry only",
			license: license.License{ExpiresAt: &expires},
			want:    &expires,
		},
		{
			name:    "grace wins over expiry",
			license: license.License{ExpiresAt: &expires, GraceUntil: &grace},
			want:    &grace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, tt.license.GraceBoundary())
		})
	}
}

func TestResolvedEntitlements(t *testing.T) {
	t.Parallel()

	lic := license.License{
		EntitlementOverrides: plan.Entitlements{
			"max_runs": {Type: plan.EntitlementTypeNumeric, Value: float64(100)},
		},
	}
	planEntitlements := plan.Entitlements{
		"max_runs":   {Type: plan.EntitlementTypeNumeric, Value: float64(10)},
		"api_access": {Type: plan.EntitlementTypeBoolean, Value: true},
	}

	resolved := lic.ResolvedEntitlements(planEntitlements)

	assert.Equal(t, float64(100), resolved["max_runs"].Value)
	assert.Equal(t, true, resolved["api_access"].Value)
}
