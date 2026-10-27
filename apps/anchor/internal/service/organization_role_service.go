package service

import (
	"context"
	"errors"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/organization"
	role "anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	invitationrepository "anchor/internal/invitation/repository"
	"anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"
)

// OrganizationRoleService serves an Organization's role routes: the roles its
// members may hold, and its own custom roles (ADR-0022 D8, D9, D10).
type OrganizationRoleService interface {
	SearchOrganizationRoles(
		ctx context.Context, input role.SearchOrganizationRolesInput,
	) (search.Result[role.ProductRole], error)
	GetOrganizationRole(ctx context.Context, input role.OrganizationRoleRef) (role.ProductRole, error)
	CreateOrganizationRole(
		ctx context.Context, input role.CreateOrganizationRoleInput,
	) (role.ProductRole, error)
	UpdateOrganizationRole(
		ctx context.Context, input role.UpdateOrganizationRoleInput,
	) (role.ProductRole, error)
	DeleteOrganizationRole(ctx context.Context, input role.OrganizationRoleRef) error
}

type organizationRoleService struct {
	*productRoleService
	orgRepo repository.OrganizationRepository
}

func NewOrganizationRoleService(
	roleRepo repository.ProductRoleRepository,
	productResourcePermissionRepo repository.ProductResourcePermissionRepository,
	invitationRepo invitationrepository.Repository,
	orgRepo repository.OrganizationRepository,
	resourceTypes rtservice.Service,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
	logger zerolog.Logger,
) OrganizationRoleService {
	roles := newProductRoleService(
		roleRepo, productResourcePermissionRepo, invitationRepo, resourceTypes, tx, eventEmitter, logger,
	)
	roles.logger = logger.With().Str("component", "organization_role_service").Logger()
	return &organizationRoleService{productRoleService: roles, orgRepo: orgRepo}
}

func (s *organizationRoleService) SearchOrganizationRoles(
	ctx context.Context, input role.SearchOrganizationRolesInput,
) (search.Result[role.ProductRole], error) {
	logger := s.logger.With().Str("operation", "SearchOrganizationRoles").Logger()
	if err := input.Validate(); err != nil {
		return search.Result[role.ProductRole]{}, err
	}
	if err := s.requireOrganization(ctx, input.ProductID, input.OrganizationID); err != nil {
		return search.Result[role.ProductRole]{}, err
	}
	run := func(
		ctx context.Context, productID string,
		request search.Request[role.SearchProductRoleFilter, role.SortFieldProductRole],
	) (search.Result[role.ProductRole], error) {
		return s.roleRepo.SearchInOrganization(ctx, productID, input.OrganizationID, request)
	}
	return scopedSearch(ctx, s.resourceTypes, logger, input.ProductID, input.Request,
		role.SearchProductRoleFilter.WithScopes, run, withRoleScope)
}

func (s *organizationRoleService) GetOrganizationRole(
	ctx context.Context, input role.OrganizationRoleRef,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "GetOrganizationRole").Logger()
	if err := validate.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	if err := s.requireOrganization(ctx, input.ProductID, input.OrganizationID); err != nil {
		return role.ProductRole{}, err
	}
	found, err := s.roleRepo.FindInOrganization(ctx, input.ProductID, input.OrganizationID, input.ID)
	if err != nil {
		return role.ProductRole{}, s.unexpected(logger, err, input.ProductID, "failed to get organization role")
	}
	if found.IsAbsent() {
		return role.ProductRole{}, role.ErrProductRoleNotFound
	}
	return s.withScope(ctx, input.ProductID, found.Value(), logger)
}

// CreateOrganizationRole counts the Organization's custom roles under its row
// lock, so two creates at 49 can't both pass the limit.
func (s *organizationRoleService) CreateOrganizationRole(
	ctx context.Context, input role.CreateOrganizationRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "CreateOrganizationRole").Logger()
	if err := validate.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	if input.ResourceType == "" {
		return role.ProductRole{}, resourcetype.ErrRequired
	}
	custom := role.NewCustom(input.ProductID, input.OrganizationID, input.Name, input.Description)
	permissions, err := s.resolveRolePermissions(
		ctx,
		input.ProductID,
		newRolePermissions(custom, input.Permissions),
		logger,
	)
	if err != nil {
		return role.ProductRole{}, err
	}
	custom.Permissions = permissions

	var created role.ProductRole
	err = s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, treeErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if treeErr != nil {
			return treeErr
		}
		if lockErr := s.requireOrganization(
			transactor.ForNoKeyUpdate(txCtx), input.ProductID, input.OrganizationID,
		); lockErr != nil {
			return lockErr
		}
		scope, known := tree.BySlug(input.ResourceType).Get()
		if !known {
			return resourcetype.NewUnknownError([]string{input.ResourceType})
		}
		if !scope.AllowsCustomRoles {
			return role.ErrCustomRolesDisabled
		}
		count, countErr := s.roleRepo.CountCustom(txCtx, input.ProductID, input.OrganizationID)
		if countErr != nil {
			return countErr
		}
		if count >= role.MaxCustomRoles {
			return role.ErrCustomRoleLimitReached
		}
		if nameErr := s.customNameAvailable(txCtx, custom); nameErr != nil {
			return nameErr
		}
		if scopeErr := s.checkPermissionScopes(
			txCtx, tree, input.ProductID, scope.Slug, custom.Permissions,
		); scopeErr != nil {
			return scopeErr
		}
		custom.ResourceTypeID = scope.ID
		var createErr error
		created, createErr = s.roleRepo.Create(txCtx, custom)
		if errors.Is(createErr, repository.ErrProductRoleNameExists) {
			return NewCustomRoleNameDuplicateError(input.Name, input.OrganizationID)
		}
		if createErr != nil {
			return createErr
		}
		created.ResourceType = scope.Slug
		return s.emitOrganizationRole(txCtx, events.OrganizationRoleCreated, created)
	})
	if err != nil {
		return role.ProductRole{}, s.unexpected(logger, err, input.ProductID, "failed to create custom role")
	}
	return created, nil
}

func (s *organizationRoleService) UpdateOrganizationRole(
	ctx context.Context, input role.UpdateOrganizationRoleInput,
) (role.ProductRole, error) {
	logger := s.logger.With().Str("operation", "UpdateOrganizationRole").Logger()
	if err := validate.ValidateStruct(input); err != nil {
		return role.ProductRole{}, err
	}
	var updated role.ProductRole
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, treeErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if treeErr != nil {
			return treeErr
		}
		current, findErr := s.editableCustomRole(
			txCtx, transactor.ForNoKeyUpdate, input.ProductID, input.OrganizationID, input.ID,
		)
		if findErr != nil {
			return findErr
		}
		current = withRoleScope(tree)(current)
		scope, _ := tree.ByID(current.ResourceTypeID).Get()
		if !scope.AllowsCustomRoles {
			return role.ErrCustomRolesDisabled
		}
		if input.ResourceType != nil && *input.ResourceType != current.ResourceType {
			return role.ErrScopeImmutable
		}
		changed := current
		changed.Name = *input.Name
		if input.Description != nil {
			changed.Description = *input.Description
		}
		if changed.Name != current.Name {
			if nameErr := s.customNameAvailable(txCtx, changed); nameErr != nil {
				return nameErr
			}
		}
		if input.Permissions != nil {
			permissions, resolveErr := s.resolveRolePermissions(txCtx, input.ProductID, input.Permissions, logger)
			if resolveErr != nil {
				return resolveErr
			}
			for i := range permissions {
				permissions[i].GenerateID()
			}
			if scopeErr := s.checkPermissionScopes(
				txCtx, tree, input.ProductID, current.ResourceType, permissions,
			); scopeErr != nil {
				return scopeErr
			}
			if replaceErr := s.roleRepo.ReplacePermissions(
				txCtx, input.ProductID, input.ID, permissions,
			); replaceErr != nil {
				return replaceErr
			}
		}
		var updateErr error
		updated, updateErr = s.roleRepo.Update(txCtx, changed)
		if errors.Is(updateErr, repository.ErrProductRoleNameExists) {
			return NewCustomRoleNameDuplicateError(changed.Name, input.OrganizationID)
		}
		if updateErr != nil {
			return updateErr
		}
		updated.ResourceType = current.ResourceType
		return s.emitOrganizationRole(txCtx, events.OrganizationRoleUpdated, updated)
	})
	if err != nil {
		return role.ProductRole{}, s.unexpected(logger, err, input.ProductID, "failed to update custom role")
	}
	return updated, nil
}

// DeleteOrganizationRole stays open while the switch is off, so an
// Organization can still clean up the custom roles it no longer may change.
func (s *organizationRoleService) DeleteOrganizationRole(
	ctx context.Context, input role.OrganizationRoleRef,
) error {
	logger := s.logger.With().Str("operation", "DeleteOrganizationRole").Logger()
	if err := validate.ValidateStruct(input); err != nil {
		return err
	}
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		// FOR UPDATE, as for a managed role: a member added on the role at the
		// same moment waits, then fails its foreign key.
		current, findErr := s.editableCustomRole(
			txCtx, transactor.ForUpdate, input.ProductID, input.OrganizationID, input.ID,
		)
		if findErr != nil {
			return findErr
		}
		holders, countErr := s.roleRepo.CountMembershipAssignments(txCtx, input.ID)
		if countErr != nil {
			return countErr
		}
		if holders > 0 {
			return NewRoleInUseError(input.ID)
		}
		if releaseErr := s.releaseInvitations(txCtx, input.ProductID, input.ID, logger); releaseErr != nil {
			return releaseErr
		}
		if deleteErr := s.roleRepo.DeleteCustom(
			txCtx, input.ProductID, input.OrganizationID, input.ID,
		); deleteErr != nil {
			return deleteErr
		}
		return s.emitOrganizationRole(txCtx, events.OrganizationRoleDeleted, current)
	})
	return s.unexpected(logger, err, input.ProductID, "failed to delete custom role")
}

func (s *organizationRoleService) requireOrganization(ctx context.Context, productID, organizationID string) error {
	found, err := s.orgRepo.FindByID(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

// editableCustomRole locks one of the Organization's custom roles with lock,
// then reads it. A managed role answers ROLE_NOT_EDITABLE.
func (s *organizationRoleService) editableCustomRole(
	txCtx context.Context, lock func(context.Context) context.Context, productID, organizationID, id string,
) (role.ProductRole, error) {
	if err := s.requireOrganization(txCtx, productID, organizationID); err != nil {
		return role.ProductRole{}, err
	}
	locked, err := s.roleRepo.ExistsCustom(lock(txCtx), productID, organizationID, id)
	if err != nil {
		return role.ProductRole{}, err
	}
	found, err := s.roleRepo.FindInOrganization(txCtx, productID, organizationID, id)
	if err != nil {
		return role.ProductRole{}, err
	}
	if found.IsAbsent() {
		return role.ProductRole{}, role.ErrProductRoleNotFound
	}
	if !locked {
		return role.ProductRole{}, role.ErrRoleNotEditable
	}
	return found.Value(), nil
}

// customNameAvailable refuses a name the Organization already uses or the
// Product manages. A managed role may later take a custom role's name, so a
// customer never blocks the Product.
func (s *organizationRoleService) customNameAvailable(ctx context.Context, custom role.ProductRole) error {
	managed, err := s.roleRepo.GetByProductIDAndName(ctx, custom.ProductID, custom.Name)
	if err != nil {
		return err
	}
	own, err := s.roleRepo.GetCustomByName(ctx, custom.ProductID, custom.OrganizationID, custom.Name)
	if err != nil {
		return err
	}
	if managed.IsPresent() || (own.IsPresent() && own.Value().ID != custom.ID) {
		return NewCustomRoleNameDuplicateError(custom.Name, custom.OrganizationID)
	}
	return nil
}

func (s *organizationRoleService) emitOrganizationRole(
	ctx context.Context, eventType events.Type, custom role.ProductRole,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: custom.ProductID,
		Data:      events.OrganizationRoleData(custom.OrganizationID, custom.ID),
	})
}

// unexpected passes a domain error through and logs anything else.
func (s *organizationRoleService) unexpected(
	logger zerolog.Logger, err error, productID, message string,
) error {
	if err == nil {
		return nil
	}
	if _, isDomain := errors.AsType[*fault.Error](err); isDomain {
		return err
	}
	logger.Error().Str("product_id", productID).Err(err).Msg(message)
	return fault.ErrUnexpected
}

func newRolePermissions(target role.ProductRole, names []string) []role.ProductRolePermission {
	return functional.Slice(names).Map(func(name string) role.ProductRolePermission {
		p := role.ProductRolePermission{
			ProductRoleID: target.ID, ProductID: target.ProductID, PermissionName: name,
		}
		p.GenerateID()
		return p
	})
}
