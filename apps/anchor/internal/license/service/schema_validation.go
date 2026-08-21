package service

// Checking a set of license field values against the declaration they must
// satisfy. This is the schema's half of the licensing subsystem that consumers
// ask about rather than edit: a license template asks it on every write, and
// the per-Organization license will ask the same question.
//
// # Values being set, never values observed
//
// Everything here runs on the path where a value is *chosen*. Rules constrain
// decisions, not observations: they bound what a limit may be set to and must
// never be applied to a reported usage value. Applying them there would make
// the "exceeded" status unreachable — an Organization genuinely past its
// ceiling would have its report refused, and Anchor would keep serving a stale
// value reading "within_limit". The usage path must not call ValidateValues.

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"

	"anchor/internal/domain/license"
	"anchor/internal/license/rules"
)

// errLicenseFieldUnknown reports a value set for a license field the schema does
// not declare. A typo has to fail loudly rather than being stored as a value
// nothing will ever read.
func errLicenseFieldUnknown(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_FIELD_UNKNOWN",
		Message: "The license field " + name + " is not declared by this product's license schema",
		Field:   name,
	}}, http.StatusBadRequest)
}

// errLicenseFieldMissing reports a declared license field left unset. Every
// declared field must be set, so no Organization can be instantiated with a
// hole in its license and no reader has to invent what an absent field means.
func errLicenseFieldMissing(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_FIELD_MISSING",
		Message: "The license field " + name + " is declared by this product's license schema and must be set",
		Field:   name,
	}}, http.StatusBadRequest)
}

// errLicenseValueInvalid reports a value the evaluator refused. The violated
// rule travels as metadata rather than being folded into the code, so a caller
// can attribute the failure to one part of the declaration — "max", "pattern",
// "values" — without matching on prose.
func errLicenseValueInvalid(name string, violation *rules.ViolationError) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "LICENSE_VALUE_INVALID",
		Message:  "The value set for the license field " + name + " is not allowed: " + violation.Message,
		Field:    name,
		Metadata: map[string]any{"rule": violation.Rule},
	}}, http.StatusBadRequest)
}

// ValidateValues reports whether values satisfy the Product's license schema:
// every key is declared, every declared field is set, and every value satisfies
// its field's rules.
//
// Every declared field is mandatory, so a set that omits one is refused. That
// is what lets a reader of a template — instantiation, status derivation, the
// admin UI — take the values at face value instead of deciding for itself what
// an absent field grants. The cost is that adding a field to a schema
// invalidates every existing template until each sets it; the schema write is
// not refused for it, because Anchor validates but never gates.
//
// It returns ErrLicenseSchemaNotDeclared when the Product has declared no
// schema, because there is then nothing for the values to satisfy. This call
// never names the schema itself, so a 409, not the 404 the schema route
// answers — see ErrLicenseSchemaNotDeclared.
//
// See the file comment: this is the path a value is *set* on, and the only kind
// of path that may consult a license field's rules.
func (s *licenseSchemaService) ValidateValues(
	ctx context.Context, tenantID string, productID string, values license.TemplateValues,
) error {
	schema, err := s.GetSchema(ctx, license.GetSchemaInput{
		TenantID:  tenantID,
		ProductID: productID,
	})
	if err != nil {
		return err
	}
	if schema == nil {
		return ErrLicenseSchemaNotDeclared
	}

	// Keys are visited in sorted order so a set with several problems always
	// reports the same one, and a test can name the field it expects.
	for _, name := range slices.Sorted(maps.Keys(values)) {
		if schema.FieldByName(name) == nil {
			return errLicenseFieldUnknown(name)
		}
	}

	// Fields are read back ordered by name, so a set missing several of them
	// always reports the same one and a test can name the field it expects.
	for i := range schema.Fields {
		field := schema.Fields[i]
		value, isSet := values[field.Name]
		if !isSet {
			return errLicenseFieldMissing(field.Name)
		}

		if err = rules.ValidateValue(field.Type, field.Rules, value); err != nil {
			if violation, ok := errors.AsType[*rules.ViolationError](err); ok {
				return errLicenseValueInvalid(field.Name, violation)
			}
			return err
		}
	}

	return nil
}
