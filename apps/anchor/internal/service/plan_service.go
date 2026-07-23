package service

import (
	"context"
	"time"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/rs/zerolog"

	"anchor/internal/domain/plan"
	"anchor/internal/repository"
)

type PlanService interface {
	Create(ctx context.Context, input plan.CreatePlanInput) (plan.Plan, error)
	Update(ctx context.Context, input plan.UpdatePlanInput) (plan.Plan, error)
	Get(ctx context.Context, input plan.GetPlanInput) (*plan.Plan, error)
	List(ctx context.Context, input plan.ListPlansInput) ([]plan.Plan, error)
	Delete(ctx context.Context, input plan.DeletePlanInput) error
}

type planService struct {
	planRepo    repository.PlanRepository
	licenseRepo repository.LicenseRepository
	transactor  transactor.Transactor
	logger      zerolog.Logger
}

func NewPlanService(
	planRepo repository.PlanRepository,
	licenseRepo repository.LicenseRepository,
	transactor transactor.Transactor,
	logger zerolog.Logger,
) PlanService {
	return &planService{
		planRepo:    planRepo,
		licenseRepo: licenseRepo,
		transactor:  transactor,
		logger:      logger.With().Str("component", "plan_service").Logger(),
	}
}

func (s *planService) Create(
	ctx context.Context, input plan.CreatePlanInput,
) (plan.Plan, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := validateStruct(input); err != nil {
		return plan.Plan{}, err
	}
	entitlements, err := normalizeEntitlements(input.Entitlements)
	if err != nil {
		return plan.Plan{}, err
	}

	existing, err := s.planRepo.FindByKey(ctx, input.ProductID, input.Key)
	if err != nil {
		logger.Error().Str("plan_key", input.Key).Err(err).Msg("failed to look up plan by key")
		return plan.Plan{}, err
	}
	if existing != nil {
		return plan.Plan{}, NewPlanKeyExistsError(input.Key, input.ProductID)
	}

	newPlan := plan.Plan{
		ProductID:    input.ProductID,
		Key:          input.Key,
		Name:         input.Name,
		Description:  input.Description,
		Entitlements: entitlements,
		IsDefault:    input.IsDefault,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	newPlan.GenerateID()

	var created plan.Plan
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if newPlan.IsDefault {
			if clearErr := s.planRepo.ClearDefaultExcept(
				txCtx, input.ProductID, newPlan.ID,
			); clearErr != nil {
				logger.Error().Err(clearErr).Msg("failed to clear previous default plan")
				return clearErr
			}
		}

		var createErr error
		created, createErr = s.planRepo.Create(txCtx, newPlan)
		if createErr != nil {
			logger.Error().Str("plan_key", input.Key).Err(createErr).Msg("failed to create plan")
			return createErr
		}

		logger.Info().Str("plan_id", created.ID).Str("plan_key", created.Key).Msg("plan created")
		return nil
	})

	return created, err
}

func (s *planService) Update(
	ctx context.Context, input plan.UpdatePlanInput,
) (plan.Plan, error) {
	logger := s.logger.With().Str("operation", "Update").Logger()

	if err := validateStruct(input); err != nil {
		return plan.Plan{}, err
	}

	var entitlements *plan.Entitlements
	if input.Entitlements != nil {
		normalized, err := normalizeEntitlements(*input.Entitlements)
		if err != nil {
			return plan.Plan{}, err
		}
		entitlements = &normalized
	}

	var updated plan.Plan
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.planRepo.FindByID(txCtx, input.ProductID, input.PlanID)
		if findErr != nil {
			logger.Error().Str("plan_id", input.PlanID).Err(findErr).Msg("failed to find plan")
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}

		updatedPlan := *existing
		if input.Name != nil {
			updatedPlan.Name = *input.Name
		}
		if input.Description != nil {
			updatedPlan.Description = *input.Description
		}
		if entitlements != nil {
			updatedPlan.Entitlements = *entitlements
		}
		if input.IsDefault != nil {
			updatedPlan.IsDefault = *input.IsDefault
		}

		if updatedPlan.IsDefault {
			if clearErr := s.planRepo.ClearDefaultExcept(
				txCtx, input.ProductID, updatedPlan.ID,
			); clearErr != nil {
				logger.Error().Err(clearErr).Msg("failed to clear previous default plan")
				return clearErr
			}
		}

		var updateErr error
		updated, updateErr = s.planRepo.Update(txCtx, input.ProductID, updatedPlan)
		if updateErr != nil {
			logger.Error().Str("plan_id", input.PlanID).Err(updateErr).Msg("failed to update plan")
			return updateErr
		}

		logger.Info().Str("plan_id", input.PlanID).Msg("plan updated")
		return nil
	})

	return updated, err
}

func (s *planService) Get(
	ctx context.Context, input plan.GetPlanInput,
) (*plan.Plan, error) {
	logger := s.logger.With().Str("operation", "Get").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.planRepo.FindByID(ctx, input.ProductID, input.PlanID)
	if err != nil {
		logger.Error().Str("plan_id", input.PlanID).Err(err).Msg("failed to find plan")
		return nil, err
	}

	return found, nil
}

func (s *planService) List(
	ctx context.Context, input plan.ListPlansInput,
) ([]plan.Plan, error) {
	logger := s.logger.With().Str("operation", "List").Logger()

	if err := validateStruct(input); err != nil {
		return nil, err
	}

	plans, err := s.planRepo.ListByProduct(ctx, input.ProductID)
	if err != nil {
		logger.Error().Str("product_id", input.ProductID).Err(err).Msg("failed to list plans")
		return nil, err
	}

	return plans, nil
}

func (s *planService) Delete(ctx context.Context, input plan.DeletePlanInput) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := validateStruct(input); err != nil {
		return err
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existing, findErr := s.planRepo.FindByID(txCtx, input.ProductID, input.PlanID)
		if findErr != nil {
			logger.Error().Str("plan_id", input.PlanID).Err(findErr).Msg("failed to find plan")
			return findErr
		}
		if existing == nil {
			return fault.ErrNotFound
		}

		licenseCount, countErr := s.licenseRepo.CountByPlan(
			txCtx, input.ProductID, input.PlanID,
		)
		if countErr != nil {
			logger.Error().Str("plan_id", input.PlanID).Err(countErr).Msg("failed to count plan licenses")
			return countErr
		}
		if licenseCount > 0 {
			return NewPlanInUseError(input.PlanID, licenseCount)
		}

		if deleteErr := s.planRepo.DeleteByID(
			txCtx, input.ProductID, input.PlanID,
		); deleteErr != nil {
			logger.Error().Str("plan_id", input.PlanID).Err(deleteErr).Msg("failed to delete plan")
			return deleteErr
		}

		logger.Info().Str("plan_id", input.PlanID).Msg("plan deleted")
		return nil
	})
}

// normalizeEntitlements coerces numeric values and runs domain validation,
// mapping failures to a 400 fault.
func normalizeEntitlements(entitlements plan.Entitlements) (plan.Entitlements, error) {
	if entitlements == nil {
		return plan.Entitlements{}, nil
	}

	normalized := entitlements.Normalize()
	if err := normalized.Validate(); err != nil {
		return nil, NewInvalidEntitlementsError(err)
	}

	return normalized, nil
}
