package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/product/user"
	"anchor/internal/events"

	"anchor/internal/repository"
	assignments "anchor/internal/roleassignment/service"

	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
)

type ProductUserService interface {
	Find(ctx context.Context, input user.FindProductUserInput) (user.ProductUser, error)
	FindByExternalID(ctx context.Context, input user.FindProductUserByExternalIDInput) (*user.ProductUser, error)
	Create(ctx context.Context, input user.CreateProductUserInput) (user.ProductUser, error)
	Delete(ctx context.Context, input user.DeleteProductUserInput) error
	Search(
		ctx context.Context, q repository.ProductUserSearch,
	) (search.Result[user.ProductUser], error)
	ListUserOrganizations(
		ctx context.Context, input user.ListUserOrganizationsInput,
	) ([]user.OrganizationMembership, error)
	// SearchUserOrganizations reads one page of the organizations a product user
	// belongs to. includePermissions loads the permissions of each role.
	SearchUserOrganizations(
		ctx context.Context, q repository.UserOrganizationSearch, includePermissions bool,
	) (search.Result[user.OrganizationMembership], error)
	GetUserOrganization(
		ctx context.Context, input user.GetUserOrganizationInput,
	) (user.OrganizationMembership, error)
}

var ErrProductUserEmailAlreadyExists = fault.Conflict(
	"PRODUCT_USER_EMAIL_ALREADY_EXISTS",
	"A product user with this email already exists in this product",
)

var ErrProductUserExternalIDAlreadyExists = fault.Conflict(
	"PRODUCT_USER_EXTERNAL_ID_ALREADY_EXISTS",
	"A product user with this external ID already exists in this product",
)

type productUserService struct {
	productUserRepo   repository.ProductUserRepository
	orgMembershipRepo repository.OrganizationMembershipRepository
	roleAssignments   assignments.Service
	transactor        transactor.Transactor
	events            events.Emitter
	logger            *slog.Logger
}

func NewProductUserService(
	productUserRepo repository.ProductUserRepository,
	orgMembershipRepo repository.OrganizationMembershipRepository,
	roleAssignments assignments.Service,
	transactor transactor.Transactor,
	eventEmitter events.Emitter,
	logger *slog.Logger,
) ProductUserService {
	return &productUserService{
		productUserRepo:   productUserRepo,
		orgMembershipRepo: orgMembershipRepo,
		roleAssignments:   roleAssignments,
		transactor:        transactor,
		events:            eventEmitter,
		logger:            logger.With(slog.String("component", "product_user_service")),
	}
}

func (s *productUserService) Find(
	ctx context.Context, input user.FindProductUserInput,
) (user.ProductUser, error) {
	logger := s.logger.With(slog.String("operation", "Find"))

	if err := validate.ValidateStruct(input); err != nil {
		return user.ProductUser{}, err
	}

	found, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find product user",
			slog.String("product_id", input.ProductID),
			slog.String("product_user_id", input.ProductUserID),
			log.Err(err),
		)
		return user.ProductUser{}, err
	}

	return found.ToResult(user.ErrProductUserNotFound).Value()
}

func (s *productUserService) Create(
	ctx context.Context, input user.CreateProductUserInput,
) (user.ProductUser, error) {
	logger := s.logger.With(slog.String("operation", "Create"))

	if err := validate.ValidateStruct(input); err != nil {
		return user.ProductUser{}, err
	}

	var createdUser user.ProductUser
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		existingUsers, err := s.productUserRepo.FindByProductID(txCtx, input.ProductID)
		if err != nil {
			logger.ErrorContext(txCtx, "failed to check for duplicate email",
				slog.String("product_id", input.ProductID),
				slog.String("email", input.Email),
				log.Err(err),
			)
			return err
		}

		emailExists := functional.Slice(existingUsers).AnyMatch(func(existingUser user.ProductUser) bool {
			return existingUser.Email == input.Email
		})
		if emailExists {
			return ErrProductUserEmailAlreadyExists
		}

		if err = s.ensureExternalIDIsFree(txCtx, input.ProductID, input.ExternalID); err != nil {
			return err
		}

		productUser := user.ProductUser{
			ProductID:  input.ProductID,
			Email:      input.Email,
			Name:       functional.FromPtr(input.Name).OrElse(""),
			ExternalID: input.ExternalID,
			Status:     input.Status,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		productUser.GenerateID()

		createdUser, err = s.productUserRepo.Create(txCtx, productUser)
		if errors.Is(err, repository.ErrProductUserEmailExists) {
			return ErrProductUserEmailAlreadyExists
		}
		if errors.Is(err, repository.ErrProductUserExternalIDExists) {
			return ErrProductUserExternalIDAlreadyExists
		}
		if err != nil {
			logger.ErrorContext(txCtx, "failed to create product user",
				slog.String("product_id", input.ProductID),
				slog.String("email", input.Email),
				log.Err(err),
			)
			return err
		}

		logger.InfoContext(txCtx, "product user created successfully",
			slog.String("product_user_id", createdUser.ID),
			slog.String("product_id", input.ProductID),
			slog.String("email", input.Email),
		)

		return s.events.Emit(txCtx, events.Event{
			Type:      events.ProductUserCreated,
			ProductID: input.ProductID,
			Data:      events.Data{events.FieldProductUserID: createdUser.ID},
		})
	})
	return createdUser, err
}

func (s *productUserService) ensureExternalIDIsFree(
	ctx context.Context, productID string, externalID *string,
) error {
	if externalID == nil {
		return nil
	}
	holder, err := s.productUserRepo.FindByExternalID(ctx, productID, *externalID)
	if err != nil {
		return err
	}
	if holder.IsPresent() {
		return ErrProductUserExternalIDAlreadyExists
	}
	return nil
}

func (s *productUserService) Delete(
	ctx context.Context, input user.DeleteProductUserInput,
) error {
	logger := s.logger.With(slog.String("operation", "Delete"))

	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	var deleted bool
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		// Its role assignments on resources go with its memberships through
		// the foreign keys, each with its event.
		if releaseErr := s.roleAssignments.ReleaseUser(txCtx, input.ProductID, input.ProductUserID); releaseErr != nil {
			return releaseErr
		}
		var delErr error
		deleted, delErr = s.productUserRepo.DeleteByProductIDAndID(txCtx, input.ProductID, input.ProductUserID)
		if delErr != nil {
			return delErr
		}
		if !deleted {
			return nil
		}
		return s.events.Emit(txCtx, events.Event{
			Type:      events.ProductUserDeleted,
			ProductID: input.ProductID,
			Data:      events.Data{events.FieldProductUserID: input.ProductUserID},
		})
	})
	if err != nil {
		logger.ErrorContext(ctx, "failed to delete product user",
			slog.String("product_id", input.ProductID),
			slog.String("product_user_id", input.ProductUserID),
			log.Err(err),
		)
		return err
	}

	if !deleted {
		logger.DebugContext(ctx, "product user already absent, nothing deleted",
			slog.String("product_user_id", input.ProductUserID),
			slog.String("product_id", input.ProductID),
		)
		return nil
	}

	logger.InfoContext(ctx, "product user deleted successfully",
		slog.String("product_user_id", input.ProductUserID),
		slog.String("product_id", input.ProductID),
	)

	return nil
}

func (s *productUserService) Search(
	ctx context.Context, q repository.ProductUserSearch,
) (search.Result[user.ProductUser], error) {
	logger := s.logger.With(slog.String("operation", "Search"))

	result, err := s.productUserRepo.SearchByProductID(ctx, q)
	if err != nil {
		if !search.IsClientError(err) {
			logger.ErrorContext(ctx, "failed to search product users",
				slog.String("product_id", q.Scope()),
				log.Err(err),
			)
		}
		return search.Result[user.ProductUser]{}, err
	}

	logger.DebugContext(ctx, "product users search completed",
		slog.String("product_id", q.Scope()),
		slog.Int("total_count", int(result.Total)),
	)

	return result, nil
}

func (s *productUserService) FindByExternalID(
	ctx context.Context, input user.FindProductUserByExternalIDInput,
) (*user.ProductUser, error) {
	logger := s.logger.With(slog.String("operation", "FindByExternalID"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}

	found, err := s.productUserRepo.FindByExternalID(
		ctx, input.ProductID, input.ExternalID,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to find product user by external ID",
			slog.String("product_id", input.ProductID),
			slog.String("external_id", input.ExternalID),
			log.Err(err),
		)
		return nil, err
	}

	return found.ToPtr(), nil
}

func (s *productUserService) ListUserOrganizations(
	ctx context.Context, input user.ListUserOrganizationsInput,
) ([]user.OrganizationMembership, error) {
	logger := s.logger.With(slog.String("operation", "ListUserOrganizations"))

	if err := validate.ValidateStruct(input); err != nil {
		return nil, err
	}

	// Verify the product user exists
	foundUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify product user exists",
			slog.String("product_id", input.ProductID),
			slog.String("product_user_id", input.ProductUserID),
			log.Err(err),
		)
		return nil, err
	}
	if foundUser.IsAbsent() {
		return nil, user.ErrProductUserNotFound
	}

	memberships, err := s.orgMembershipRepo.FindByProductUserID(
		ctx, input.ProductID, input.ProductUserID, input.IncludePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to list user organizations",
			slog.String("product_id", input.ProductID),
			slog.String("product_user_id", input.ProductUserID),
			log.Err(err),
		)
		return nil, err
	}

	return memberships, nil
}

func (s *productUserService) SearchUserOrganizations(
	ctx context.Context, q repository.UserOrganizationSearch, includePermissions bool,
) (search.Result[user.OrganizationMembership], error) {
	logger := s.logger.With(slog.String("operation", "SearchUserOrganizations"))
	scope := q.Scope()

	foundUser, err := s.productUserRepo.FindByProductIDAndID(ctx, scope.ProductID, scope.ProductUserID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify product user exists",
			slog.String("product_id", scope.ProductID),
			slog.String("product_user_id", scope.ProductUserID),
			log.Err(err),
		)
		return search.Result[user.OrganizationMembership]{}, err
	}
	if foundUser.IsAbsent() {
		return search.Result[user.OrganizationMembership]{}, user.ErrProductUserNotFound
	}

	result, err := s.orgMembershipRepo.SearchByProductUserID(ctx, q)
	if err != nil {
		if !search.IsClientError(err) {
			logger.ErrorContext(ctx, "failed to search user organizations",
				slog.String("product_id", scope.ProductID),
				slog.String("product_user_id", scope.ProductUserID),
				log.Err(err),
			)
		}
		return search.Result[user.OrganizationMembership]{}, err
	}
	if includePermissions {
		if err := s.attachRolePermissions(ctx, result.Items); err != nil {
			return search.Result[user.OrganizationMembership]{}, err
		}
	}

	return result, nil
}

// attachRolePermissions reads the permissions of the roles that the memberships
// hold, in one statement, and sets them on each membership.
func (s *productUserService) attachRolePermissions(
	ctx context.Context, items []user.OrganizationMembership,
) error {
	var roleIDs []string
	for _, item := range items {
		if item.Role != nil {
			roleIDs = append(roleIDs, item.Role.ID)
		}
	}
	if len(roleIDs) == 0 {
		return nil
	}
	permissions, err := s.orgMembershipRepo.RolePermissionsByRoleID(ctx, roleIDs)
	if err != nil {
		return err
	}
	for i := range items {
		if held := items[i].Role; held != nil {
			items[i].RolePermissions = permissions[held.ID]
		}
	}
	return nil
}

func (s *productUserService) GetUserOrganization(
	ctx context.Context, input user.GetUserOrganizationInput,
) (user.OrganizationMembership, error) {
	logger := s.logger.With(slog.String("operation", "GetUserOrganization"))

	if err := validate.ValidateStruct(input); err != nil {
		return user.OrganizationMembership{}, err
	}

	// Verify the product user exists
	foundUser, err := s.productUserRepo.FindByProductIDAndID(
		ctx, input.ProductID, input.ProductUserID,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to verify product user exists",
			slog.String("product_id", input.ProductID),
			slog.String("product_user_id", input.ProductUserID),
			log.Err(err),
		)
		return user.OrganizationMembership{}, err
	}
	if foundUser.IsAbsent() {
		return user.OrganizationMembership{}, user.ErrProductUserNotFound
	}

	found, err := s.orgMembershipRepo.FindByProductUserIDAndOrgID(
		ctx, input.ProductID, input.ProductUserID, input.OrganizationID, input.IncludePermissions,
	)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get user organization",
			slog.String("product_id", input.ProductID),
			slog.String("product_user_id", input.ProductUserID),
			slog.String("organization_id", input.OrganizationID),
			log.Err(err),
		)
		return user.OrganizationMembership{}, err
	}

	return found.ToResult(organization.ErrMembershipNotFound).Value()
}
