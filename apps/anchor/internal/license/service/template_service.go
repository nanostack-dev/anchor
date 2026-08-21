package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

// licenseTemplateNameConstraint is the partial UNIQUE (product_id, name) index
// on license_templates, postgres-named. It covers active templates only, so
// archiving a tier frees its name. It is the real guard against two templates
// claiming one name; the lookup before the write only exists to turn the race
// loser's failure into the same message the check would have produced.
const licenseTemplateNameConstraint = "uq_license_templates_product_name_active"

var (
	ErrLicenseTemplateNotFound = fault.NotFound(
		"LICENSE_TEMPLATE_NOT_FOUND",
		"This product has no license template with that identifier",
	)
	ErrLicenseTemplateArchived = fault.Conflict(
		"LICENSE_TEMPLATE_ARCHIVED",
		"This license template is archived; the tier is no longer offered",
	)
)

func ErrLicenseTemplateNotFoundInRequest(templateID string) *fault.Error {
	return fault.BadRequest(
		"LICENSE_TEMPLATE_NOT_FOUND_IN_REQUEST",
		"This product has no license template with that identifier",
	).Metadata(map[string]any{
		"template_id": templateID,
	})
}

// errLicenseTemplateInUse refuses to delete a template an Organization license
// still names. Archive is the withdrawal that stays available.
func errLicenseTemplateInUse(licenseCount int) *fault.Error {
	return fault.Conflict(
		"LICENSE_TEMPLATE_IN_USE",
		fmt.Sprintf(
			"This license template cannot be deleted because %d organization license(s) name it; archive it instead",
			licenseCount,
		),
	)
}

func errLicenseTemplateNameExists(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_TEMPLATE_NAME_EXISTS",
		Message: "This product already has a license template named " + name,
		Field:   "name",
	}}, http.StatusConflict)
}

// LicenseTemplateService owns a Product's license templates: named sets of
// values that satisfy the Product's license schema.
//
// Whether a set of values satisfies the schema is not decided here. That is the
// schema's question, answered by [LicenseSchemaService.ValidateValues], which
// is why this service holds no schema repository and never reads a declaration.
// What it owns is the template itself: its name, its identity, and the rule
// that a write is refused unless the schema accepts its values.
//
// Templates carry no version and no publish step, because a template is
// consulted once — when its values are copied onto an Organization's license.
// The one lifecycle step every template can reach is withdrawal: archiving,
// which never deletes the row, so the licenses that name it keep resolving.
// See docs/adr/0010-license-templates-are-archived.md. A template no
// Organization license has ever named can also be deleted outright. See
// docs/adr/0011-unreferenced-license-template-can-be-deleted.md.
type LicenseTemplateService interface {
	CreateTemplate(ctx context.Context, in license.CreateTemplateInput) (license.Template, error)
	// GetTemplate returns the template whatever its status. An archived one still
	// has to resolve, because a license names it.
	GetTemplate(ctx context.Context, in license.GetTemplateInput) (*license.Template, error)
	ListTemplates(ctx context.Context, in license.ListTemplatesInput) ([]license.Template, error)
	UpdateTemplate(ctx context.Context, in license.UpdateTemplateInput) (license.Template, error)
	// ArchiveTemplate withdraws the tier and returns it. It is idempotent, and it
	// cannot be undone. Prefer this to DeleteTemplate once a template might have
	// customers.
	ArchiveTemplate(
		ctx context.Context, in license.ArchiveTemplateInput,
	) (license.Template, error)
	// DeleteTemplate removes the template outright. Refused with
	// errLicenseTemplateInUse if any Organization license names it — archive
	// that one instead. Unlike ArchiveTemplate it is not idempotent: deleting an
	// already-deleted template answers 404, the ordinary shape for a second
	// delete of anything.
	DeleteTemplate(ctx context.Context, in license.DeleteTemplateInput) error
}

type licenseTemplateService struct {
	templateRepo   licenserepo.TemplateRepository
	orgLicenseRepo licenserepo.OrganizationLicenseRepository
	schemas        LicenseSchemaService
	logger         zerolog.Logger
}

func NewLicenseTemplateService(
	templateRepo licenserepo.TemplateRepository,
	orgLicenseRepo licenserepo.OrganizationLicenseRepository,
	schemas LicenseSchemaService,
	logger zerolog.Logger,
) LicenseTemplateService {
	return &licenseTemplateService{
		templateRepo:   templateRepo,
		orgLicenseRepo: orgLicenseRepo,
		schemas:        schemas,
		logger:         logger.With().Str("component", "license_template_service").Logger(),
	}
}

func (s *licenseTemplateService) CreateTemplate(
	ctx context.Context, in license.CreateTemplateInput,
) (license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Template{}, err
	}

	if err := s.schemas.ValidateValues(ctx, in.TenantID, in.ProductID, in.Values); err != nil {
		return license.Template{}, err
	}

	existing, err := s.templateRepo.FindByName(ctx, in.TenantID, in.ProductID, in.Name)
	if err != nil {
		return license.Template{}, err
	}
	if existing.IsPresent() {
		return license.Template{}, errLicenseTemplateNameExists(in.Name)
	}

	template := license.Template{
		PlatformTenantID: in.TenantID,
		ProductID:        in.ProductID,
		Name:             in.Name,
		Description:      in.Description,
		Status:           license.TemplateActive,
		Values:           in.Values,
	}
	template.GenerateID()

	created, err := s.templateRepo.Create(ctx, template)
	if err != nil {
		// Two creates racing both pass the check above at READ COMMITTED, so the
		// unique index is what actually decides. Last one in loses, and loses the
		// same way it would have lost the check.
		if pgerr.IsUniqueViolation(err, licenseTemplateNameConstraint) {
			return license.Template{}, errLicenseTemplateNameExists(in.Name)
		}
		return license.Template{}, err
	}

	return created, nil
}

func (s *licenseTemplateService) GetTemplate(
	ctx context.Context, in license.GetTemplateInput,
) (*license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return nil, err
	}
	return found.ToPtr(), nil
}

func (s *licenseTemplateService) ListTemplates(
	ctx context.Context, in license.ListTemplatesInput,
) ([]license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	return s.templateRepo.ListByProduct(ctx, in.TenantID, in.ProductID, in.Status)
}

func (s *licenseTemplateService) UpdateTemplate(
	ctx context.Context, in license.UpdateTemplateInput,
) (license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Template{}, err
	}

	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return license.Template{}, err
	}
	if found.IsAbsent() {
		return license.Template{}, ErrLicenseTemplateNotFound
	}
	existing := found.Value()
	if existing.IsArchived() {
		return license.Template{}, ErrLicenseTemplateArchived
	}

	if in.Description != nil {
		existing.Description = *in.Description
	}
	// A nil Values leaves the set alone; a supplied one replaces it wholesale, so
	// a license field the caller omitted is an unset rather than a no-op.
	if in.Values != nil {
		existing.Values = *in.Values
	}

	// Validated on every write, not only when Values changed. A rename is still a
	// write, and a template whose schema has tightened underneath it should be
	// corrected rather than quietly re-saved while it no longer satisfies the
	// declaration it is defined against.
	if err = s.schemas.ValidateValues(
		ctx, in.TenantID, in.ProductID, existing.Values,
	); err != nil {
		return license.Template{}, err
	}

	if in.Name != nil && *in.Name != existing.Name {
		conflict, findErr := s.templateRepo.FindByName(ctx, in.TenantID, in.ProductID, *in.Name)
		if findErr != nil {
			return license.Template{}, findErr
		}
		if conflict.IsPresent() {
			return license.Template{}, errLicenseTemplateNameExists(*in.Name)
		}
		existing.Name = *in.Name
	}

	updated, err := s.templateRepo.Update(ctx, in.TenantID, existing)
	if err != nil {
		if pgerr.IsUniqueViolation(err, licenseTemplateNameConstraint) {
			return license.Template{}, errLicenseTemplateNameExists(existing.Name)
		}
		return license.Template{}, err
	}

	return updated, nil
}

func (s *licenseTemplateService) ArchiveTemplate(
	ctx context.Context, in license.ArchiveTemplateInput,
) (license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Template{}, err
	}

	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return license.Template{}, err
	}
	if found.IsAbsent() {
		return license.Template{}, ErrLicenseTemplateNotFound
	}
	existing := found.Value()
	// Withdrawing a withdrawn tier is the outcome the caller asked for.
	if existing.IsArchived() {
		return existing, nil
	}

	// Organizations instantiated from this template keep their own copy of the
	// values, so there is nothing here to cascade. What the row is kept for is
	// the provenance those licenses name.
	return s.templateRepo.Archive(ctx, in.TenantID, in.ProductID, in.TemplateID)
}

// DeleteTemplate removes a template no Organization license names. See
// docs/adr/0011-unreferenced-license-template-can-be-deleted.md.
func (s *licenseTemplateService) DeleteTemplate(
	ctx context.Context, in license.DeleteTemplateInput,
) error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}

	found, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return ErrLicenseTemplateNotFound
	}

	// Checked before the write so the common case answers with the field-level
	// count rather than the foreign key's own error. This is not atomic against
	// a concurrent instantiation; fk_organization_licenses_template is the real
	// guarantee, and the race is handled below the same way it is for a
	// colliding name.
	licenseCount, err := s.orgLicenseRepo.CountLicensesForTemplate(
		ctx, in.TenantID, in.ProductID, in.TemplateID,
	)
	if err != nil {
		return err
	}
	if licenseCount > 0 {
		return errLicenseTemplateInUse(licenseCount)
	}

	if err = s.templateRepo.Delete(ctx, in.TenantID, in.ProductID, in.TemplateID); err != nil {
		if pgerr.IsForeignKeyViolation(err, "fk_organization_licenses_template") {
			return errLicenseTemplateInUse(1)
		}
		return err
	}

	return nil
}
