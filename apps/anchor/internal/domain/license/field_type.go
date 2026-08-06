package license

import "anchor/internal/license/rules"

// FieldType is the type of a license field. The set is flat and closed: a
// license field is never a list or a nested object.
//
// This is the domain's name for the type, and the one the OpenAPI contract
// maps onto, so the generated API package never reaches into the evaluator's
// subpackage.
//
// It is an alias, not a distinct type. The direction of the dependency is
// fixed — the evaluator imports nothing licensing-specific so it can later
// move to nanostack-framework, so the domain names its type rather than the
// other way round — and an alias keeps that a rename rather than a second
// declaration that could drift. The values stay in [rules] for the same
// reason; naming them again here would be the duplication the domain mapping
// exists to avoid.
type FieldType = rules.FieldType

// FieldRules is the validation rules declared on a license field: structured
// data, never an encoded tag string. Aliased for the same reason as
// [FieldType].
type FieldRules = rules.Set
