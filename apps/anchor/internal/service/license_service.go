package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/rs/zerolog"

	"anchor/internal/domain/license"
	"anchor/internal/domain/plan"
	"anchor/internal/domain/webhook"
	"anchor/internal/repository"
)

type LicenseService interface {
	List(ctx context.Context, input license.ListLicensesInput) ([]license.License, error)
	Get(ctx context.Context, input license.GetLicenseInput) (*license.License, error)
	// Put assigns a license to the organization or fully replaces the existing
	// one (plan, expiry, grace, overrides, refresh interval).
	Put(ctx context.Context, input license.PutLicenseInput) (license.License, error)
	Revoke(ctx context.Context, input license.RevokeLicenseInput) (license.License, error)
	Suspend(ctx context.Context, input license.SuspendLicenseInput) (license.License, error)
	Reinstate(ctx context.Context, input license.ReinstateLicenseInput) (license.License, error)
	// GetEntitlements resolves the organization's license into an entitlement
	// snapshot (plan entitlements merged with per-org overrides, effective
	// status, refresh hint). Organizations without a license row fall back to
	// the product's default plan when one exists.
	GetEntitlements(
		ctx context.Context, input license.GetEntitlementsInput,
	) (license.EntitlementSnapshot, error)
}

type licenseService struct {
	licenseRepo repository.LicenseRepository
	planRepo    repository.PlanRepository
	orgRepo     repository.OrganizationRepository
	emitter     WebhookEmitter
	transactor  transactor.Transactor
	logger      zerolog.Logger
}

func NewLicenseService(
	licenseRepo repository.LicenseRepository,
	planRepo repository.PlanRepository,
	orgRepo repository.OrganizationRepository,
	emitter WebhookEmitter,
	transactor transactor.Transactor,
	logger zerolog.Logger,
) LicenseService {
	return &licenseService{
		licenseRepo: licenseRepo,
		planRepo:    planRepo,
		orgRepo:     orgRepo,
		emitter:     emitter,
		transactor:  transactor,
		logger:      logger.With().Str("component", "license_service").Logger(),
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

	refreshInterval := license.DefaultRefreshIntervalSeconds
	if input.RefreshIntervalSeconds != nil {
		refreshInterval = *input.RefreshIntervalSeconds
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
				ProductID:              input.ProductID,
				OrganizationID:         input.OrganizationID,
				PlanID:                 input.PlanID,
				Status:                 license.StatusActive,
				ExpiresAt:              input.ExpiresAt,
				GraceUntil:             input.GraceUntil,
				EntitlementOverrides:   overrides,
				RefreshIntervalSeconds: refreshInterval,
				CreatedAt:              time.Now(),
				UpdatedAt:              time.Now(),
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

			// Emitted on the business transaction: the event exists if and
			// only if the license write commits.
			return s.emitLicenseEvent(
				txCtx, webhook.EventTypeLicenseCreated, result, targetPlan.Key, nil,
			)
		}

		updatedLicense := *existing
		updatedLicense.PlanID = input.PlanID
		updatedLicense.ExpiresAt = input.ExpiresAt
		updatedLicense.GraceUntil = input.GraceUntil
		updatedLicense.EntitlementOverrides = overrides
		updatedLicense.RefreshIntervalSeconds = refreshInterval
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

		return s.emitLicenseEvent(
			txCtx,
			webhook.EventTypeLicenseUpdated,
			result,
			targetPlan.Key,
			licenseChanges(*existing, result),
		)
	})

	return result, err
}

// licenseChanges reports the fields that actually moved, so a receiver can
// decide whether it cares without diffing the resource itself.
func licenseChanges(previous, next license.License) map[string]webhook.LicenseChange {
	changes := make(map[string]webhook.LicenseChange)
	if previous.Status != next.Status {
		changes["status"] = webhook.LicenseChange{
			Previous: string(previous.Status), New: string(next.Status),
		}
	}
	if previous.PlanID != next.PlanID {
		changes["plan_id"] = webhook.LicenseChange{
			Previous: previous.PlanID, New: next.PlanID,
		}
	}
	if !equalTimePtr(previous.ExpiresAt, next.ExpiresAt) {
		changes["expires_at"] = webhook.LicenseChange{
			Previous: formatTimePtr(previous.ExpiresAt), New: formatTimePtr(next.ExpiresAt),
		}
	}
	if !equalTimePtr(previous.GraceUntil, next.GraceUntil) {
		changes["grace_until"] = webhook.LicenseChange{
			Previous: formatTimePtr(previous.GraceUntil), New: formatTimePtr(next.GraceUntil),
		}
	}
	if len(changes) == 0 {
		return nil
	}

	return changes
}

func equalTimePtr(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Equal(*b)
	}
}

func formatTimePtr(value *time.Time) any {
	if value == nil {
		return nil
	}

	return value.UTC().Format(time.RFC3339)
}

// emitLicenseEvent publishes a license event on the caller's transaction. A
// failure to emit fails the business write: silently losing an event would let
// a consumer serve a revoked license until its next poll.
func (s *licenseService) emitLicenseEvent(
	ctx context.Context,
	eventType string,
	lic license.License,
	planKey string,
	changes map[string]webhook.LicenseChange,
) error {
	organizationID := lic.OrganizationID
	_, err := s.emitter.Emit(ctx, webhook.EmitInput{
		ProductID:      lic.ProductID,
		OrganizationID: &organizationID,
		EventType:      eventType,
		Data: webhook.LicenseEventData{
			LicenseID: lic.ID,
			PlanID:    lic.PlanID,
			PlanKey:   planKey,
			Status:    string(lic.Status),
			Changes:   changes,
		},
	})

	return err
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

		// Single emit point for revoke, suspend and reinstate. Revocation gets
		// its own event type because it is the one transition a consumer must
		// react to immediately; suspend and reinstate are ordinary updates
		// carrying the status transition in `changes`.
		eventType := webhook.EventTypeLicenseUpdated
		if status == license.StatusRevoked {
			eventType = webhook.EventTypeLicenseRevoked
		}

		licensePlan, planErr := s.planRepo.FindByID(txCtx, productID, result.PlanID)
		if planErr != nil {
			return planErr
		}
		planKey := ""
		if licensePlan != nil {
			planKey = licensePlan.Key
		}

		return s.emitLicenseEvent(
			txCtx, eventType, result, planKey, licenseChanges(*existing, result),
		)
	})

	return result, err
}

func (s *licenseService) GetEntitlements(
	ctx context.Context, input license.GetEntitlementsInput,
) (license.EntitlementSnapshot, error) {
	logger := s.logger.With().Str("operation", "GetEntitlements").Logger()

	if err := validateStruct(input); err != nil {
		return license.EntitlementSnapshot{}, err
	}

	organization, err := s.orgRepo.FindByID(ctx, input.ProductID, input.OrganizationID)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to find organization")
		return license.EntitlementSnapshot{}, err
	}
	if organization == nil {
		return license.EntitlementSnapshot{}, fault.ErrNotFound
	}

	lic, err := s.licenseRepo.FindByOrganization(
		ctx, input.ProductID, input.OrganizationID,
	)
	if err != nil {
		logger.Error().Str("organization_id", input.OrganizationID).Err(err).
			Msg("failed to find license")
		return license.EntitlementSnapshot{}, err
	}

	now := time.Now()
	resolved, err := s.resolveEntitlements(ctx, input, lic, now, logger)
	if err != nil {
		return license.EntitlementSnapshot{}, err
	}

	return license.EntitlementSnapshot{
		OrganizationID: input.OrganizationID,
		ProductID:      input.ProductID,
		PlanKey:        resolved.planKey,
		Status:         resolved.status,
		Entitlements:   resolved.entitlements,
		ExpiresAt:      resolved.expiresAt,
		GraceUntil:     resolved.graceUntil,
		RefreshAfter: now.Add(
			time.Duration(resolved.refreshIntervalSeconds) * time.Second,
		),
	}, nil
}

// resolvedLicense is the license state a snapshot is built from.
type resolvedLicense struct {
	planKey                string
	status                 license.EffectiveStatus
	entitlements           plan.Entitlements
	expiresAt              *time.Time
	graceUntil             *time.Time
	refreshIntervalSeconds int32
}

// resolveEntitlements applies the read semantics: default-plan fallback for
// organizations without a license row; REVOKED or past the grace boundary is a
// 409; SUSPENDED still resolves (status SUSPENDED); past expiry but within
// grace resolves with status GRACE.
func (s *licenseService) resolveEntitlements(
	ctx context.Context,
	input license.GetEntitlementsInput,
	lic *license.License,
	now time.Time,
	logger zerolog.Logger,
) (resolvedLicense, error) {
	if lic == nil {
		defaultPlan, err := s.planRepo.FindDefault(ctx, input.ProductID)
		if err != nil {
			logger.Error().Str("product_id", input.ProductID).Err(err).
				Msg("failed to find default plan")
			return resolvedLicense{}, err
		}
		if defaultPlan == nil {
			return resolvedLicense{}, NewLicenseNotFoundError(input.OrganizationID)
		}

		return resolvedLicense{
			planKey:                defaultPlan.Key,
			status:                 license.EffectiveStatusActive,
			entitlements:           defaultPlan.Entitlements,
			refreshIntervalSeconds: license.DefaultRefreshIntervalSeconds,
		}, nil
	}

	if lic.Status == license.StatusRevoked {
		return resolvedLicense{}, ErrLicenseRevoked
	}

	boundary := lic.GraceBoundary()
	if boundary != nil && now.After(*boundary) {
		return resolvedLicense{}, ErrLicenseExpired
	}

	status := license.EffectiveStatusActive
	switch {
	case lic.Status == license.StatusSuspended:
		status = license.EffectiveStatusSuspended
	case lic.ExpiresAt != nil && now.After(*lic.ExpiresAt):
		status = license.EffectiveStatusGrace
	}

	licensePlan, err := s.planRepo.FindByID(ctx, input.ProductID, lic.PlanID)
	if err != nil {
		logger.Error().Str("plan_id", lic.PlanID).Err(err).Msg("failed to find license plan")
		return resolvedLicense{}, err
	}
	if licensePlan == nil {
		logger.Error().Str("plan_id", lic.PlanID).Msg("license references missing plan")
		return resolvedLicense{}, fault.ErrUnexpected
	}

	refreshInterval := lic.RefreshIntervalSeconds
	if refreshInterval <= 0 {
		refreshInterval = license.DefaultRefreshIntervalSeconds
	}

	return resolvedLicense{
		planKey:                licensePlan.Key,
		status:                 status,
		entitlements:           lic.ResolvedEntitlements(licensePlan.Entitlements),
		expiresAt:              lic.ExpiresAt,
		graceUntil:             lic.GraceUntil,
		refreshIntervalSeconds: refreshInterval,
	}, nil
}
