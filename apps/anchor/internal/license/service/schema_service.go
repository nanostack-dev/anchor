// Package service holds the licensing orchestration layer: it validates a
// license schema declaration and persists it.
//
// Anchor validates but never gates. Nothing here refuses an action on the
// grounds that a limit was reached — the only writes it refuses are malformed
// ones.
package service

import (
	"context"
	"errors"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	"anchor/internal/license/rules"
)

// licenseSchemaProductConstraint is the UNIQUE (product_id) index on
// license_schemas, postgres-named. It is the real guard against two products
// declaring a schema at once.
const licenseSchemaProductConstraint = "license_schemas_product_id_key"

var (
	ErrLicenseSchemaNotFound = fault.NewWithStatus(
		"LICENSE_SCHEMA_NOT_FOUND",
		"This product has not declared a license schema",
		http.StatusNotFound,
	)
	ErrLicenseSchemaAlreadyExists = fault.BadRequest(
		"LICENSE_SCHEMA_EXISTS",
		"This product has already declared a license schema; update it instead",
	)
)

// errLicenseFieldNameDuplicate reports two declarations of the same field name.
// The name is both the message subject and the structured field, so a form can
// highlight the offending row without parsing the message.
func errLicenseFieldNameDuplicate(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_FIELD_NAME_DUPLICATE",
		Message: "The license field " + name + " is declared more than once",
		Field:   name,
	}}, http.StatusBadRequest)
}

// errLicenseFieldRuleInvalid reports a rule declaration the evaluator refused.
// The violated rule travels as metadata rather than being folded into the code,
// so a caller can attribute the failure to one part of the declaration —
// "max", "pattern", "values" — without matching on prose.
func errLicenseFieldRuleInvalid(name string, violation *rules.ViolationError) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "LICENSE_FIELD_RULE_INVALID",
		Message:  "The license field " + name + " is not well-formed: " + violation.Message,
		Field:    name,
		Metadata: map[string]any{"rule": violation.Rule},
	}}, http.StatusBadRequest)
}

// errLicenseFieldUsageShapeMissing reports a limit declared with no usage
// shape. Every report against it needs to be checked as a gauge or a windowed
// counter, and there is nothing to check it against without one. See
// docs/adr/0013-usage-shape-is-declared-not-inferred.md.
func errLicenseFieldUsageShapeMissing(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_FIELD_USAGE_SHAPE_MISSING",
		Message: "The license field " + name + " is a limit and must declare a usage_shape",
		Field:   name,
	}}, http.StatusBadRequest)
}

// errLicenseFieldUsageShapeNotApplicable reports a usage shape declared on a
// field that is not a limit. Only a limit ever carries usage, so a shape on
// anything else describes nothing.
func errLicenseFieldUsageShapeNotApplicable(name string, fieldType license.FieldType) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "LICENSE_FIELD_USAGE_SHAPE_NOT_APPLICABLE",
		Message:  "The license field " + name + " is not a limit and must not declare a usage_shape",
		Field:    name,
		Metadata: map[string]any{"type": string(fieldType)},
	}}, http.StatusBadRequest)
}

// errLicenseFieldUsageShapeInvalid reports a usage shape outside GAUGE and
// WINDOWED_COUNTER. The contract's request validator does not check request
// bodies (ExcludeRequestBody), so an unrecognised value reaches here rather
// than being refused before the handler runs — the same reason
// rules.ValidateDeclaration checks field type itself.
func errLicenseFieldUsageShapeInvalid(name string, shape license.UsageShape) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "LICENSE_FIELD_USAGE_SHAPE_INVALID",
		Message:  "The license field " + name + " declares an unrecognised usage_shape " + string(shape),
		Field:    name,
		Metadata: map[string]any{"usage_shape": string(shape)},
	}}, http.StatusBadRequest)
}

// LicenseSchemaService owns the license schema: the per-Product declaration of
// every field a license may carry.
//
// A schema is a singleton on its Product, so every method addresses it by
// product rather than by its own identifier.
//
// It also answers whether a set of values satisfies the declaration —
// [LicenseSchemaService.ValidateValues], defined in schema_validation.go. That
// question belongs to the schema because the schema is what makes a value legal
// or not; a license template asks it on every write, and the per-Organization
// license will ask it with the same question and the same errors. A consumer
// that reached for the schema repositories itself would end up owning a second
// copy of the answer, and the two would drift.
type LicenseSchemaService interface {
	CreateSchema(ctx context.Context, in license.CreateSchemaInput) (license.Schema, error)
	GetSchema(ctx context.Context, in license.GetSchemaInput) (*license.Schema, error)
	UpdateSchema(ctx context.Context, in license.UpdateSchemaInput) (license.Schema, error)
	DeleteSchema(ctx context.Context, in license.DeleteSchemaInput) error

	// ValidateValues checks values against the Product's schema: every key is
	// declared, every declared license field is set, and every value satisfies
	// its field's rules. See schema_validation.go.
	ValidateValues(
		ctx context.Context, tenantID string, productID string, values license.TemplateValues,
	) error
}

type licenseSchemaService struct {
	transactor transactor.Transactor
	schemaRepo licenserepo.SchemaRepository
	fieldRepo  licenserepo.SchemaFieldRepository
	logger     zerolog.Logger
}

func NewLicenseSchemaService(
	tx transactor.Transactor,
	schemaRepo licenserepo.SchemaRepository,
	fieldRepo licenserepo.SchemaFieldRepository,
	logger zerolog.Logger,
) LicenseSchemaService {
	return &licenseSchemaService{
		transactor: tx,
		schemaRepo: schemaRepo,
		fieldRepo:  fieldRepo,
		logger:     logger.With().Str("component", "license_schema_service").Logger(),
	}
}

// validateUsageShape checks that a field's usage_shape is present exactly
// when the field is a limit and recognised when present. Every other field
// type carries no usage, so a shape declared on one describes nothing and is
// refused rather than silently ignored. See
// docs/adr/0013-usage-shape-is-declared-not-inferred.md.
func validateUsageShape(d license.FieldDeclaration) error {
	if d.Type != rules.Limit {
		if d.UsageShape != nil {
			return errLicenseFieldUsageShapeNotApplicable(d.Name, d.Type)
		}
		return nil
	}

	if d.UsageShape == nil {
		return errLicenseFieldUsageShapeMissing(d.Name)
	}
	if !d.UsageShape.Valid() {
		return errLicenseFieldUsageShapeInvalid(d.Name, *d.UsageShape)
	}
	return nil
}

// declareFields checks a declaration and turns it into storable fields.
//
// The rule check is delegated wholesale to the rules evaluator; duplicating any
// of it here would let the two drift. Only name uniqueness is checked locally,
// because it is a property of the set rather than of any one field.
func declareFields(declarations []license.FieldDeclaration) ([]license.Field, error) {
	seen := make(map[string]struct{}, len(declarations))
	fields := make([]license.Field, 0, len(declarations))

	for _, d := range declarations {
		if _, duplicate := seen[d.Name]; duplicate {
			return nil, errLicenseFieldNameDuplicate(d.Name)
		}
		seen[d.Name] = struct{}{}

		if err := rules.ValidateDeclaration(d.Type, d.Rules); err != nil {
			if violation, ok := errors.AsType[*rules.ViolationError](err); ok {
				return nil, errLicenseFieldRuleInvalid(d.Name, violation)
			}
			return nil, err
		}

		if err := validateUsageShape(d); err != nil {
			return nil, err
		}

		field := license.Field{
			Name:        d.Name,
			Type:        d.Type,
			Description: d.Description,
			Rules:       d.Rules,
			UsageShape:  d.UsageShape,
		}
		field.GenerateID()
		fields = append(fields, field)
	}

	return fields, nil
}

func (s *licenseSchemaService) CreateSchema(
	ctx context.Context, in license.CreateSchemaInput,
) (license.Schema, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Schema{}, err
	}

	fields, err := declareFields(in.Fields)
	if err != nil {
		return license.Schema{}, err
	}

	schema := license.Schema{
		PlatformTenantID: in.TenantID,
		ProductID:        in.ProductID,
		Description:      in.Description,
	}
	schema.GenerateID()

	// The envelope and its fields are one declaration, so they land together or
	// not at all: a schema row without its fields would read as an empty
	// declaration rather than as a failed write. The "already declared" check
	// runs in here too, so it reads the same snapshot the insert writes to.
	var created license.Schema
	if txErr := inTx(ctx, s.transactor, func(txCtx context.Context) error {
		existing, findErr := s.schemaRepo.FindByProduct(txCtx, in.TenantID, in.ProductID)
		if findErr != nil {
			return findErr
		}
		if existing != nil {
			return ErrLicenseSchemaAlreadyExists
		}

		created, err = s.schemaRepo.Create(txCtx, schema)
		if err != nil {
			// Two creates racing both pass the check above at READ COMMITTED, so
			// the unique index is what actually decides. Last one in loses, and
			// loses the same way it would have lost the check.
			if pgerr.IsUniqueViolation(err, licenseSchemaProductConstraint) {
				return ErrLicenseSchemaAlreadyExists
			}
			return err
		}
		written, writeErr := s.fieldRepo.ReplaceAll(txCtx, created.ID, fields)
		if writeErr != nil {
			return writeErr
		}
		created.Fields = written
		return nil
	}); txErr != nil {
		return license.Schema{}, txErr
	}

	return created, nil
}

func (s *licenseSchemaService) GetSchema(
	ctx context.Context, in license.GetSchemaInput,
) (*license.Schema, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	schema, err := s.schemaRepo.FindByProduct(ctx, in.TenantID, in.ProductID)
	if err != nil {
		return nil, err
	}
	if schema == nil {
		return nil, nil //nolint:nilnil // absence is not an error; the handler maps it to 404
	}

	fields, err := s.fieldRepo.ListBySchema(ctx, schema.ID)
	if err != nil {
		return nil, err
	}
	schema.Fields = fields

	return schema, nil
}

func (s *licenseSchemaService) UpdateSchema(
	ctx context.Context, in license.UpdateSchemaInput,
) (license.Schema, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Schema{}, err
	}

	var fields []license.Field
	if in.Fields != nil {
		declared, err := declareFields(*in.Fields)
		if err != nil {
			return license.Schema{}, err
		}
		fields = declared
	}

	existing, err := s.schemaRepo.FindByProduct(ctx, in.TenantID, in.ProductID)
	if err != nil {
		return license.Schema{}, err
	}
	if existing == nil {
		return license.Schema{}, ErrLicenseSchemaNotFound
	}
	if in.Description != nil {
		existing.Description = *in.Description
	}

	var updated license.Schema
	if txErr := inTx(ctx, s.transactor, func(txCtx context.Context) error {
		updated, err = s.schemaRepo.Update(txCtx, in.TenantID, *existing)
		if err != nil {
			return err
		}
		// A nil Fields leaves the declaration alone; a non-nil one replaces it
		// wholesale, so a field the caller omitted is a removal.
		if in.Fields == nil {
			current, listErr := s.fieldRepo.ListBySchema(txCtx, updated.ID)
			if listErr != nil {
				return listErr
			}
			updated.Fields = current
			return nil
		}
		written, writeErr := s.fieldRepo.ReplaceAll(txCtx, updated.ID, fields)
		if writeErr != nil {
			return writeErr
		}
		updated.Fields = written
		return nil
	}); txErr != nil {
		return license.Schema{}, txErr
	}

	return updated, nil
}

func (s *licenseSchemaService) DeleteSchema(
	ctx context.Context, in license.DeleteSchemaInput,
) error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}

	existing, err := s.schemaRepo.FindByProduct(ctx, in.TenantID, in.ProductID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrLicenseSchemaNotFound
	}

	// Fields cascade with the envelope; the migration owns that, not this layer.
	return s.schemaRepo.DeleteByProduct(ctx, in.TenantID, in.ProductID)
}
