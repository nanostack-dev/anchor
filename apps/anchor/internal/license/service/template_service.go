package service

import (
	"context"
	"net/http"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/pgerr"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	licenserepo "anchor/internal/license/repository"
)

// licenseTemplateNameConstraint is the UNIQUE (product_id, name) index on
// license_templates, postgres-named. It is the real guard against two templates
// claiming one name; the lookup before the write only exists to turn the race
// loser's failure into the same message the check would have produced.
const licenseTemplateNameConstraint = "license_templates_product_id_name_key"

var ErrLicenseTemplateNotFound = fault.NewWithStatus(
	"LICENSE_TEMPLATE_NOT_FOUND",
	"This product has no license template with that identifier",
	http.StatusNotFound,
)

// errLicenseTemplateNameExists reports a name already taken within the Product.
//
// Field names the request member a form has to highlight, which here is the
// template's own "name" input. The schema validation errors put a license field
// name there for the same reason — Field is what the operator edits, never the
// value they typed.
func errLicenseTemplateNameExists(name string) *fault.Error {
	return fault.NewWithDetails([]fault.Detail{{
		Code:    "LICENSE_TEMPLATE_NAME_EXISTS",
		Message: "This product already has a license template named " + name,
		Field:   "name",
	}}, http.StatusBadRequest)
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
// Templates carry no version and no lifecycle state. There is no publish step
// and nothing to archive, because a template is consulted once — when its
// values are copied onto an Organization's license.
type LicenseTemplateService interface {
	CreateTemplate(ctx context.Context, in license.CreateTemplateInput) (license.Template, error)
	GetTemplate(ctx context.Context, in license.GetTemplateInput) (*license.Template, error)
	ListTemplates(ctx context.Context, in license.ListTemplatesInput) ([]license.Template, error)
	UpdateTemplate(ctx context.Context, in license.UpdateTemplateInput) (license.Template, error)
	DeleteTemplate(ctx context.Context, in license.DeleteTemplateInput) error
}

type licenseTemplateService struct {
	templateRepo licenserepo.TemplateRepository
	schemas      LicenseSchemaService
	logger       zerolog.Logger
}

func NewLicenseTemplateService(
	templateRepo licenserepo.TemplateRepository,
	schemas LicenseSchemaService,
	logger zerolog.Logger,
) LicenseTemplateService {
	return &licenseTemplateService{
		templateRepo: templateRepo,
		schemas:      schemas,
		logger:       logger.With().Str("component", "license_template_service").Logger(),
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
	if existing != nil {
		return license.Template{}, errLicenseTemplateNameExists(in.Name)
	}

	template := license.Template{
		PlatformTenantID: in.TenantID,
		ProductID:        in.ProductID,
		Name:             in.Name,
		Description:      in.Description,
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

	return s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
}

func (s *licenseTemplateService) ListTemplates(
	ctx context.Context, in license.ListTemplatesInput,
) ([]license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return nil, err
	}

	return s.templateRepo.ListByProduct(ctx, in.TenantID, in.ProductID)
}

func (s *licenseTemplateService) UpdateTemplate(
	ctx context.Context, in license.UpdateTemplateInput,
) (license.Template, error) {
	if err := validate.ValidateStruct(in); err != nil {
		return license.Template{}, err
	}

	existing, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return license.Template{}, err
	}
	if existing == nil {
		return license.Template{}, ErrLicenseTemplateNotFound
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
		if conflict != nil {
			return license.Template{}, errLicenseTemplateNameExists(*in.Name)
		}
		existing.Name = *in.Name
	}

	updated, err := s.templateRepo.Update(ctx, in.TenantID, *existing)
	if err != nil {
		if pgerr.IsUniqueViolation(err, licenseTemplateNameConstraint) {
			return license.Template{}, errLicenseTemplateNameExists(existing.Name)
		}
		return license.Template{}, err
	}

	return updated, nil
}

func (s *licenseTemplateService) DeleteTemplate(
	ctx context.Context, in license.DeleteTemplateInput,
) error {
	if err := validate.ValidateStruct(in); err != nil {
		return err
	}

	existing, err := s.templateRepo.FindByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrLicenseTemplateNotFound
	}

	// Organizations instantiated from this template keep their own copy of the
	// values, so there is nothing here to cascade.
	return s.templateRepo.DeleteByID(ctx, in.TenantID, in.ProductID, in.TemplateID)
}
