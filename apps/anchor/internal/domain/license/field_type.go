package license

import "anchor/internal/license/rules"

// FieldType is the type of a license field. The set is flat and closed: a
// license field is never a list or a nested object.
//
// This is the domain's name for the type, and the one the OpenAPI contract
// maps onto, so the generated API package never reaches into the evaluator's
// subpackage.
//
// It is an alias, not a distinct type: there is one type here, not two that
// could drift, and no conversion at any boundary. The declaration sits in
// [rules] because that package is pure logic over its own inputs — a rule set
// and a value in, a violation out — and stays testable without a database or
// a domain import. The values stay there too; naming them again here would be
// the duplication the domain mapping exists to avoid.
type FieldType = rules.FieldType

// FieldRules is the validation rules declared on a license field: structured
// data, never an encoded tag string. Aliased for the same reason as
// [FieldType].
type FieldRules = rules.Set
