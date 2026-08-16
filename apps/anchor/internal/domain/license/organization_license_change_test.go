package license_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
)

func adjustedLicense(values license.TemplateValues) license.OrganizationLicense {
	return license.OrganizationLicense{
		ID:               "lic_test",
		PlatformTenantID: "tenant_test",
		ProductID:        "prd_test",
		OrganizationID:   "org_test",
		TemplateID:       "ltpl_test",
		Values:           values,
	}
}

func TestNewAdjustmentChanges(t *testing.T) {
	t.Parallel()

	changedAt := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		previous license.TemplateValues
		adjusted license.TemplateValues
		expected []struct {
			field    string
			oldValue any
			newValue any
		}
	}{
		{
			name:     "a raised limit names the field and both values",
			previous: license.TemplateValues{"flows": 500.0, "sso": true},
			adjusted: license.TemplateValues{"flows": 800.0, "sso": true},
			expected: []struct {
				field    string
				oldValue any
				newValue any
			}{{field: "flows", oldValue: 500.0, newValue: 800.0}},
		},
		{
			name:     "a value restated unchanged records nothing",
			previous: license.TemplateValues{"flows": 500.0, "sso": true},
			adjusted: license.TemplateValues{"flows": 500.0, "sso": true},
			expected: nil,
		},
		{
			// A license behind a tightened schema gains the field on the next
			// adjustment. There is no earlier value to report.
			name:     "a field the license did not carry records an absent old value",
			previous: license.TemplateValues{"flows": 500.0},
			adjusted: license.TemplateValues{"flows": 500.0, "region": "ca-central"},
			expected: []struct {
				field    string
				oldValue any
				newValue any
			}{{field: "region", oldValue: nil, newValue: "ca-central"}},
		},
		{
			name:     "several moved fields are ordered by name",
			previous: license.TemplateValues{"flows": 500.0, "sso": true, "region": "ca-central"},
			adjusted: license.TemplateValues{"flows": 900.0, "sso": false, "region": "ca-central"},
			expected: []struct {
				field    string
				oldValue any
				newValue any
			}{
				{field: "flows", oldValue: 500.0, newValue: 900.0},
				{field: "sso", oldValue: true, newValue: false},
			},
		},
		{
			// A value that went through a JSON decode arrives as a float64 and
			// one built in process does not. Neither is a change.
			name:     "a number is compared numerically rather than by type",
			previous: license.TemplateValues{"flows": 500},
			adjusted: license.TemplateValues{"flows": 500.0},
			expected: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			changes := license.NewAdjustmentChanges(
				adjustedLicense(test.adjusted), test.previous, changedAt,
			)

			require.Len(t, changes, len(test.expected))
			for i, expected := range test.expected {
				assert.Equal(t, license.ChangeAdjusted, changes[i].Type)
				assert.Equal(t, expected.field, *changes[i].Field)
				assert.Equal(t, expected.oldValue, changes[i].OldValue)
				assert.Equal(t, expected.newValue, changes[i].NewValue)
				assert.Equal(t, changedAt, changes[i].ChangedAt)
				assert.Nil(t, changes[i].TemplateID, "an adjustment names no template")
				assert.NotEmpty(t, changes[i].ID)
			}
		})
	}
}

func TestNewInstantiationChange(t *testing.T) {
	t.Parallel()

	changedAt := time.Date(2026, time.August, 15, 12, 0, 0, 0, time.UTC)
	values := license.TemplateValues{"flows": 500.0, "sso": true}

	change := license.NewInstantiationChange(adjustedLicense(values), changedAt)

	assert.Equal(t, license.ChangeInstantiated, change.Type)
	assert.Equal(t, "ltpl_test", *change.TemplateID)
	assert.Equal(t, "lic_test", change.LicenseID)
	assert.Equal(t, map[string]any(values), change.NewValue)
	assert.Nil(t, change.Field, "an instantiation names no single license field")
	assert.Nil(t, change.OldValue, "the organization held no license before")
	assert.Equal(t, changedAt, change.ChangedAt)
	assert.NotEmpty(t, change.ID)
}
