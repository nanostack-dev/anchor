package rules_test

import (
	"testing"

	"anchor/internal/license/rules"

	"github.com/stretchr/testify/require"
)

func TestValidateDeclaration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fieldType rules.FieldType
		set       rules.Set
		wantRule  string // empty means the declaration is expected to be valid
	}{
		// limit
		{name: "limit with no rules", fieldType: rules.Limit},
		{
			name:      "limit with min and max",
			fieldType: rules.Limit,
			set:       rules.Set{Min: new(float64(0)), Max: new(float64(100))},
		},
		{
			name:      "limit with min equal to max",
			fieldType: rules.Limit,
			set:       rules.Set{Min: new(float64(5)), Max: new(float64(5))},
		},
		{
			name: "limit with min above max", fieldType: rules.Limit,
			set: rules.Set{Min: new(float64(100)), Max: new(float64(0))}, wantRule: "min",
		},
		{
			name: "limit with negative min", fieldType: rules.Limit,
			set: rules.Set{Min: new(float64(-1))}, wantRule: "min",
		},
		{
			name: "limit with negative max", fieldType: rules.Limit,
			set: rules.Set{Max: new(float64(-1))}, wantRule: "max",
		},
		{
			name: "limit with a pattern", fieldType: rules.Limit,
			set: rules.Set{Pattern: new("^a$")}, wantRule: "pattern",
		},
		{
			name: "limit with allowed values", fieldType: rules.Limit,
			set: rules.Set{Values: []string{"a"}}, wantRule: "values",
		},

		// number — like limit, but may go negative
		{
			name:      "number with negative min",
			fieldType: rules.Number,
			set:       rules.Set{Min: new(float64(-10)), Max: new(float64(10))},
		},
		{
			name: "number with min above max", fieldType: rules.Number,
			set: rules.Set{Min: new(float64(1)), Max: new(float64(0))}, wantRule: "min",
		},

		// boolean
		{name: "boolean with no rules", fieldType: rules.Boolean},
		{
			name: "boolean with a min", fieldType: rules.Boolean,
			set: rules.Set{Min: new(float64(0))}, wantRule: "min",
		},

		// enum
		{name: "enum with values", fieldType: rules.Enum, set: rules.Set{Values: []string{"bronze", "gold"}}},
		{name: "enum without values", fieldType: rules.Enum, wantRule: "values"},
		{
			name: "enum with an empty value list", fieldType: rules.Enum,
			set: rules.Set{Values: []string{}}, wantRule: "values",
		},
		{
			name: "enum with duplicate values", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"gold", "gold"}}, wantRule: "values",
		},
		{
			name: "enum with a blank value", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"gold", ""}}, wantRule: "values",
		},
		{
			name: "enum with a max", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"gold"}, Max: new(float64(1))}, wantRule: "max",
		},

		// string
		{name: "string with no rules", fieldType: rules.String},
		{
			name: "string with a compilable pattern", fieldType: rules.String,
			set: rules.Set{Pattern: new(`^cust_[a-z0-9]+$`)},
		},
		{
			name: "string with an uncompilable pattern", fieldType: rules.String,
			set: rules.Set{Pattern: new("^cust_[a-z")}, wantRule: "pattern",
		},
		{
			name:      "string with a length range",
			fieldType: rules.String,
			set:       rules.Set{MinLength: new(1), MaxLength: new(10)},
		},
		{
			name: "string with min length above max length", fieldType: rules.String,
			set: rules.Set{MinLength: new(10), MaxLength: new(1)}, wantRule: "min_length",
		},
		{
			name: "string with a negative min length", fieldType: rules.String,
			set: rules.Set{MinLength: new(-1)}, wantRule: "min_length",
		},
		{
			name: "string with a min", fieldType: rules.String,
			set: rules.Set{Min: new(float64(1))}, wantRule: "min",
		},

		// unknown type
		{name: "unrecognised field type", fieldType: rules.FieldType("colour"), wantRule: "type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := rules.ValidateDeclaration(tc.fieldType, tc.set)

			if tc.wantRule == "" {
				require.NoError(t, err)
				return
			}

			var violation *rules.ViolationError
			require.ErrorAs(t, err, &violation)
			require.Equal(t, tc.wantRule, violation.Rule)
		})
	}
}

func TestValidateValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		fieldType rules.FieldType
		set       rules.Set
		value     any
		wantRule  string
	}{
		// limit
		{
			name:      "limit within range",
			fieldType: rules.Limit,
			set:       rules.Set{Min: new(float64(0)), Max: new(float64(100))},
			value:     50.0,
		},
		{
			name:      "limit at min boundary",
			fieldType: rules.Limit,
			set:       rules.Set{Min: new(float64(0)), Max: new(float64(100))},
			value:     0.0,
		},
		{
			name:      "limit at max boundary",
			fieldType: rules.Limit,
			set:       rules.Set{Min: new(float64(0)), Max: new(float64(100))},
			value:     100.0,
		},
		{
			name: "limit above max", fieldType: rules.Limit,
			set: rules.Set{Max: new(float64(100))}, value: 101.0, wantRule: "max",
		},
		{
			name: "limit below min", fieldType: rules.Limit,
			set: rules.Set{Min: new(float64(10))}, value: 9.0, wantRule: "min",
		},
		{name: "limit accepts an integer", fieldType: rules.Limit, value: 42},
		{name: "limit accepts an int64", fieldType: rules.Limit, value: int64(42)},
		{
			name: "limit rejects a negative value", fieldType: rules.Limit,
			value: -1.0, wantRule: "min",
		},
		{
			name: "limit rejects a string", fieldType: rules.Limit,
			value: "50", wantRule: "type",
		},
		{
			name: "limit rejects a bool", fieldType: rules.Limit,
			value: true, wantRule: "type",
		},
		{name: "limit with no rules accepts any non-negative number", fieldType: rules.Limit, value: 999999.0},

		// number
		{name: "number accepts a negative value", fieldType: rules.Number, value: -5.0},
		{
			name: "number below min", fieldType: rules.Number,
			set: rules.Set{Min: new(float64(-1))}, value: -2.0, wantRule: "min",
		},

		// boolean
		{name: "boolean true", fieldType: rules.Boolean, value: true},
		{name: "boolean false", fieldType: rules.Boolean, value: false},
		{
			name: "boolean rejects a string", fieldType: rules.Boolean,
			value: "true", wantRule: "type",
		},
		{
			name: "boolean rejects a number", fieldType: rules.Boolean,
			value: 1, wantRule: "type",
		},

		// enum
		{
			name: "enum with an allowed value", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"bronze", "gold"}}, value: "gold",
		},
		{
			name: "enum with a disallowed value", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"bronze", "gold"}}, value: "platinum", wantRule: "values",
		},
		{
			name: "enum is case sensitive", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"gold"}}, value: "Gold", wantRule: "values",
		},
		{
			name: "enum rejects a number", fieldType: rules.Enum,
			set: rules.Set{Values: []string{"gold"}}, value: 1, wantRule: "type",
		},

		// string
		{name: "string with no rules", fieldType: rules.String, value: "anything"},
		{
			name: "string matching its pattern", fieldType: rules.String,
			set: rules.Set{Pattern: new(`^cust_[a-z0-9]+$`)}, value: "cust_abc123",
		},
		{
			name: "string not matching its pattern", fieldType: rules.String,
			set: rules.Set{Pattern: new(`^cust_[a-z0-9]+$`)}, value: "customer_abc", wantRule: "pattern",
		},
		{
			name: "string at its minimum length", fieldType: rules.String,
			set: rules.Set{MinLength: new(3)}, value: "abc",
		},
		{
			name: "string below its minimum length", fieldType: rules.String,
			set: rules.Set{MinLength: new(3)}, value: "ab", wantRule: "min_length",
		},
		{
			name: "string above its maximum length", fieldType: rules.String,
			set: rules.Set{MaxLength: new(3)}, value: "abcd", wantRule: "max_length",
		},
		{
			name: "string length counts runes not bytes", fieldType: rules.String,
			set: rules.Set{MaxLength: new(3)}, value: "héé",
		},
		{
			name: "string rejects a number", fieldType: rules.String,
			value: 1, wantRule: "type",
		},

		// missing values
		{name: "nil value is rejected", fieldType: rules.String, value: nil, wantRule: "type"},

		// unknown type
		{name: "unrecognised field type", fieldType: rules.FieldType("colour"), value: "x", wantRule: "type"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := rules.ValidateValue(tc.fieldType, tc.set, tc.value)

			if tc.wantRule == "" {
				require.NoError(t, err)
				return
			}

			var violation *rules.ViolationError
			require.ErrorAs(t, err, &violation)
			require.Equal(t, tc.wantRule, violation.Rule)
		})
	}
}

// A usage report must never be checked against the field's rules. Reporting
// usage above the limit is exactly the "exceeded" case the system exists to
// surface, so the evaluator must not be reachable from that path with the
// field's declared ceiling. This test pins the property the service depends on:
// the value check is a pure function of the rules it is handed, so the service
// can hand it an empty set for usage.
func TestValidateValue_EmptySetAcceptsAnyNonNegativeNumber(t *testing.T) {
	t.Parallel()

	for _, v := range []any{0.0, 1.0, 512.0, 1e9} {
		require.NoError(t, rules.ValidateValue(rules.Limit, rules.Set{}, v))
	}
}
