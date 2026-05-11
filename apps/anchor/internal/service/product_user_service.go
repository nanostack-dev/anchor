package service

import (
	"context"
	"database/sql"
	"time"

	"anchor/internal/domain/product/user"

	"anchor/internal/repository"

	"github.com/nanostack-dev/shared/toolkit"
	"github.com/nanostack-dev/shared/toolkit/search"

	"github.com/rs/zerolog"
)

type ProductUserService interface {
	Find(ctx context.Context, input user.FindProductUserInput) (*user.ProductUser, error)
	FindByExternalID(ctx context.Context, input user.FindProductUserByExternalIDInput) (*user.ProductUser, error)
	Create(ctx context.Context, input user.CreateProductUserInput) (user.ProductUser, error)
	Delete(ctx context.Context, input user.DeleteProductUserInput) error
	Search(
		ctx context.Context, input user.SearchProductUserInput,
	) (search.Result[user.ProductUser], error)
	ListUserOrganizations(
		ctx context.Context, input user.ListUserOrganizationsInput,
	) ([]user.OrganizationMembership, error)
	GetUserOrganization(
		ctx context.Context, input user.GetUserOrganizationInput,
	) (*user.OrganizationMembership, error)
}

var ErrProductUserEmailAlreadyExists = toolkit.NewNanostackBadRequestError(
	"PRODUCT_USER_EMAIL_ALREADY_EXISTS",
	"A product user with this email already exists in this product",
)

type productUserService struct {
	productUserRepo   repository.ProductUserRepository
	orgMembershipRepo repository.OrganizationMembershipRepository
	db                *sql.DB
	logger            zerolog.Logger
}

func NewProductUserService(
	productUserRepo repository.ProductUserRepository,
	orgMembershipRepo repository.OrganizationMembershipRepository,
	db *sql.DB,
	logger zerolog.Logger,
) ProductUserService {
	return &productUserService{
		productUserRepo:   productUserRepo,
		orgMembershipRepo: orgMembershipRepo,
		db:                db,
		logger:            logger.With().Str("component", "product_user_service").Logger(),
	}
}

func (s *productUserService) Find(
	ctx context.Context, input user.FindProductUserInput,
) (*user.ProductUser, error) {
	logger := s.logger.With().Str("operation", "Find").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	productUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to find product user")
		return nil, err
	}

	return productUser, nil
}

func (s *productUserService) Create(
	ctx context.Context, input user.CreateProductUserInput,
) (user.ProductUser, error) {
	logger := s.logger.With().Str("operation", "Create").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return user.ProductUser{}, err
	}

	return toolkit.WithTxReturn(
		s.db, func(tx *sql.Tx) (user.ProductUser, error) {
			txOptions := &toolkit.DBOptions{Tx: tx}

			existingUsers, err := s.productUserRepo.FindByProductID(ctx, input.ProductID, txOptions)
			if err != nil {
				logger.Error().
					Str("product_id", input.ProductID).
					Str("email", input.Email).
					Err(err).
					Msg("failed to check for duplicate email")
				return user.ProductUser{}, err
			}

			for _, existingUser := range existingUsers {
				if existingUser.Email == input.Email {
					return user.ProductUser{}, ErrProductUserEmailAlreadyExists
				}
			}

			productUser := user.ProductUser{
				ProductID: input.ProductID,
				Email:     input.Email,
				Name:      input.Name,
				Status:    input.Status,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			productUser.GenerateID()

			createdUser, err := s.productUserRepo.Create(ctx, productUser, txOptions)
			if err != nil {
				logger.Error().
					Str("product_id", input.ProductID).
					Str("email", input.Email).
					Err(err).
					Msg("failed to create product user")
				return user.ProductUser{}, err
			}

			logger.Info().
				Str("product_user_id", createdUser.ID).
				Str("product_id", input.ProductID).
				Str("email", input.Email).
				Msg("product user created successfully")

			return createdUser, nil
		},
	)
}

func (s *productUserService) Delete(
	ctx context.Context, input user.DeleteProductUserInput,
) error {
	logger := s.logger.With().Str("operation", "Delete").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return err
	}

	err := s.productUserRepo.DeleteByID(ctx, input.ProductID, input.ProductUserID, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to delete product user")
		return err
	}

	logger.Info().
		Str("product_user_id", input.ProductUserID).
		Str("product_id", input.ProductID).
		Msg("product user deleted successfully")

	return nil
}

func (s *productUserService) Search(
	ctx context.Context, input user.SearchProductUserInput,
) (search.Result[user.ProductUser], error) {
	logger := s.logger.With().Str("operation", "Search").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return search.Result[user.ProductUser]{}, err
	}

	result, err := s.productUserRepo.SearchByProductID(ctx, input.ProductID, input.Request, nil)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Err(err).
			Msg("failed to search product users")
		return search.Result[user.ProductUser]{}, err
	}

	logger.Debug().
		Str("product_id", input.ProductID).
		Int("total_count", int(result.Total)).
		Msg("product users search completed")

	return result, nil
}

func (s *productUserService) FindByExternalID(
	ctx context.Context, input user.FindProductUserByExternalIDInput,
) (*user.ProductUser, error) {
	logger := s.logger.With().Str("operation", "FindByExternalID").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	productUser, err := s.productUserRepo.FindByExternalID(
		ctx, input.ProductID, input.ExternalID, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("external_id", input.ExternalID).
			Err(err).
			Msg("failed to find product user by external ID")
		return nil, err
	}

	return productUser, nil
}

func (s *productUserService) ListUserOrganizations(
	ctx context.Context, input user.ListUserOrganizationsInput,
) ([]user.OrganizationMembership, error) {
	logger := s.logger.With().Str("operation", "ListUserOrganizations").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	// Verify the product user exists
	existingUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to verify product user exists")
		return nil, err
	}
	if existingUser == nil {
		return nil, toolkit.ErrNotFound
	}

	memberships, err := s.orgMembershipRepo.FindByProductUserID(
		ctx, input.ProductID, input.ProductUserID, input.IncludePermissions, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to list user organizations")
		return nil, err
	}

	return memberships, nil
}

func (s *productUserService) GetUserOrganization(
	ctx context.Context, input user.GetUserOrganizationInput,
) (*user.OrganizationMembership, error) {
	logger := s.logger.With().Str("operation", "GetUserOrganization").Logger()

	if err := toolkit.ValidateStruct(input); err != nil {
		return nil, err
	}

	// Verify the product user exists
	existingUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Err(err).
			Msg("failed to verify product user exists")
		return nil, err
	}
	if existingUser == nil {
		return nil, toolkit.ErrNotFound
	}

	membership, err := s.orgMembershipRepo.FindByProductUserIDAndOrgID(
		ctx, input.ProductID, input.ProductUserID, input.OrganizationID, input.IncludePermissions, nil,
	)
	if err != nil {
		logger.Error().
			Str("product_id", input.ProductID).
			Str("product_user_id", input.ProductUserID).
			Str("organization_id", input.OrganizationID).
			Err(err).
			Msg("failed to get user organization")
		return nil, err
	}

	return membership, nil
}
