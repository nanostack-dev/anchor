package plan

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
)

type EntitlementType string

const (
	EntitlementTypeBoolean EntitlementType = "boolean"
	EntitlementTypeNumeric EntitlementType = "numeric"
)

// EntitlementValue is a single typed entitlement: a boolean feature gate or a
// numeric limit. Value holds a bool for boolean entitlements and a float64 for
// numeric ones (Normalize coerces other Go numeric types).
type EntitlementValue struct {
	Type  EntitlementType `json:"type"`
	Value any             `json:"value"`
}

// Entitlements maps stable entitlement keys (e.g.
// "flow_schedules.max_flows_per_run") to typed values.
type Entitlements map[string]EntitlementValue

const (
	maxEntitlementKeyLength = 200
	maxEntitlementEntries   = 200
)

// entitlementKeyPattern allows dot-separated snake_case segments, e.g.
// "executions.max_cloud_duration_seconds".
var entitlementKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)

var ErrTooManyEntitlements = fmt.Errorf(
	"entitlement map exceeds %d entries", maxEntitlementEntries,
)

// Validate checks key format and type/value coherence for every entry. Call
// Normalize first so numeric values are coerced to float64.
func (e Entitlements) Validate() error {
	if len(e) > maxEntitlementEntries {
		return ErrTooManyEntitlements
	}

	for key, value := range e {
		if err := validateEntitlementKey(key); err != nil {
			return err
		}
		if err := value.validate(key); err != nil {
			return err
		}
	}

	return nil
}

func validateEntitlementKey(key string) error {
	if key == "" {
		return errors.New("entitlement key must not be empty")
	}
	if len(key) > maxEntitlementKeyLength {
		return fmt.Errorf(
			"entitlement key %q exceeds %d characters", key, maxEntitlementKeyLength,
		)
	}
	if !entitlementKeyPattern.MatchString(key) {
		return fmt.Errorf(
			"entitlement key %q is invalid: keys are dot-separated snake_case segments (e.g. \"flow_schedules.max_flows_per_run\")",
			key,
		)
	}

	return nil
}

func (v EntitlementValue) validate(key string) error {
	switch v.Type {
	case EntitlementTypeBoolean:
		if _, ok := v.Value.(bool); !ok {
			return fmt.Errorf(
				"entitlement %q is boolean but value %v (%T) is not a bool", key, v.Value, v.Value,
			)
		}
	case EntitlementTypeNumeric:
		if _, ok := v.Value.(float64); !ok {
			return fmt.Errorf(
				"entitlement %q is numeric but value %v (%T) is not a number", key, v.Value, v.Value,
			)
		}
	default:
		return fmt.Errorf(
			"entitlement %q has unknown type %q (allowed: %q, %q)",
			key, v.Type, EntitlementTypeBoolean, EntitlementTypeNumeric,
		)
	}

	return nil
}

// Normalize returns a copy with numeric values coerced to float64 (JSON
// decoding and Go literals may produce int/int32/int64/float32). Non-numeric
// values are passed through untouched and rejected later by Validate.
func (e Entitlements) Normalize() Entitlements {
	if e == nil {
		return nil
	}

	normalized := make(Entitlements, len(e))
	for key, value := range e {
		if value.Type == EntitlementTypeNumeric {
			value.Value = coerceNumeric(value.Value)
		}
		normalized[key] = value
	}

	return normalized
}

func coerceNumeric(value any) any {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return value
	}
}

// MergedWith resolves plan defaults against per-license overrides: every key
// in overrides wins over the same key in e. Neither receiver nor argument is
// mutated.
func (e Entitlements) MergedWith(overrides Entitlements) Entitlements {
	merged := make(Entitlements, len(e)+len(overrides))
	maps.Copy(merged, e)
	maps.Copy(merged, overrides)

	return merged
}
