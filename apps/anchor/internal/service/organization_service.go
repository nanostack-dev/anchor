package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/pgkit/pglock"

	"anchor/internal/domain/license"
	"anchor/internal/domain/organization"
	licensesvc "anchor/internal/license/service"
	"anchor/internal/repository"

	"github.com/rs/zerolog"
)

const lockKeyCreateWithMember = "organization:create-with-member:%s:%s"

// Organization metadata limits. These are business rules, so they live here
// rather than as CHECK constraints on the column, and they are documented in
// the Metadata schema in openapi.yaml.
const (
	maxOrganizationMetadataKeys        = 50
	maxOrganizationMetadataKeyLength   = 64
	maxOrganizationMetadataValueLength = 512
)

// buildOrganizationMetadata validates caller-supplied metadata and encodes it
// for storage. A nil or empty map yields nil, which is stored as SQL NULL.
func buildOrganizationMetadata(metadata map[string]any) (json.RawMessage, error) {
	if len(metadata) == 0 {
		return nil, nil
	}

	if len(metadata) > maxOrganizationMetadataKeys {
		return nil, NewOrganizationMetadataTooManyKeysError(
			len(metadata), maxOrganizationMetadataKeys,
		)
	}

	for key, value := range metadata {
		if strings.TrimSpace(key) == "" || len(key) > maxOrganizationMetadataKeyLength {
			return nil, NewOrganizationMetadataInvalidKeyError(
				key, maxOrganizationMetadataKeyLength,
			)
		}

		switch typed := value.(type) {
		case string:
			if len(typed) > maxOrganizationMetadataValueLength {
				return nil, NewOrganizationMetadataValueTooLongError(
					key, maxOrganizationMetadataValueLength,
				)
			}
		case bool, float64, float32, json.Number,
			int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			// Scalar values are accepted as-is.
		default:
			// Rejects null, arrays and nested objects.
			return nil, NewOrganizationMetadataInvalidValueError(key)
		}
	}

	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fault.ErrUnexpected
	}

	return encoded, nil
}

type OrganizationService interface {
	Find(ctx context.Context, input organization.FindOrganizationInput) (
		*organization.Organization, error,
	)
	Create(
		ctx context.Context, input organization.CreateOrganizationInput,
	) (organization.CreateOrganizationResult, error)
	CreateWithMember(
		ctx context.Context, input organization.CreateOrganizationWithMemberInput,
	) (organization.OrganizationWithMemberResult, error)
	Update(
		ctx context.Context, input organization.UpdateOrganizationInput,
	) (organization.Organization, error)
	Delete(ctx context.Context, input organization.DeleteOrganizationInput) error
	Search(
		ctx context.Context, input organization.SearchProductOrganizationsInput,
	) (search.Result[organization.Organization], error)
}

type organizationService struct {
	organizationRepo  repository.OrganizationRepository
	orgMembershipRepo repository.OrganizationMembershipRepository
	productUserRepo   repository.ProductUserRepository
	productRoleRepo   repository.ProductRoleRepository
	licenses          licensesvc.OrganizationLicenseService
	licenseTemplates  licensesvc.LicenseTemplateService
	transactor        transactor.Transactor
	lock              *pglock.Client
	logger            zerolog.Logger
}

func NewOrganizationService(
	organizationRepo repository.OrganizationRepository,
	orgMembershipRepo repository.OrganizationMembershipRepository,
	productUserRepo repository.ProductUserRepository,
	productRoleRepo repository.ProductRoleRepository,
	licenses licensesvc.OrganizationLicenseService,
	licenseTemplates licensesvc.LicenseTemplateService,
	tx transactor.Transactor,
	lock *pglock.Client,
	logger zerolog.Logger,
) OrganizationService {
	return &organizationService{
		organizationRepo:  organizationRepo,
		orgMembershipRepo: orgMembershipRepo,
		productUserRepo:   productUserRepo,
		productRoleRepo:   productRoleRepo,
		licenses:          licenses,
		licenseTemplates:  licenseTemplates,
		transactor:        tx,
		lock:              lock,
		logger:            logger.With().Str("component", "organization_service").Logger(),
	}
}

func (s *organizationService) resolveLicenseTemplate(
	ctx context.Context, tenantID, productID string, templateID *string,
) (*license.Template, error) {
	if templateID == nil {
		return nil, nil //nolint:nilnil // no license asked for is not an error
	}

	template, err := s.licenseTemplates.GetTemplate(ctx, license.GetTemplateInput{
		TenantID:   tenantID,
		ProductID:  productID,
		TemplateID: *templateID,
	})
	if err != nil {
		return nil, err
	}
	if template == nil {
		// A bad request, not the 404 the license route answers: this call
		// addressed the organization collection, which exists.
		return nil, NewOrganizationLicenseTemplateNotFoundError(*templateID)
	}
	return template, nil
}

func (s *organizationService) instantiateLicense(
	ctx context.Context, tenantID, productID, organizationID string, template *license.Template,
) (*license.OrganizationLicense, error) {
	if template == nil {
		return nil, nil //nolint:nilnil // no license asked for is not an error
	}

	instantiated, err := s.licenses.InstantiateInTx(ctx, license.InstantiateLicenseInput{
		TenantID:       tenantID,
		ProductID:      productID,
		OrganizationID: organizationID,
		TemplateID:     template.ID,
	})
	if err != nil {
		return nil, err
	}
	return &instantiated, nil
}

func (s *organizationService) Find(
	ctx context.Context, input organization.FindOrganizationInput,
) (*organization.Organization, error) {
	logger := s.logger.With().Str("operation", "Find").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	org, err := s.organizationRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to find organization")
		return nil, err
	}
	return org, nil
}

func (s *organizationService) Create(
	ctx context.Context, input organization.CreateOrganizationInput,
) (organization.CreateOrganizationResult, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return organization.CreateOrganizationResult{}, err
	}

	metadataJSON, err := buildOrganizationMetadata(input.Metadata)
	if err != nil {
		return organization.CreateOrganizationResult{}, err
	}

	org := organization.Organization{
		ProductID:    input.ProductID,
		Name:         input.Name,
		Description:  input.Description,
		MetadataJSON: metadataJSON,
	}
	org.GenerateID()

	// One transaction so a refused license template leaves no organization behind.
	var result organization.CreateOrganizationResult
	if txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		template, templateErr := s.resolveLicenseTemplate(
			txCtx, input.TenantID, input.ProductID, input.LicenseTemplateID,
		)
		if templateErr != nil {
			return templateErr
		}

		created, createErr := s.organizationRepo.Create(txCtx, org)
		if createErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("name", input.Name).
				Err(createErr).
				Msg("failed to create organization")
			return createErr
		}

		instantiated, licenseErr := s.instantiateLicense(
			txCtx, input.TenantID, input.ProductID, created.ID, template,
		)
		if licenseErr != nil {
			logger.Error().
				Str("product_id", input.ProductID).
				Str("organization_id", created.ID).
				Err(licenseErr).
				Msg("failed to license the new organization")
			return licenseErr
		}

		result = organization.CreateOrganizationResult{Organization: created, License: instantiated}
		return nil
	}); txErr != nil {
		return organization.CreateOrganizationResult{}, txErr
	}

	logger.Info().
		Str("organization_id", result.Organization.ID).
		Str("product_id", input.ProductID).
		Str("name", input.Name).
		Bool("licensed", result.License != nil).
		Msg("organization created")

	return result, nil
}

func (s *organizationService) CreateWithMember(
	ctx context.Context, input organization.CreateOrganizationWithMemberInput,
) (organization.OrganizationWithMemberResult, error) {
	logger := s.logger.With().Str("operation", "CreateWithMember").Logger()

	if err := validateStruct(input); err != nil {
		return organization.OrganizationWithMemberResult{}, err
	}

	metadataJSON, err := buildOrganizationMetadata(input.Metadata)
	if err != nil {
		return organization.OrganizationWithMemberResult{}, err
	}

	existingMemberships, err := s.orgMembershipRepo.FindByProductUserID(
		ctx, input.ProductID, input.ProductUserID, false,
	)
	if err != nil {
		logger.Error().Err(err).
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Msg("failed to check existing memberships for idempotency")
		return organization.OrganizationWithMemberResult{}, err
	}
	if len(existingMemberships) > 0 {
		existing := existingMemberships[0]

		org, errGetOrg := s.organizationRepo.FindByID(ctx, input.ProductID, existing.OrganizationID)
		if errGetOrg != nil {
			return organization.OrganizationWithMemberResult{}, errGetOrg
		}
		if org == nil {
			return organization.OrganizationWithMemberResult{}, fault.ErrNotFound
		}

		membership, errGetOrg := s.orgMembershipRepo.FindByOrgIDAndUserID(
			ctx, input.ProductID, existing.OrganizationID, input.ProductUserID, false,
		)
		if errGetOrg != nil {
			return organization.OrganizationWithMemberResult{}, errGetOrg
		}
		if membership == nil {
			return organization.OrganizationWithMemberResult{}, fault.ErrNotFound
		}

		logger.Info().
			Str("product_id", input.ProductID).
			Str("organization_id", org.ID).
			Str("product_user_id", input.ProductUserID).
			Msg("CreateWithMember: returning existing org+membership (idempotent)")

		return organization.OrganizationWithMemberResult{
			Organization: *org,
			Membership:   *membership,
			WasExisting:  true,
		}, nil
	}

	// No existing membership — acquire the lock and create the org + founding membership atomically.
	lockKey := fmt.Sprintf(lockKeyCreateWithMember, input.ProductID, input.ProductUserID)

	var createdOrg organization.Organization
	var createdMembership organization.Membership
	var instantiatedLicense *license.OrganizationLicense

	acquired, err := s.lock.TryWithLock(ctx, lockKey, func(lockCtx context.Context, tx *sql.Tx) error {
		lockCtx = transactor.WithTx(lockCtx, tx)

		productUserEntity, lookupErr := s.productUserRepo.FindByProductIDAndID(
			lockCtx,
			input.ProductID,
			input.ProductUserID,
		)
		if lookupErr != nil {
			logger.Error().Err(lookupErr).
				Str("product_id", input.ProductID).
				Str("product_user_id", input.ProductUserID).
				Msg("failed to verify product user")
			return lookupErr
		}
		if productUserEntity == nil {
			return fault.ErrNotFound
		}

		role, roleErr := s.productRoleRepo.FindByProductIDAndRoleID(
			lockCtx,
			input.ProductID,
			input.RoleID,
		)
		if roleErr != nil {
			logger.Error().Err(roleErr).
				Str("product_id", input.ProductID).
				Str("role_id", input.RoleID).
				Msg("failed to verify product role")
			return roleErr
		}
		if role == nil {
			return NewRoleNotFoundError(input.RoleID)
		}

		template, templateErr := s.resolveLicenseTemplate(
			lockCtx, input.TenantID, input.ProductID, input.LicenseTemplateID,
		)
		if templateErr != nil {
			return templateErr
		}

		org := organization.Organization{
			ProductID:    input.ProductID,
			Name:         input.Name,
			Description:  input.Description,
			MetadataJSON: metadataJSON,
		}
		org.GenerateID()

		createdOrg, err = s.organizationRepo.Create(lockCtx, org)
		if err != nil {
			logger.Error().Err(err).
				Str("product_id", input.ProductID).
				Str("name", input.Name).
				Msg("failed to create organization")
			return err
		}

		createdMembership, err = s.orgMembershipRepo.Create(
			lockCtx, input.ProductID, createdOrg.ID, input.ProductUserID, input.RoleID,
		)
		if err != nil {
			logger.Error().Err(err).
				Str("product_id", input.ProductID).
				Str("organization_id", createdOrg.ID).
				Str("product_user_id", input.ProductUserID).
				Msg("failed to create founding membership")
			return err
		}

		createdLicense, licenseErr := s.instantiateLicense(
			lockCtx, input.TenantID, input.ProductID, createdOrg.ID, template,
		)
		if licenseErr != nil {
			logger.Error().Err(licenseErr).
				Str("product_id", input.ProductID).
				Str("organization_id", createdOrg.ID).
				Msg("failed to license the new organization")
			return licenseErr
		}
		instantiatedLicense = createdLicense

		return nil
	})
	if err != nil {
		return organization.OrganizationWithMemberResult{}, err
	}
	if !acquired {
		logger.Warn().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Msg("create-with-member lock was busy; concurrent request in progress")
		return organization.OrganizationWithMemberResult{}, fault.ErrUnexpected
	}

	logger.Info().
		Str("product_id", input.ProductID).
		Str("organization_id", createdOrg.ID).
		Str("product_user_id", input.ProductUserID).
		Str("role_id", input.RoleID).
		Msg("organization created with founding member")

	return organization.OrganizationWithMemberResult{
		Organization: createdOrg,
		Membership:   createdMembership,
		WasExisting:  false,
		License:      instantiatedLicense,
	}, nil
}

func (s *organizationService) Update(
	ctx context.Context, input organization.UpdateOrganizationInput,
) (organization.Organization, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return organization.Organization{}, err
	}

	metadataJSON, err := buildOrganizationMetadata(input.Metadata)
	if err != nil {
		return organization.Organization{}, err
	}

	optOrg, err := s.organizationRepo.FindByID(
		ctx, input.ProductID, input.OrganizationID,
	)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to find organization")
		return organization.Organization{}, err
	}
	if optOrg == nil {
		logger.Debug().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Msg("organization not found for update")
		return organization.Organization{}, fault.ErrNotFound
	}

	org := *optOrg
	if input.Name != nil {
		org.Name = *input.Name
	}
	org.Description = input.Description
	org.MetadataJSON = metadataJSON

	updated, err := s.organizationRepo.Update(ctx, input.ProductID, org)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to update organization")
		return organization.Organization{}, err
	}

	logger.Info().
		Str("organization_id", input.OrganizationID).
		Str("product_id", input.ProductID).
		Msg("organization updated")

	return updated, nil
}

func (s *organizationService) Delete(
	ctx context.Context, input organization.DeleteOrganizationInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	optOrg, err := s.organizationRepo.FindByID(
		ctx, input.ProductID, input.OrganizationID,
	)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to find organization")
		return err
	}
	if optOrg == nil {
		logger.Debug().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Msg("organization not found for deletion")
		return fault.ErrNotFound
	}

	err = s.organizationRepo.DeleteByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		logger.Error().
			Str("organization_id", input.OrganizationID).
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to delete organization")
		return err
	}

	logger.Info().
		Str("organization_id", input.OrganizationID).
		Str("product_id", input.ProductID).
		Msg("organization deleted")

	return nil
}

func (s *organizationService) Search(
	ctx context.Context, input organization.SearchProductOrganizationsInput,
) (search.Result[organization.Organization], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := validateStruct(input); err != nil {
		return search.Result[organization.Organization]{}, err
	}

	result, err := s.organizationRepo.SearchByProductID(ctx, input.ProductID, input.Request)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search organizations")
		return search.Result[organization.Organization]{}, err
	}

	return result, nil
}
