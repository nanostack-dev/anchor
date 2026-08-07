package license

import (
	"maps"
	"reflect"
	"slices"
)

// DifferenceKind names why one license field appears in a diff.
type DifferenceKind string

// The contract documents what each kind means for a reader of the API. See
// LicenseDifferenceKind in cmd/http/openapi.yaml.
const (
	DifferenceChanged        DifferenceKind = "changed"
	DifferenceOnlyInLicense  DifferenceKind = "only_in_license"
	DifferenceOnlyInTemplate DifferenceKind = "only_in_template"
)

// FieldDifference is one license field on which two sets of values disagree.
// The side a [DifferenceKind] says is absent is left nil.
type FieldDifference struct {
	Field         string
	Kind          DifferenceKind
	LicenseValue  any
	TemplateValue any
}

// DiffValues reports how an Organization's license differs from the template it
// was instantiated from, one license field at a time.
//
// Differences come back ordered by license field name and never as a
// zero-length slice, so a caller can compare against nil. A nil set on either
// side reads as empty.
func DiffValues(licenseValues, templateValues TemplateValues) []FieldDifference {
	names := make(map[string]struct{}, len(licenseValues)+len(templateValues))
	for name := range licenseValues {
		names[name] = struct{}{}
	}
	for name := range templateValues {
		names[name] = struct{}{}
	}

	var differences []FieldDifference
	for _, name := range slices.Sorted(maps.Keys(names)) {
		licenseValue, inLicense := licenseValues[name]
		templateValue, inTemplate := templateValues[name]

		switch {
		case inLicense && !inTemplate:
			differences = append(differences, FieldDifference{
				Field: name, Kind: DifferenceOnlyInLicense, LicenseValue: licenseValue,
			})
		case !inLicense && inTemplate:
			differences = append(differences, FieldDifference{
				Field: name, Kind: DifferenceOnlyInTemplate, TemplateValue: templateValue,
			})
		case !valuesEqual(licenseValue, templateValue):
			differences = append(differences, FieldDifference{
				Field:         name,
				Kind:          DifferenceChanged,
				LicenseValue:  licenseValue,
				TemplateValue: templateValue,
			})
		}
	}

	return differences
}

// valuesEqual compares two license field values numerically where both are
// numbers, because a value that went through a JSON decode arrives as a float64
// and one built in process does not. Everything else falls through to a deep
// comparison, which unlike == cannot panic on a value a tampered row left.
func valuesEqual(a, b any) bool {
	aNumber, aIsNumber := asFloat(a)
	bNumber, bIsNumber := asFloat(b)
	if aIsNumber || bIsNumber {
		return aIsNumber && bIsNumber && aNumber == bNumber
	}
	return reflect.DeepEqual(a, b)
}

// asFloat reports a value as a float64 when it is any numeric type a license
// field value can arrive as.
func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	default:
		return 0, false
	}
}
