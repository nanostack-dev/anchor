package email_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/domain/email"
)

func TestMissingRequiredVariables(t *testing.T) {
	schema := []email.VariableSchema{
		{Name: "name", Type: email.VariableTypeString, Required: true},
		{Name: "company", Type: email.VariableTypeString, Required: true},
		{Name: "nickname", Type: email.VariableTypeString, Required: false},
		{Name: "count", Type: email.VariableTypeNumber, Required: true},
	}

	tests := []struct {
		name string
		vars map[string]any
		want []string
	}{
		{
			name: "all required present",
			vars: map[string]any{"name": "Ada", "company": "Analytical", "count": 3},
			want: nil,
		},
		{
			name: "absent required reported in schema order",
			vars: map[string]any{"count": 1},
			want: []string{"name", "company"},
		},
		{
			name: "blank and whitespace strings count as missing",
			vars: map[string]any{"name": "", "company": "   ", "count": 1},
			want: []string{"name", "company"},
		},
		{
			name: "nil value counts as missing",
			vars: map[string]any{"name": nil, "company": "Co", "count": 1},
			want: []string{"name"},
		},
		{
			name: "optional missing is ignored, non-string required value is kept",
			vars: map[string]any{"name": "Ada", "company": "Co", "count": 0},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, email.MissingRequiredVariables(schema, tt.vars))
		})
	}
}
