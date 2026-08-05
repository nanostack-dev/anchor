package service_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/service"
)

func TestBuildOrganizationMetadata(t *testing.T) {
	t.Run(
		"NilMetadataIsStoredAsNull", func(t *testing.T) {
			encoded, err := service.BuildOrganizationMetadata(nil)

			require.NoError(t, err)
			assert.Nil(t, encoded, "nil metadata should map to a NULL column")
		},
	)

	t.Run(
		"EmptyMetadataIsStoredAsNull", func(t *testing.T) {
			encoded, err := service.BuildOrganizationMetadata(map[string]any{})

			require.NoError(t, err)
			assert.Nil(t, encoded, "empty metadata should map to a NULL column")
		},
	)

	t.Run(
		"AcceptsScalarValues", func(t *testing.T) {
			encoded, err := service.BuildOrganizationMetadata(
				map[string]any{
					"billing_ref": "cust_abc123",
					"seats":       float64(12),
					"trial":       true,
				},
			)

			require.NoError(t, err)

			var decoded map[string]any
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			assert.Equal(
				t, map[string]any{
					"billing_ref": "cust_abc123",
					"seats":       float64(12),
					"trial":       true,
				}, decoded,
			)
		},
	)

	t.Run(
		"RejectsNestedObject", func(t *testing.T) {
			_, err := service.BuildOrganizationMetadata(
				map[string]any{"billing": map[string]any{"ref": "cust_abc123"}},
			)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "string, number, or boolean")
		},
	)

	t.Run(
		"RejectsArrayValue", func(t *testing.T) {
			_, err := service.BuildOrganizationMetadata(
				map[string]any{"regions": []any{"us-east-1"}},
			)

			require.Error(t, err)
		},
	)

	t.Run(
		"RejectsNullValue", func(t *testing.T) {
			_, err := service.BuildOrganizationMetadata(map[string]any{"region": nil})

			require.Error(t, err)
		},
	)

	t.Run(
		"RejectsBlankKey", func(t *testing.T) {
			_, err := service.BuildOrganizationMetadata(map[string]any{"  ": "value"})

			require.Error(t, err)
		},
	)

	t.Run(
		"RejectsOverlongKey", func(t *testing.T) {
			_, err := service.BuildOrganizationMetadata(
				map[string]any{strings.Repeat("k", 65): "value"},
			)

			require.Error(t, err)
		},
	)

	t.Run(
		"RejectsOverlongStringValue", func(t *testing.T) {
			_, err := service.BuildOrganizationMetadata(
				map[string]any{"note": strings.Repeat("v", 513)},
			)

			require.Error(t, err)
		},
	)

	t.Run(
		"RejectsTooManyKeys", func(t *testing.T) {
			metadata := make(map[string]any, 51)
			for i := range 51 {
				metadata[string(rune('a'+i%26))+string(rune('a'+i/26))] = "value"
			}
			require.Len(t, metadata, 51, "test fixture should produce 51 distinct keys")

			_, err := service.BuildOrganizationMetadata(metadata)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "at most 50 keys")
		},
	)

	t.Run(
		"AcceptsMaximumAllowedSizes", func(t *testing.T) {
			encoded, err := service.BuildOrganizationMetadata(
				map[string]any{
					strings.Repeat("k", 64): strings.Repeat("v", 512),
				},
			)

			require.NoError(t, err)
			assert.NotNil(t, encoded)
		},
	)
}
