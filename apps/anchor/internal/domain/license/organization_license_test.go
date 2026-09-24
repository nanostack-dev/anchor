package license_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/license"
)

func TestSyncedValues(t *testing.T) {
	cases := []struct {
		name     string
		held     license.TemplateValues
		adjusted []string
		template license.TemplateValues
		want     license.TemplateValues
	}{
		{
			name:     "an unadjusted license takes the template whole",
			held:     license.TemplateValues{"flows": 500, "seats": 10},
			adjusted: nil,
			template: license.TemplateValues{"flows": 5000, "seats": 50},
			want:     license.TemplateValues{"flows": 5000, "seats": 50},
		},
		{
			name:     "an adjusted field keeps the value held",
			held:     license.TemplateValues{"flows": 800, "seats": 10},
			adjusted: []string{"flows"},
			template: license.TemplateValues{"flows": 5000, "seats": 50},
			want:     license.TemplateValues{"flows": 800, "seats": 50},
		},
		{
			name:     "an adjusted field the template dropped is not resurrected",
			held:     license.TemplateValues{"flows": 800, "legacy": "on"},
			adjusted: []string{"legacy"},
			template: license.TemplateValues{"flows": 5000},
			want:     license.TemplateValues{"flows": 5000},
		},
		{
			name:     "a field the template gained lands with the template value",
			held:     license.TemplateValues{"flows": 500},
			adjusted: nil,
			template: license.TemplateValues{"flows": 5000, "seats": 50},
			want:     license.TemplateValues{"flows": 5000, "seats": 50},
		},
		{
			name:     "an adjusted field named but never held follows the template",
			held:     license.TemplateValues{"flows": 500},
			adjusted: []string{"seats"},
			template: license.TemplateValues{"flows": 5000, "seats": 50},
			want:     license.TemplateValues{"flows": 5000, "seats": 50},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			organizationLicense := license.OrganizationLicense{
				Values:         tc.held,
				AdjustedFields: tc.adjusted,
			}
			assert.Equal(t, tc.want, organizationLicense.SyncedValues(tc.template))
		})
	}
}

func TestRecordAdjustedFields(t *testing.T) {
	organizationLicense := license.OrganizationLicense{AdjustedFields: []string{"sso", "flows"}}

	organizationLicense.RecordAdjustedFields([]string{"flows", "region"})

	assert.Equal(t, []string{"flows", "region", "sso"}, organizationLicense.AdjustedFields)
}

func TestNewTemplateSyncChange(t *testing.T) {
	changedAt := time.Now()
	synced := license.OrganizationLicense{
		ID:               "lic_1",
		PlatformTenantID: "tenant_1",
		ProductID:        "prd_1",
		OrganizationID:   "org_1",
		TemplateID:       "ltpl_1",
		Values:           license.TemplateValues{"flows": 5000, "seats": 12},
	}
	previous := license.TemplateValues{"flows": 500, "seats": 12}

	change := license.NewTemplateSyncChange(synced, previous, changedAt)

	assert.NotEmpty(t, change.ID)
	assert.Equal(t, license.ChangeTemplateSynced, change.Type)
	assert.Equal(t, "lic_1", change.LicenseID)
	assert.Equal(t, "ltpl_1", *change.TemplateID)
	assert.Nil(t, change.PreviousTemplateID)
	assert.Nil(t, change.Field)
	assert.Equal(t, map[string]any(previous), change.OldValue)
	assert.Equal(t, map[string]any(synced.Values), change.NewValue)
	assert.Equal(t, changedAt, change.ChangedAt)
}
