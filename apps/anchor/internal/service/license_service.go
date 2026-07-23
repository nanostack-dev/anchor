package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	"anchor/internal/domain/plan"
	"anchor/internal/repository"
)

type LicenseService interface {
	List(ctx context.Context, input license.ListLicensesInput) ([]license.License, error)
	Get(ctx context.Context, input license.GetLicenseInput) (*license.License, error)
	// Put assigns a license to the organization or fully replaces the existing
	// one (plan, expiry, grace, overrides, token TTL).
	Put(ctx context.Context, input license.PutLicenseInput) (license.License, error)
	Revoke(ctx context.Context, input license.RevokeLicenseInput) (license.License, error)
	Suspend(ctx context.Context, input license.SuspendLicenseInput) (license.License, error)
	Reinstate(ctx context.Context, input license.ReinstateLicenseInput) (license.License, error)
	// IssueToken resolves the organization's license into a signed PASETO
	// v4.public token. Organizations without a license row fall back to the
	// product's default plan when one exists.
	IssueToken(ctx context.Context, input license.IssueTokenInput) (license.IssuedToken, error)
	ListSigningKeys(
		ctx context.Context, input license.ListSigningKeysInput,
	) ([]license.SigningKey, error)
}

type licenseService struct {
	licenseRepo    repository.LicenseRepository
	planRepo       repository.PlanRepository
	signingKeyRepo repository.LicenseSigningKeyRepository
	orgRepo        repository.OrganizationRepository
	signer         LicenseSigningService
	transactor     transactor.Transactor
	logger         zerolog.Logger
}

func NewLicenseService(
	licenseRepo repository.LicenseRepository,
	planRepo repository.PlanRepository,
	signingKeyRepo repository.LicenseSigningKeyRepository,
	orgRepo repository.OrganizationRepository,
	signer LicenseSigningService,
	transactor transactor.Transactor,
	logger zerolog.Logger,
) LicenseService {
	return &licenseService{
		licenseRepo:    licenseRepo,
		planRepo:       planRepo,
		signingKeyRepo: signingKeyRepo,
		orgRepo:        orgRepo,
		signer:         signer,
		transactor:     transactor,
		logger:         logger.With().Str("component", "license_service").Logger(),
	}
}

func (s *licenseService) List(
	ctx context.Context, input license.ListLicensesInput,
) ([]license.License, error) {
	logger := s.logger.With().Str("operation", "List").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	licenses, err := s.licenseRepo.ListByProduct(ctx, input.ProductID)
	if err != nil {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to list licenses")
		return nil, err
	}

	return licenses, nil
}

func (s *licenseService) Get(
	ctx context.Context, input license.GetLicenseInput,
) (*license.License, error) {
	logger := s.logger.With().Str("operation", "Get").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.licenseRepo.FindByOrganization(
		ctx, input.ProductID, input.OrganizationID,
	)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to find license")
		return nil, err
	}

	return found, nil
}

func (s *licenseService) Put(
	ctx context.Context, input license.PutLicenseInput,
) (license.License, error) {
	logger := s.logger.With().Str("operation", "Put").Logger()

	if err := validateStruct(input); err != nil {
		return license.License{}, err
	}
	if input.Status != nil && !input.Status.IsValid() {
		return license.License{}, NewInvalidLicenseStatusError(string(*input.Status))
	}
	if input.ExpiresAt != nil && input.GraceUntil != nil &&
		input.GraceUntil.Before(*input.ExpiresAt) {
		return license.License{}, NewInvalidLicenseGraceError()
	}
	overrides, err := normalizeEntitlements(input.EntitlementOverrides)
	if err != nil {
		return license.License{}, err
	}

	organization, err := s.orgRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to find organization")
		return license.License{}, err
	}
	if organization == nil {
		return license.License{}, fault.ErrNotFound
	}

	targetPlan, err := s.planRepo.FindByID(ctx, input.ProductID, input.PlanID)
	if err != nil {
		logger.Error().Str("plan_id", input.PlanID).Err(err).Msg("failed to find plan")
		return license.License{}, err
	}
	if targetPlan == nil {
		return license.License{}, NewPlanReferenceInvalidError(input.PlanID)
	}

	tokenTTL := license.DefaultTokenTTLSeconds
	if input.TokenTTLSeconds != nil {
		tokenTTL = *input.TokenTTLSeconds
	}

	var result license.License
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.licenseRepo.FindByOrganization(
			txCtx, input.ProductID, input.OrganizationID,
		)
		if findErr != nil {
			return findErr
		}

		if existing == nil {
			newLicense := license.License{
				ProductID:            input.ProductID,
				OrganizationID:       input.OrganizationID,
				PlanID:               input.PlanID,
				Status:               license.StatusActive,
				ExpiresAt:            input.ExpiresAt,
				GraceUntil:           input.GraceUntil,
				EntitlementOverrides: overrides,
				TokenTTLSeconds:      tokenTTL,
				CreatedAt:            time.Now(),
				UpdatedAt:            time.Now(),
			}
			if input.Status != nil {
				newLicense.Status = *input.Status
			}
			newLicense.GenerateID()

			var createErr error
			result, createErr = s.licenseRepo.Create(txCtx, newLicense)
			if createErr != nil {
				logger.Error().Str("organization_id", input.OrganizationID).Err(createErr).
					Msg("failed to create license")
				return createErr
			}
			logger.Info().Str("license_id", result.ID).
				Str("organization_id", input.OrganizationID).Msg("license created")
			return nil
		}

		updatedLicense := *existing
		updatedLicense.PlanID = input.PlanID
		updatedLicense.ExpiresAt = input.ExpiresAt
		updatedLicense.GraceUntil = input.GraceUntil
		updatedLicense.EntitlementOverrides = overrides
		updatedLicense.TokenTTLSeconds = tokenTTL
		if input.Status != nil {
			updatedLicense.Status = *input.Status
		}

		var updateErr error
		result, updateErr = s.licenseRepo.Update(txCtx, input.ProductID, updatedLicense)
		if updateErr != nil {
			logger.Error().Str("license_id", existing.ID).Err(updateErr).
				Msg("failed to update license")
			return updateErr
		}
		logger.Info().Str("license_id", result.ID).
			Str("organization_id", input.OrganizationID).Msg("license updated")
		return nil
	})

	return result, err
}

func (s *licenseService) Revoke(
	ctx context.Context, input license.RevokeLicenseInput,
) (license.License, error) {
	if err := validateStruct(input); err != nil {
		return license.License{}, err
	}

	return s.setStatus(
		ctx, input.ProductID, input.OrganizationID, license.StatusRevoked, "Revoke",
	)
}

func (s *licenseService) Suspend(
	ctx context.Context, input license.SuspendLicenseInput,
) (license.License, error) {
	if err := validateStruct(input); err != nil {
		return license.License{}, err
	}

	return s.setStatus(
		ctx, input.ProductID, input.OrganizationID, license.StatusSuspended, "Suspend",
	)
}

func (s *licenseService) Reinstate(
	ctx context.Context, input license.ReinstateLicenseInput,
) (license.License, error) {
	if err := validateStruct(input); err != nil {
		return license.License{}, err
	}

	return s.setStatus(
		ctx, input.ProductID, input.OrganizationID, license.StatusActive, "Reinstate",
	)
}

func (s *licenseService) setStatus(
	ctx context.Context,
	productID string,
	organizationID string,
	status license.Status,
	operation string,
) (license.License, error) {
	logger := s.logger.With().Str("operation", operation).Logger()

	var result license.License
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.licenseRepo.FindByOrganization(
			txCtx, productID, organizationID,
		)
		if findErr != nil {
			logger.Error().Str("organization_id", organizationID).Err(findErr).
				Msg("failed to find license")
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}

		updatedLicense := *existing
		updatedLicense.Status = status

		var updateErr error
		result, updateErr = s.licenseRepo.Update(txCtx, productID, updatedLicense)
		if updateErr != nil {
			logger.Error().Str("license_id", existing.ID).Err(updateErr).
				Msg("failed to update license status")
			return updateErr
		}

		logger.Info().Str("license_id", result.ID).Str("status", string(status)).
			Msg("license status changed")
		return nil
	})

	return result, err
}

func (s *licenseService) IssueToken(
	ctx context.Context, input license.IssueTokenInput,
) (license.IssuedToken, error) {
	logger := s.logger.With().Str("operation", "IssueToken").Logger()

	if err := validateStruct(input); err != nil {
		return license.IssuedToken{}, err
	}

	organization, err := s.orgRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to find organization")
		return license.IssuedToken{}, err
	}
	if organization == nil {
		return license.IssuedToken{}, fault.ErrNotFound
	}

	lic, err := s.licenseRepo.FindByOrganization(
		ctx, input.ProductID, input.OrganizationID,
	)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to find license")
		return license.IssuedToken{}, err
	}

	now := time.Now()
	snapshot, err := s.resolveTokenSnapshot(ctx, input, lic, now, logger)
	if err != nil {
		return license.IssuedToken{}, err
	}

	// Consumers should refresh at half the token lifetime (design doc:
	// refresh_after = ttl/2) so a full grace window remains on failure.
	const refreshAfterDivisor = 2

	ttl := time.Duration(snapshot.tokenTTLSeconds) * time.Second
	claims := license.Claims{
		OrganizationID: input.OrganizationID,
		ProductID:      input.ProductID,
		PlanKey:        snapshot.planKey,
		Status:         snapshot.status,
		Entitlements:   snapshot.entitlements,
		IssuedAt:       now,
		ExpiresAt:      now.Add(ttl),
		GraceUntil:     snapshot.graceUntil,
		RefreshAfter:   now.Add(ttl / refreshAfterDivisor),
		SchemaVersion:  license.ClaimsSchemaVersion,
	}

	token, err := s.signer.Sign(ctx, claims)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to sign license token")
		return license.IssuedToken{}, err
	}

	return license.IssuedToken{
		Token:        token,
		RefreshAfter: claims.RefreshAfter,
		ExpiresAt:    claims.ExpiresAt,
	}, nil
}

// tokenSnapshot is the resolved state a token gets minted from.
type tokenSnapshot struct {
	planKey         string
	status          license.TokenStatus
	entitlements    plan.Entitlements
	graceUntil      *time.Time
	tokenTTLSeconds int32
}

// resolveTokenSnapshot applies the issuance semantics: default-plan fallback
// for organizations without a license row; REVOKED or past the grace boundary
// is a 409; SUSPENDED still issues (status SUSPENDED); past expiry but within
// grace issues with status GRACE.
func (s *licenseService) resolveTokenSnapshot(
	ctx context.Context,
	input license.IssueTokenInput,
	lic *license.License,
	now time.Time,
	logger zerolog.Logger,
) (tokenSnapshot, error) {
	if lic == nil {
		defaultPlan, err := s.planRepo.FindDefault(ctx, input.ProductID)
		if err != nil {
			logger.Error().Str("product_id", input.ProductID).Err(err).
				Msg("failed to find default plan")
			return tokenSnapshot{}, err
		}
		if defaultPlan == nil {
			return tokenSnapshot{}, NewLicenseNotFoundError(input.OrganizationID)
		}

		return tokenSnapshot{
			planKey:         defaultPlan.Key,
			status:          license.TokenStatusActive,
			entitlements:    defaultPlan.Entitlements,
			tokenTTLSeconds: license.DefaultTokenTTLSeconds,
		}, nil
	}

	if lic.Status == license.StatusRevoked {
		return tokenSnapshot{}, ErrLicenseRevoked
	}

	boundary := lic.GraceBoundary()
	if boundary != nil && now.After(*boundary) {
		return tokenSnapshot{}, ErrLicenseExpired
	}

	status := license.TokenStatusActive
	switch {
	case lic.Status == license.StatusSuspended:
		status = license.TokenStatusSuspended
	case lic.ExpiresAt != nil && now.After(*lic.ExpiresAt):
		status = license.TokenStatusGrace
	}

	licensePlan, err := s.planRepo.FindByID(ctx, input.ProductID, lic.PlanID)
	if err != nil {
		logger.Error().Str("plan_id", lic.PlanID).Err(err).Msg("failed to find license plan")
		return tokenSnapshot{}, err
	}
	if licensePlan == nil {
		logger.Error().Str("plan_id", lic.PlanID).Msg("license references missing plan")
		return tokenSnapshot{}, fault.ErrUnexpected
	}

	tokenTTL := lic.TokenTTLSeconds
	if tokenTTL <= 0 {
		tokenTTL = license.DefaultTokenTTLSeconds
	}

	return tokenSnapshot{
		planKey:         licensePlan.Key,
		status:          status,
		entitlements:    lic.ResolvedEntitlements(licensePlan.Entitlements),
		graceUntil:      lic.GraceUntil,
		tokenTTLSeconds: tokenTTL,
	}, nil
}

func (s *licenseService) ListSigningKeys(
	ctx context.Context, input license.ListSigningKeysInput,
) ([]license.SigningKey, error) {
	logger := s.logger.With().Str("operation", "ListSigningKeys").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	keys, err := s.signingKeyRepo.ListAll(ctx)
	if err != nil {
		logger.Error().Err(err).Msg("failed to list license signing keys")
		return nil, err
	}

	return keys, nil
}
