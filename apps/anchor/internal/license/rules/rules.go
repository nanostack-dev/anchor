// Package rules evaluates the validation rules declared on a license field.
//
// It answers two questions, and deliberately nothing else:
//
//   - [ValidateDeclaration] — is a rule set well-formed for a field type? Used
//     when a license schema is written, so a nonsensical declaration is refused
//     at authoring time rather than the first time a value is checked.
//   - [ValidateValue] — does a value satisfy a rule set? Used when a license
//     template or an organization's license sets a field.
//
// # Rules constrain decisions, not observations
//
// A rule bounds what a limit may be *set* to. It is never applied to a reported
// usage value. An organization that genuinely holds 150,000 flows must have that
// report accepted even where the field declares a maximum of 100,000, or the
// "exceeded" status becomes unreachable and anchor keeps serving a stale value
// that reads as compliant.
//
// This package cannot enforce that on its own — it validates whatever rule set
// it is handed. The guarantee lives in the caller: the usage path never passes a
// field's declared rules here.
//
// # Scope
//
// This package is pure logic: a rule set and a value in, a violation out. It
// holds no state, touches no database, and imports nothing licensing-specific,
// which is what lets its combinatorial matrix — field type × rule × valid,
// invalid, boundary — be table-tested directly rather than through a hundred
// HTTP round-trips. That is the whole reason it is its own package.
package rules

import (
	"fmt"
	"regexp"
	"slices"
	"unicode/utf8"
)

// FieldType is the type of a license field. The set is flat and closed: a
// license field is never a list or a nested object.
type FieldType string

const (
	// Limit is a non-negative numeric ceiling. Limits are the only fields that
	// carry usage and a derived status.
	Limit FieldType = "LIMIT"
	// Number is an arbitrary numeric value, which may be negative.
	Number FieldType = "NUMBER"
	// Boolean is an on/off field, such as a feature being available.
	Boolean FieldType = "BOOLEAN"
	// Enum is a value drawn from a declared list.
	Enum FieldType = "ENUM"
	// String is free text, optionally constrained by pattern or length.
	String FieldType = "STRING"
)

// FieldTypes lists every recognised field type, for validation and for
// enumerating the type in the API contract.
func FieldTypes() []FieldType {
	return []FieldType{Limit, Number, Boolean, Enum, String}
}

// Set is the validation rules declared on a license field. Every rule is
// optional; which ones are meaningful depends on the field type, and
// [ValidateDeclaration] rejects any that do not apply.
type Set struct {
	// Min and Max bound a numeric field, inclusive.
	Min *float64 `json:"min,omitempty"`
	Max *float64 `json:"max,omitempty"`
	// Pattern is a regular expression a string field must match.
	Pattern *string `json:"pattern,omitempty"`
	// MinLength and MaxLength bound a string field's length in runes, inclusive.
	MinLength *int `json:"min_length,omitempty"`
	MaxLength *int `json:"max_length,omitempty"`
	// Values is the list an enum field's value must be drawn from.
	Values []string `json:"values,omitempty"`
}

// ViolationError is a failed rule check. Rule names the rule that was violated —
// "min", "max", "pattern", "min_length", "max_length", "values", or "type" —
// so a caller can attribute the failure to a specific part of the declaration
// when building a structured API error.
type ViolationError struct {
	Rule    string
	Message string
}

func (v *ViolationError) Error() string {
	return fmt.Sprintf("%s: %s", v.Rule, v.Message)
}

func violate(rule, format string, args ...any) *ViolationError {
	return &ViolationError{Rule: rule, Message: fmt.Sprintf(format, args...)}
}

// numericTypes reports whether a field type holds a number.
func numeric(t FieldType) bool { return t == Limit || t == Number }

// ValidateDeclaration reports whether set is a well-formed rule declaration for
// fieldType. It rejects rules that do not apply to the type, bounds that cross,
// a regular expression that does not compile, and a malformed enum list.
func ValidateDeclaration(fieldType FieldType, set Set) error {
	if !slices.Contains(FieldTypes(), fieldType) {
		return violate("type", "unknown field type %q", fieldType)
	}

	if err := rejectInapplicable(fieldType, set); err != nil {
		return err
	}

	if err := validateBounds(fieldType, set); err != nil {
		return err
	}

	if err := validateLengths(set); err != nil {
		return err
	}

	if set.Pattern != nil {
		if _, err := regexp.Compile(*set.Pattern); err != nil {
			return violate("pattern", "does not compile: %v", err)
		}
	}

	if fieldType == Enum {
		return validateEnumValues(set.Values)
	}

	return nil
}

// rejectInapplicable refuses rules that carry no meaning for the field type,
// so a declaration cannot silently ignore part of itself.
func rejectInapplicable(fieldType FieldType, set Set) error {
	numericOK := numeric(fieldType)
	stringOK := fieldType == String

	switch {
	case set.Min != nil && !numericOK:
		return violate("min", "does not apply to a %s field", fieldType)
	case set.Max != nil && !numericOK:
		return violate("max", "does not apply to a %s field", fieldType)
	case set.Pattern != nil && !stringOK:
		return violate("pattern", "does not apply to a %s field", fieldType)
	case set.MinLength != nil && !stringOK:
		return violate("min_length", "does not apply to a %s field", fieldType)
	case set.MaxLength != nil && !stringOK:
		return violate("max_length", "does not apply to a %s field", fieldType)
	case set.Values != nil && fieldType != Enum:
		return violate("values", "does not apply to a %s field", fieldType)
	default:
		return nil
	}
}

// validateBounds checks Min and Max are coherent. A limit is a ceiling on a
// count, so neither bound may be negative; a plain number may go either way.
func validateBounds(fieldType FieldType, set Set) error {
	if fieldType == Limit {
		if set.Min != nil && *set.Min < 0 {
			return violate("min", "must not be negative for a limit")
		}
		if set.Max != nil && *set.Max < 0 {
			return violate("max", "must not be negative for a limit")
		}
	}

	if set.Min != nil && set.Max != nil && *set.Min > *set.Max {
		return violate("min", "must not exceed max (%v > %v)", *set.Min, *set.Max)
	}

	return nil
}

// validateLengths checks the string length bounds are non-negative and ordered.
func validateLengths(set Set) error {
	if set.MinLength != nil && *set.MinLength < 0 {
		return violate("min_length", "must not be negative")
	}
	if set.MaxLength != nil && *set.MaxLength < 0 {
		return violate("max_length", "must not be negative")
	}
	if set.MinLength != nil && set.MaxLength != nil && *set.MinLength > *set.MaxLength {
		return violate("min_length", "must not exceed max_length (%d > %d)", *set.MinLength, *set.MaxLength)
	}

	return nil
}

// validateEnumValues checks an enum declares a usable list: present, non-empty,
// with no blank or repeated entries.
func validateEnumValues(values []string) error {
	if len(values) == 0 {
		return violate("values", "an enum field must declare at least one allowed value")
	}

	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		if v == "" {
			return violate("values", "must not contain a blank value")
		}
		if _, duplicate := seen[v]; duplicate {
			return violate("values", "must not contain the duplicate value %q", v)
		}
		seen[v] = struct{}{}
	}

	return nil
}

// ValidateValue reports whether value satisfies set for fieldType. The value's
// Go type must match the field type; a numeric field accepts any integer or
// float, but never a bool.
//
// set is applied exactly as given. Callers that must not constrain a value —
// the usage report path — pass an empty Set.
func ValidateValue(fieldType FieldType, set Set, value any) error {
	switch fieldType {
	case Limit, Number:
		return validateNumericValue(fieldType, set, value)
	case Boolean:
		if _, ok := value.(bool); !ok {
			return violate("type", "expected a boolean, got %T", value)
		}
		return nil
	case Enum:
		return validateEnumValue(set, value)
	case String:
		return validateStringValue(set, value)
	default:
		return violate("type", "unknown field type %q", fieldType)
	}
}

func validateNumericValue(fieldType FieldType, set Set, value any) error {
	n, ok := toFloat(value)
	if !ok {
		return violate("type", "expected a number, got %T", value)
	}

	// A limit counts things, so it is non-negative regardless of what the
	// declaration says. An explicit Min below is checked on top of this.
	if fieldType == Limit && n < 0 {
		return violate("min", "a limit must not be negative, got %v", n)
	}
	if set.Min != nil && n < *set.Min {
		return violate("min", "%v is below the minimum of %v", n, *set.Min)
	}
	if set.Max != nil && n > *set.Max {
		return violate("max", "%v exceeds the maximum of %v", n, *set.Max)
	}

	return nil
}

func validateEnumValue(set Set, value any) error {
	s, ok := value.(string)
	if !ok {
		return violate("type", "expected a string, got %T", value)
	}
	if !slices.Contains(set.Values, s) {
		return violate("values", "%q is not one of %v", s, set.Values)
	}

	return nil
}

func validateStringValue(set Set, value any) error {
	s, ok := value.(string)
	if !ok {
		return violate("type", "expected a string, got %T", value)
	}

	// Length is counted in runes, so a multi-byte character costs one, not four.
	length := utf8.RuneCountInString(s)
	if set.MinLength != nil && length < *set.MinLength {
		return violate("min_length", "length %d is below the minimum of %d", length, *set.MinLength)
	}
	if set.MaxLength != nil && length > *set.MaxLength {
		return violate("max_length", "length %d exceeds the maximum of %d", length, *set.MaxLength)
	}

	if set.Pattern != nil {
		re, err := regexp.Compile(*set.Pattern)
		if err != nil {
			// A stored schema is validated on write, so this is unreachable in
			// practice. Report it as a pattern violation rather than panicking.
			return violate("pattern", "does not compile: %v", err)
		}
		if !re.MatchString(s) {
			return violate("pattern", "%q does not match %q", s, *set.Pattern)
		}
	}

	return nil
}

// toFloat converts any Go numeric type to a float64. It deliberately rejects
// bool, which Go would otherwise let through a careless type switch.
func toFloat(value any) (float64, bool) {
	switch n := value.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}
