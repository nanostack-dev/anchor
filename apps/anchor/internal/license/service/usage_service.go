package service

// Recording what an Organization has used against the limits its license
// grants.
//
// # The checks that are here, and the one that is not
//
// A usage report is checked for four things: the Product's license schema
// declares the key, that license field is a limit, the value is a finite
// non-negative number, and the window is coherent.
//
// It is never checked against the license field's rules, and this service
// therefore holds no evaluator. Rules bound what a limit may be *set* to. An
// Organization that genuinely holds 512 flows against a limit of 500 must have
// that report accepted, or the "exceeded" status becomes unreachable and Anchor
// keeps serving a stale value reading "within_limit". See
// docs/adr/0001-anchor-validates-but-never-gates.md — Anchor validates the
// shape of a write and never refuses the reality it describes.
//
// # No license is required
//
// An Organization can report usage before it holds a license, and keeps
// reporting after a tier is withdrawn. Usage is what happened; it stays true
// whether or not a limit was ever granted. The schema is what a key is resolved
// against, and the schema belongs to the Product.

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

// isMissingOrganization reports whether an append failed because the
// Organization does not exist, or belongs to another Product. The two fail the
// same way, which is correct — from this Product's side they are the same
// thing.
//
// The constraint is matched by SQLSTATE alone rather than by name. Anchor's
// usual pattern names the constraint, but a hypertable cannot: TimescaleDB
// copies a constraint onto every chunk under a generated name, so the insert
// that fails reports "1_1_fk_usage_observations_organization_product" against
// "_hyper_1_1_chunk", and the prefix moves with each chunk.
//
// Matching the code alone is safe here because only one foreign key on this
// table can fail at this point. The Product and tenant were already resolved by
// the license schema read above, which is itself keyed on them.
//
// A foreign key added to usage_observations later would break that reasoning,
// and this is the line to revisit.
func isMissingOrganization(err error) bool {
	return pgerr.IsForeignKeyViolation(err)
}

// The ways a report is malformed on its own terms. Each is the contract-facing
// half of a sentinel in the domain package, which is where the check itself
// lives so it can be table-tested without a database.
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

// errLicenseFieldNotALimit reports usage measured against a license field that
// carries none. A boolean feature toggle is either granted or not; there is no
// amount of it to have used, so a report naming one is a mistake in the caller
// rather than a number Anchor should keep.
func errLicenseFieldNotALimit(name string, fieldType license.FieldType) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:     "LICENSE_FIELD_NOT_A_LIMIT",
		Message:  "Usage can only be reported against a limit, and the license field " + name + " is not one",
		Field:    name,
		Metadata: map[string]any{"type": string(fieldType)},
	}}, http.StatusBadRequest)
}

// UsageService records what an Organization has used. Reports are appended, so
// this service reads nothing back and derives nothing: the status a limit is in
// is computed when the license is read, never stored here.
type UsageService interface {
	// ReportUsage stores one absolute snapshot and returns it. Reporting the
	// same value twice is accepted and produces a second observation, because
	// Anchor never sums and the two are indistinguishable to every reader.
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

	// One clock reading for the whole report, so a window left open ends at the
	// same instant the observation is stamped with rather than a moment before.
	now := time.Now()

	// Everything the report can be judged on by itself, before a schema is
	// loaded. A malformed report costs no database round trip.
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
		// Set here rather than taken from the caller, so a consumer cannot
		// write into the future or rewrite its own history by backdating.
		ObservedAt: now,
	}
	observation.GenerateID()

	stored, err := s.observations.Append(ctx, observation)
	if err != nil {
		// Checked by the foreign key rather than by a read before the write:
		// one round trip, and no window in which the Organization is deleted
		// between the check and the insert.
		if isMissingOrganization(err) {
			return license.UsageObservation{}, ErrLicenseOrganizationNotFound
		}
		return license.UsageObservation{}, err
	}

	return stored, nil
}

// resolveLimit reports whether the key names a license field the Product
// declares, and whether that field is a limit. A key absent from the schema has
// to fail loudly: stored, it would start a series nothing will ever read.
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

// asUsageFault maps a domain sentinel onto the contract. The domain stays pure
// so its checks stay table-testable, and the HTTP shape is decided in one place
// here rather than at each check.
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
