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

var (
	ErrUsageValueNegative = fault.BadRequest(
		"USAGE_VALUE_NEGATIVE",
		"A reported usage value cannot be negative",
	)
	ErrUsageValueNotFinite = fault.BadRequest(
		"USAGE_VALUE_NOT_FINITE",
		"A reported usage value must be a finite number",
	)
	ErrUsageWindowIncomplete = fault.BadRequest(
		"USAGE_WINDOW_INCOMPLETE",
		"A usage window that gives to must also give from",
	)
	ErrUsageWindowEmpty = fault.BadRequest(
		"USAGE_WINDOW_EMPTY",
		"A usage window must start before it ends",
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
	if err := validate.ValidateStruct(in); err != nil {
		return license.UsageObservation{}, err
	}

	// One reading for the whole report, so a window left open ends at the same
	// instant the observation is stamped with.
	now := time.Now()

	normalized, err := in.Normalize(now)
	if err != nil {
		return license.UsageObservation{}, asUsageFault(err)
	}

	if err = s.resolveLimit(ctx, normalized); err != nil {
		return license.UsageObservation{}, err
	}

	observation := license.UsageObservation{
		PlatformTenantID: normalized.TenantID,
		ProductID:        normalized.ProductID,
		OrganizationID:   normalized.OrganizationID,
		Key:              normalized.Key,
		Value:            normalized.Value,
		From:             normalized.From,
		To:               normalized.To,
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
// declares, and whether that field is a limit.
func (s *usageService) resolveLimit(ctx context.Context, in license.ReportUsageInput) error {
	schema, err := s.schemas.GetSchema(ctx, license.GetSchemaInput{
		TenantID:  in.TenantID,
		ProductID: in.ProductID,
	})
	if err != nil {
		return err
	}
	if schema == nil {
		return ErrLicenseSchemaNotFound
	}

	field := schema.FieldByName(in.Key)
	if field == nil {
		return errLicenseFieldUnknown(in.Key)
	}
	if field.Type != rules.Limit {
		return errLicenseFieldNotALimit(field.Name, field.Type)
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
	case errors.Is(err, license.ErrUsageValueNegative):
		return ErrUsageValueNegative
	case errors.Is(err, license.ErrUsageValueNotFinite):
		return ErrUsageValueNotFinite
	case errors.Is(err, license.ErrUsageWindowIncomplete):
		return ErrUsageWindowIncomplete
	case errors.Is(err, license.ErrUsageWindowEmpty):
		return ErrUsageWindowEmpty
	case errors.Is(err, license.ErrUsageWindowTooLong):
		return ErrUsageWindowTooLong
	default:
		return err
	}
}
