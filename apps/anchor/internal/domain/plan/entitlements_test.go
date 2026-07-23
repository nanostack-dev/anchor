package plan_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/plan"
)

func TestEntitlementsValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		entitlements plan.Entitlements
		wantErr      string
	}{
		{
			name:         "empty map is valid",
			entitlements: plan.Entitlements{},
		},
		{
			name:         "nil map is valid",
			entitlements: nil,
		},
		{
			name: "valid boolean and numeric entries",
			entitlements: plan.Entitlements{
				"api_access": {Type: plan.EntitlementTypeBoolean, Value: true},
				"flow_schedules.max_flows_per_run": {
					Type: plan.EntitlementTypeNumeric, Value: float64(25),
				},
			},
		},
		{
			name: "dotted multi-segment key",
			entitlements: plan.Entitlements{
				"a.b.c_d.e2": {Type: plan.EntitlementTypeBoolean, Value: false},
			},
		},
		{
			name: "invalid key format",
			entitlements: plan.Entitlements{
				"Not-Valid": {Type: plan.EntitlementTypeBoolean, Value: true},
			},
			wantErr: "is invalid",
		},
		{
			name: "key starting with digit",
			entitlements: plan.Entitlements{
				"1abc": {Type: plan.EntitlementTypeBoolean, Value: true},
			},
			wantErr: "is invalid",
		},
		{
			name: "boolean with numeric value",
			entitlements: plan.Entitlements{
				"api_access": {Type: plan.EntitlementTypeBoolean, Value: float64(1)},
			},
			wantErr: "not a bool",
		},
		{
			name: "numeric with string value",
			entitlements: plan.Entitlements{
				"max_runs": {Type: plan.EntitlementTypeNumeric, Value: "25"},
			},
			wantErr: "not a number",
		},
		{
			name: "unknown type",
			entitlements: plan.Entitlements{
				"max_runs": {Type: "metered", Value: float64(1)},
			},
			wantErr: "unknown type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.entitlements.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestEntitlementsNormalizeCoercesNumerics(t *testing.T) {
	t.Parallel()

	entitlements := plan.Entitlements{
		"int_val":     {Type: plan.EntitlementTypeNumeric, Value: 25},
		"int64_val":   {Type: plan.EntitlementTypeNumeric, Value: int64(9)},
		"float32_val": {Type: plan.EntitlementTypeNumeric, Value: float32(1.5)},
		"bool_val":    {Type: plan.EntitlementTypeBoolean, Value: true},
	}

	normalized := entitlements.Normalize()

	assert.InDelta(t, float64(25), normalized["int_val"].Value, 0.0001)
	assert.InDelta(t, float64(9), normalized["int64_val"].Value, 0.0001)
	assert.InDelta(t, 1.5, normalized["float32_val"].Value, 0.0001)
	assert.Equal(t, true, normalized["bool_val"].Value)
	require.NoError(t, normalized.Validate())

	// The original map is untouched.
	assert.Equal(t, 25, entitlements["int_val"].Value)
}

func TestEntitlementsMergedWithOverrideWins(t *testing.T) {
	t.Parallel()

	base := plan.Entitlements{
		"api_access": {Type: plan.EntitlementTypeBoolean, Value: false},
		"max_runs":   {Type: plan.EntitlementTypeNumeric, Value: float64(10)},
	}
	overrides := plan.Entitlements{
		"max_runs":  {Type: plan.EntitlementTypeNumeric, Value: float64(100)},
		"beta_flag": {Type: plan.EntitlementTypeBoolean, Value: true},
	}

	merged := base.MergedWith(overrides)

	assert.Len(t, merged, 3)
	assert.InDelta(t, float64(100), merged["max_runs"].Value, 0.0001)
	assert.Equal(t, false, merged["api_access"].Value)
	assert.Equal(t, true, merged["beta_flag"].Value)

	// Inputs are not mutated.
	assert.InDelta(t, float64(10), base["max_runs"].Value, 0.0001)
	assert.Len(t, base, 2)
	assert.Len(t, overrides, 2)
}

func TestEntitlementsMergedWithEmptyOverrides(t *testing.T) {
	t.Parallel()

	base := plan.Entitlements{
		"api_access": {Type: plan.EntitlementTypeBoolean, Value: true},
	}

	merged := base.MergedWith(nil)

	assert.Equal(t, base, merged)
}
