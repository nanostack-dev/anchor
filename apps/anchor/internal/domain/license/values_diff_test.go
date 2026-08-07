package license_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/license"
)

// TestDiffValues pins the comparison an Organization's license is read against
// its template with. It is the whole of "how does this customer differ from the
// tier they are on", so every shape that difference can take is named here.
func TestDiffValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		license  license.TemplateValues
		template license.TemplateValues
		expected []license.FieldDifference
	}{
		{
			name:     "a fresh copy differs in nothing",
			license:  license.TemplateValues{"flows": 500.0, "sso": true, "region": "ca-central"},
			template: license.TemplateValues{"flows": 500.0, "sso": true, "region": "ca-central"},
			expected: nil,
		},
		{
			name:     "an adjusted value is reported with both sides",
			license:  license.TemplateValues{"flows": 800.0, "sso": true},
			template: license.TemplateValues{"flows": 500.0, "sso": true},
			expected: []license.FieldDifference{{
				Field:         "flows",
				Kind:          license.DifferenceChanged,
				LicenseValue:  800.0,
				TemplateValue: 500.0,
			}},
		},
		{
			name:     "a field the template gained after instantiation",
			license:  license.TemplateValues{"flows": 500.0},
			template: license.TemplateValues{"flows": 500.0, "sso": true},
			expected: []license.FieldDifference{{
				Field:         "sso",
				Kind:          license.DifferenceOnlyInTemplate,
				TemplateValue: true,
			}},
		},
		{
			name:     "a field the template dropped after instantiation",
			license:  license.TemplateValues{"flows": 500.0, "sso": true},
			template: license.TemplateValues{"flows": 500.0},
			expected: []license.FieldDifference{{
				Field:        "sso",
				Kind:         license.DifferenceOnlyInLicense,
				LicenseValue: true,
			}},
		},
		{
			// Reading a diff is how an operator finds bespoke accounts, so the
			// order has to be the same on every call rather than a map's.
			name:     "differences are ordered by license field name",
			license:  license.TemplateValues{"zone": "b", "alpha": 1.0, "middle": true},
			template: license.TemplateValues{"zone": "a", "alpha": 2.0, "middle": false},
			expected: []license.FieldDifference{
				{Field: "alpha", Kind: license.DifferenceChanged, LicenseValue: 1.0, TemplateValue: 2.0},
				{Field: "middle", Kind: license.DifferenceChanged, LicenseValue: true, TemplateValue: false},
				{Field: "zone", Kind: license.DifferenceChanged, LicenseValue: "b", TemplateValue: "a"},
			},
		},
		{
			// One side has been through a JSON decode and the other has not.
			// Reporting that as a deviation would put every account on the
			// bespoke list.
			name:     "an int and a float of the same value are not a difference",
			license:  license.TemplateValues{"flows": 500},
			template: license.TemplateValues{"flows": 500.0},
			expected: nil,
		},
		{
			name:     "a number and a string of the same digits are a difference",
			license:  license.TemplateValues{"flows": "500"},
			template: license.TemplateValues{"flows": 500.0},
			expected: []license.FieldDifference{{
				Field:         "flows",
				Kind:          license.DifferenceChanged,
				LicenseValue:  "500",
				TemplateValue: 500.0,
			}},
		},
		{
			name:     "two empty sets differ in nothing",
			license:  license.TemplateValues{},
			template: license.TemplateValues{},
			expected: nil,
		},
		{
			name:     "a nil set is read as empty rather than as a failure",
			license:  nil,
			template: license.TemplateValues{"flows": 500.0},
			expected: []license.FieldDifference{{
				Field:         "flows",
				Kind:          license.DifferenceOnlyInTemplate,
				TemplateValue: 500.0,
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			differences := license.DiffValues(tt.license, tt.template)

			require.Len(t, differences, len(tt.expected))
			assert.Equal(t, tt.expected, differences)
		})
	}
}
