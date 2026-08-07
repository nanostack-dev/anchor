package service

import (
	"context"
	"net/http"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

// Postgres-named constraints on organization_licenses. The unique index is the
// real guard against one Organization holding two licenses. The composite
// foreign key refuses an Organization that does not exist and one belonging to
// another Product with the same failure, which is correct — from this Product's
// side the two are the same thing.
const (
	organizationLicenseUniqueConstraint       = "organization_licenses_organization_id_key"
	organizationLicenseOrganizationConstraint = "fk_organization_licenses_organization_product"
)

var (
	ErrOrganizationLicenseNotFound = fault.NewWithStatus(
		"ORGANIZATION_LICENSE_NOT_FOUND",
		"This organization has no license",
		http.StatusNotFound,
	)
	ErrOrganizationLicenseAlreadyExists = fault.BadRequest(
		"ORGANIZATION_LICENSE_EXISTS",
		"This organization already has a license; adjust it instead",
	)
	ErrLicenseOrganizationNotFound = fault.NewWithStatus(
		"ORGANIZATION_NOT_FOUND",
		"This product has no organization with that identifier",
		http.StatusNotFound,
	)
)

// OrganizationLicenseService owns one Organization's license: its own copy of a
// [license.Template]'s values. See
// docs/adr/0004-license-schema-template-and-copy.md.
//
// As with a template, whether a set of values satisfies the declaration is the
// schema's question, answered by [LicenseSchemaService.ValidateValues]. This
// service holds no schema repository, so an adjustment cannot drift from what a
// template write is held to.
type OrganizationLicenseService interface {
	Instantiate(
		ctx context.Context, in license.InstantiateLicenseInput,
	) (license.OrganizationLicense, error)
	// GetLicense returns the Organization's license, or nil when it has never
	// been instantiated.
	GetLicense(
		ctx context.Context, in license.GetLicenseInput,
	) (*license.OrganizationLicense, error)
	AdjustValues(
		ctx context.Context, in license.AdjustLicenseInput,
	) (license.OrganizationLicense, error)
	DiffAgainstTemplate(
		ctx context.Context, in license.GetLicenseInput,
	) (license.OrganizationLicenseDiff, error)
}

type organizationLicenseService struct {
	licenseRepo licenserepo.OrganizationLicenseRepository
	templates   LicenseTemplateService
	schemas     LicenseSchemaService
	logger      zerolog.Logger
}

func NewOrganizationLicenseService(
	licenseRepo licenserepo.OrganizationLicenseRepository,
	templates LicenseTemplateService,
	schemas LicenseSchemaService,
	logger zerolog.Logger,
) OrganizationLicenseService {
	return &organizationLicenseService{
		licenseRepo: licenseRepo,
		templates:   templates,
		schemas:     schemas,
		logger:      logger.With().Str("component", "organization_license_service").Logger(),
	}
}

func (s *organizationLicenseService) Instantiate(
	ctx context.Context, in license.InstantiateLicenseInput,
) (license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicense{}, err
	}

	template, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: in.TemplateID,
	})
	if err != nil {
		return license.OrganizationLicense{}, err
	}
	if template == nil {
		return license.OrganizationLicense{}, ErrLicenseTemplateNotFound
	}

	// The copied values are deliberately not re-validated: a schema tightened
	// since the template was last written would otherwise block onboarding onto
	// a tier still on sale, and Anchor validates but never gates. See
	// docs/adr/0009-every-license-field-is-mandatory.md.
	instantiated := license.OrganizationLicense{
		PlatformTenantID: in.TenantID,
		ProductID:        in.ProductID,
		OrganizationID:   in.OrganizationID,
		TemplateID:       template.ID,
		InstantiatedAt:   time.Now(),
		Values:           template.Values,
	}
	instantiated.GenerateID()

	created, err := s.licenseRepo.Create(ctx, instantiated)
	if err != nil {
		// Checked by the foreign key rather than by a read before the write: one
		// round trip, and no window in which the Organization is deleted between
		// the check and the insert.
		if pgerr.IsForeignKeyViolation(err, organizationLicenseOrganizationConstraint) {
			return license.OrganizationLicense{}, ErrLicenseOrganizationNotFound
		}
		if pgerr.IsUniqueViolation(err, organizationLicenseUniqueConstraint) {
			return license.OrganizationLicense{}, ErrOrganizationLicenseAlreadyExists
		}
		return license.OrganizationLicense{}, err
	}

	return created, nil
}

func (s *organizationLicenseService) GetLicense(
	ctx context.Context, in license.GetLicenseInput,
) (*license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	return s.licenseRepo.FindByOrganization(ctx, in.TenantID, in.ProductID, in.OrganizationID)
}

func (s *organizationLicenseService) AdjustValues(
	ctx context.Context, in license.AdjustLicenseInput,
) (license.OrganizationLicense, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicense{}, err
	}

	existing, err := s.licenseRepo.FindByOrganization(
		ctx, in.TenantID, in.ProductID, in.OrganizationID,
	)
	if err != nil {
		return license.OrganizationLicense{}, err
	}
	if existing == nil {
		return license.OrganizationLicense{}, ErrOrganizationLicenseNotFound
	}

	// Merged, not replaced. The merged set is validated whole, so a license that
	// has fallen behind a tightened schema is corrected rather than re-saved.
	adjusted := existing.AdjustedValues(in.Values)
	if err = s.schemas.ValidateValues(ctx, in.TenantID, in.ProductID, adjusted); err != nil {
		return license.OrganizationLicense{}, err
	}
	existing.Values = adjusted

	return s.licenseRepo.Update(ctx, in.TenantID, *existing)
}

func (s *organizationLicenseService) DiffAgainstTemplate(
	ctx context.Context, in license.GetLicenseInput,
) (license.OrganizationLicenseDiff, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.OrganizationLicenseDiff{}, err
	}

	existing, err := s.licenseRepo.FindByOrganization(
		ctx, in.TenantID, in.ProductID, in.OrganizationID,
	)
	if err != nil {
		return license.OrganizationLicenseDiff{}, err
	}
	if existing == nil {
		return license.OrganizationLicenseDiff{}, ErrOrganizationLicenseNotFound
	}

	// A deleted template leaves the license intact but leaves nothing to compare
	// against, so the diff is what is missing here, not the license.
	template, err := s.templates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   in.TenantID,
		ProductID:  in.ProductID,
		TemplateID: existing.TemplateID,
	})
	if err != nil {
		return license.OrganizationLicenseDiff{}, err
	}
	if template == nil {
		return license.OrganizationLicenseDiff{}, ErrLicenseTemplateNotFound
	}

	return license.OrganizationLicenseDiff{
		OrganizationID: existing.OrganizationID,
		TemplateID:     template.ID,
		Differences:    license.DiffValues(existing.Values, template.Values),
	}, nil
}
