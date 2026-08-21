package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
	"anchor/internal/license/rules"
)

// The rest of a malformed report is refused by the validate tags on
// license.ReportUsageInput, which report VALIDATION_ERROR with the rule.
var (
	ErrUsageValueNotFinite = fault.BadRequest(
		"USAGE_VALUE_NOT_FINITE",
		"A reported usage value must be a finite number",
	)
	ErrUsageWindowTooLong = fault.BadRequest(
		"USAGE_WINDOW_TOO_LONG",
		"A usage window cannot be longer than one year",
	)
)

func errLicenseFieldNotALimit(name string, fieldType license.FieldType) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "LICENSE_FIELD_NOT_A_LIMIT",
		Message:  "Usage can only be reported against a limit, and the license field " + name + " is not one",
		Field:    name,
		Metadata: map[string]any{"type": string(fieldType)},
	}}, http.StatusBadRequest)
}

// errLicenseFieldUsageShapeUndeclared reports a limit whose schema field
// carries no usage_shape. A limit declared before
// docs/adr/0013-usage-shape-is-declared-not-inferred.md can still have one:
// the column was added without a backfill, since nothing but the field's own
// owner can say whether its history reads as a gauge or a windowed counter.
// Redeclaring the field is what fills it in.
func errLicenseFieldUsageShapeUndeclared(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_FIELD_USAGE_SHAPE_UNDECLARED",
		Message: "The license field " + name + " has no usage_shape declared; redeclare it before reporting usage",
		Field:   name,
	}}, http.StatusBadRequest)
}

// errUsageShapeMismatch reports a usage report whose window presence
// contradicts the field's declared shape: a gauge report carries no window,
// and a windowed counter report requires one. See
// docs/adr/0013-usage-shape-is-declared-not-inferred.md.
//
// Named without the errLicenseField prefix its neighbour above carries: the
// field itself is well-formed here, and it is the report that is refused.
func errUsageShapeMismatch(name string, shape license.UsageShape) *fault.Error {
	requirement := "must not carry a window"
	if shape == license.UsageShapeWindowedCounter {
		requirement = "must carry a window"
	}
	message := "The license field " + name + " is declared " + string(shape) +
		" and a usage report against it " + requirement
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "USAGE_SHAPE_MISMATCH",
		Message:  message,
		Field:    name,
		Metadata: map[string]any{"usage_shape": string(shape)},
	}}, http.StatusBadRequest)
}

// UsageService records what an Organization has used. It holds no rules
// evaluator on purpose: rules bound what a limit may be set to, never what
// usage turns out to be, so a value past the limit is stored as reported.
type UsageService interface {
	ReportUsage(
		ctx context.Context, in license.ReportUsageInput,
	) (license.UsageObservation, error)
}

type usageService struct {
	observations licenserepo.UsageObservationRepository
	schemas      LicenseSchemaService
	logger       zerolog.Logger
}

func NewUsageService(
	observations licenserepo.UsageObservationRepository,
	schemas LicenseSchemaService,
	logger zerolog.Logger,
) UsageService {
	return &usageService{
		observations: observations,
		schemas:      schemas,
		logger:       logger.With().Str("component", "usage_service").Logger(),
	}
}

func (s *usageService) ReportUsage(
	ctx context.Context, in license.ReportUsageInput,
) (license.UsageObservation, error) {
	// One reading for the whole report, so a window left open ends at the same
	// instant the observation is stamped with. Defaults are filled before the
	// tags run, or gtfield would judge a To the caller never sent.
	now := time.Now()
	report := in.WithDefaults(now)

	if err := validate.ValidateStruct(report); err != nil {
		return license.UsageObservation{}, err
	}
	if err := report.Check(); err != nil {
		return license.UsageObservation{}, asUsageFault(err)
	}

	if err := s.resolveLimit(ctx, report); err != nil {
		return license.UsageObservation{}, err
	}

	observation := license.UsageObservation{
		PlatformTenantID: report.TenantID,
		ProductID:        report.ProductID,
		OrganizationID:   report.OrganizationID,
		Key:              report.Key,
		Value:            report.Value,
		From:             report.From,
		To:               report.To,
		ObservedAt:       now,
	}
	observation.GenerateID()

	stored, err := s.observations.Append(ctx, observation)
	if err != nil {
		if isMissingOrganization(err) {
			return license.UsageObservation{}, ErrLicenseOrganizationNotFound
		}
		return license.UsageObservation{}, err
	}

	return stored, nil
}

// resolveLimit reports whether the key names a license field the Product
// declares, whether that field is a limit, and whether the report's window
// presence matches the shape that field declares.
func (s *usageService) resolveLimit(ctx context.Context, in license.ReportUsageInput) error {
	schema, err := s.schemas.GetSchema(ctx, license.GetSchemaInput{
		TenantID:  in.TenantID,
		ProductID: in.ProductID,
	})
	if err != nil {
		return err
	}
	if schema == nil {
		// The usage route never names the schema itself, so this is not the
		// 404 the schema route answers — see ErrLicenseSchemaNotDeclared.
		return ErrLicenseSchemaNotDeclared
	}

	field := schema.FieldByName(in.Key)
	if field == nil {
		return errLicenseFieldUnknown(in.Key)
	}
	if field.Type != rules.Limit {
		return errLicenseFieldNotALimit(field.Name, field.Type)
	}
	if field.UsageShape == nil {
		return errLicenseFieldUsageShapeUndeclared(field.Name)
	}

	// From alone decides: To cannot arrive without it (required_with=To on
	// ReportUsageInput), and WithDefaults fills To in when a window is left
	// open, so From's presence is exactly the report's window presence.
	hasWindow := in.From != nil
	expectsWindow := *field.UsageShape == license.UsageShapeWindowedCounter
	if hasWindow != expectsWindow {
		return errUsageShapeMismatch(field.Name, *field.UsageShape)
	}

	return nil
}

// isMissingOrganization matches by SQLSTATE alone rather than by constraint
// name, because TimescaleDB copies a constraint onto every chunk under a
// generated name: the failure reports
// "1_1_fk_usage_observations_organization_product" against "_hyper_1_1_chunk".
//
// Only one foreign key on this table can fail at this point — the schema read
// above already resolved the Product and tenant. Adding another foreign key to
// usage_observations breaks that, and this is the line to revisit.
func isMissingOrganization(err error) bool {
	return pgerr.IsForeignKeyViolation(err)
}

func asUsageFault(err error) error {
	switch {
	case errors.Is(err, license.ErrUsageValueNotFinite):
		return ErrUsageValueNotFinite
	case errors.Is(err, license.ErrUsageWindowTooLong):
		return ErrUsageWindowTooLong
	default:
		return err
	}
}
