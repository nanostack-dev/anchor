package license_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/license"
)

// TestMigratedValues pins what an Organization holds after a move onto another
// template. The target is taken whole and then the Organization's own values
// are put back over it, so every shape that "its own value" can take is named
// here: the field both templates declare, the field only the target declares,
// and the field the target dropped.
func TestMigratedValues(t *testing.T) {
	cases := []struct {
		name    string
		held    license.TemplateValues
		current license.TemplateValues
		target  license.TemplateValues
		policy  license.DifferencePolicy
		want    license.TemplateValues
	}{
		{
			name:    "an adjusted field survives the move",
			held:    license.TemplateValues{"flows": 800, "seats": 10},
			current: license.TemplateValues{"flows": 500, "seats": 10},
			target:  license.TemplateValues{"flows": 5000, "seats": 50},
			policy:  license.CarryForwardDifferences,
			want:    license.TemplateValues{"flows": 800, "seats": 50},
		},
		{
			name:    "discard takes the target whole",
			held:    license.TemplateValues{"flows": 800, "seats": 10},
			current: license.TemplateValues{"flows": 500, "seats": 10},
			target:  license.TemplateValues{"flows": 5000, "seats": 50},
			policy:  license.DiscardDifferences,
			want:    license.TemplateValues{"flows": 5000, "seats": 50},
		},
		{
			name: "a field the target dropped is not resurrected",
			// `legacy` is adjusted too, so this says the target's declaration
			// decides what a license carries, not the customer's own value.
			held:    license.TemplateValues{"flows": 800, "legacy": "on"},
			current: license.TemplateValues{"flows": 500, "legacy": "off"},
			target:  license.TemplateValues{"flows": 5000},
			policy:  license.CarryForwardDifferences,
			want:    license.TemplateValues{"flows": 800},
		},
		{
			name: "a field the schema gained after the copy takes the target value",
			// The license was instantiated before `seats` was declared, so it
			// holds nothing for it. There is no own value to carry.
			held:    license.TemplateValues{"flows": 500},
			current: license.TemplateValues{"flows": 500, "seats": 10},
			target:  license.TemplateValues{"flows": 5000, "seats": 50},
			policy:  license.CarryForwardDifferences,
			want:    license.TemplateValues{"flows": 5000, "seats": 50},
		},
		{
			name: "a field the current template does not declare counts as its own",
			// Held but absent from the template it is on: the value cannot have
			// come from that template, so it is this customer's own.
			held:    license.TemplateValues{"flows": 500, "seats": 3},
			current: license.TemplateValues{"flows": 500},
			target:  license.TemplateValues{"flows": 5000, "seats": 50},
			policy:  license.CarryForwardDifferences,
			want:    license.TemplateValues{"flows": 5000, "seats": 3},
		},
		{
			name:    "a license matching its template takes the target whole",
			held:    license.TemplateValues{"flows": 500},
			current: license.TemplateValues{"flows": 500},
			target:  license.TemplateValues{"flows": 5000},
			policy:  license.CarryForwardDifferences,
			want:    license.TemplateValues{"flows": 5000},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := license.MigratedValues(
				testCase.held, testCase.current, testCase.target, testCase.policy,
			)

			assert.Equal(t, testCase.want, got)
		})
	}
}

// TestMigratedValuesLeavesTheHeldSetAlone guards the copy: the caller keeps the
// license it passed in, so a failed write cannot leave a half-migrated value
// behind in memory.
func TestMigratedValuesLeavesTheHeldSetAlone(t *testing.T) {
	held := license.TemplateValues{"flows": 800}
	current := license.TemplateValues{"flows": 500}
	target := license.TemplateValues{"flows": 5000, "seats": 50}

	license.MigratedValues(held, current, target, license.CarryForwardDifferences)

	assert.Equal(t, license.TemplateValues{"flows": 800}, held)
}
