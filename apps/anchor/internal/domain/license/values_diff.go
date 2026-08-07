package license

import (
	"maps"
	"reflect"
	"slices"
)

// DifferenceKind names why one license field appears in a diff.
type DifferenceKind string

const (
	// DifferenceChanged is set on both sides with different values. On an
	// Organization's license this is the bespoke arrangement an operator is
	// looking for: someone raised this customer's limit.
	DifferenceChanged DifferenceKind = "changed"
	// DifferenceOnlyInLicense is set on the license and not on the template.
	// The template dropped the license field after the copy was taken.
	DifferenceOnlyInLicense DifferenceKind = "only_in_license"
	// DifferenceOnlyInTemplate is set on the template and not on the license.
	// The template gained the license field after the copy was taken, so this
	// Organization was never granted it.
	DifferenceOnlyInTemplate DifferenceKind = "only_in_template"
)

// FieldDifference is one license field on which two sets of values disagree.
// Both sides travel, so a caller can render "500 → 800" without a second read.
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
// A version comparison is unavailable by design: a license is a copy, and
// templates carry no version. The diff is what replaces it, and it is arguably
// the better answer because it names which license fields differ rather than
// which revision they came from. See
// docs/adr/0004-license-schema-template-and-copy.md.
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

// valuesEqual compares two license field values.
//
// Numbers are compared numerically rather than by their Go type. A value that
// has been through a JSON decode arrives as a float64 and one built in process
// does not, and reporting that as a deviation would put every Organization on
// the bespoke list. Everything else falls through to a deep comparison, which
// unlike == cannot panic on a value a tampered row put in the map.
func valuesEqual(a, b any) bool {
	aNumber, aIsNumber := asFloat(a)
	bNumber, bIsNumber := asFloat(b)
	if aIsNumber || bIsNumber {
		return aIsNumber && bIsNumber && aNumber == bNumber
	}
	return reflect.DeepEqual(a, b)
}

// asFloat reports a value as a float64 when it is any of the numeric types a
// license field value can arrive as: a JSON decode produces float64, the
// generated client and hand-built values produce the int kinds.
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
