package service

import (
	"cmp"
	"context"
	"errors"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"
	"github.com/rs/zerolog"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/organizationrole"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	invitationrepository "anchor/internal/invitation/repository"
	orgrolerepository "anchor/internal/organizationrole/repository"
	orgroleservice "anchor/internal/organizationrole/service"
	"anchor/internal/repository"
	rtservice "anchor/internal/resourcetype/service"
)

// OrganizationCustomRoleService writes the roles an Organization shapes itself
// (ADR-0022 D9, D10): custom roles, and copies of role templates it
// customizes. Every write needs the custom_roles switch of the role's
// resource type, except a delete or a reset, which only undo.
type OrganizationCustomRoleService interface {
	Create(ctx context.Context, input organizationrole.CreateInput) (organizationrole.Role, error)
	Update(ctx context.Context, input organizationrole.UpdateInput) (organizationrole.Role, error)
	Delete(ctx context.Context, productID, organizationID, roleID string) error
	Reset(ctx context.Context, productID, organizationID, roleID string) (organizationrole.Role, error)
}

type organizationCustomRoleService struct {
	repo          orgrolerepository.Repository
	roles         orgroleservice.Service
	resourceTypes rtservice.Service
	permissions   repository.ProductResourcePermissionRepository
	invitations   invitationrepository.Repository
	transactor    transactor.Transactor
	events        events.Emitter
	logger        zerolog.Logger
}

func NewOrganizationCustomRoleService(
	repo orgrolerepository.Repository,
	roles orgroleservice.Service,
	resourceTypes rtservice.Service,
	permissions repository.ProductResourcePermissionRepository,
	invitations invitationrepository.Repository,
	tx transactor.Transactor,
	emitter events.Emitter,
	logger zerolog.Logger,
) OrganizationCustomRoleService {
	return &organizationCustomRoleService{
		repo:          repo,
		roles:         roles,
		resourceTypes: resourceTypes,
		permissions:   permissions,
		invitations:   invitations,
		transactor:    tx,
		events:        emitter,
		logger:        logger.With().Str("component", "organization_custom_role_service").Logger(),
	}
}

func (s *organizationCustomRoleService) Create(
	ctx context.Context, input organizationrole.CreateInput,
) (organizationrole.Role, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return organizationrole.Role{}, err
	}
	if input.ResourceType == "" {
		return organizationrole.Role{}, resourcetype.ErrRequired
	}
	names, resolveErr := resolvePermissionNames(ctx, s.permissions, input.ProductID, input.Permissions, s.logger)
	if resolveErr != nil {
		return organizationrole.Role{}, resolveErr
	}
	key := cmp.Or(input.Key, strings.ToLower(input.Name))
	var created organizationrole.Role
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, txErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if txErr != nil {
			return txErr
		}
		var err error
		scope, known := tree.BySlug(input.ResourceType).Get()
		if !known {
			return resourcetype.NewUnknownError([]string{input.ResourceType})
		}
		if !scope.CustomRoles {
			return organizationrole.ErrCustomRolesDisabled
		}
		if err = s.lockOrganization(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		// The key lock comes before any template lock, in the order a template
		// create takes them.
		if err = s.claimKey(txCtx, input.ProductID, input.OrganizationID, key); err != nil {
			return err
		}
		if err = s.roles.EnsureCopies(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		count, err := s.repo.CountCustom(txCtx, input.ProductID, input.OrganizationID)
		if err != nil {
			return err
		}
		if count >= organizationrole.MaxCustomRoles {
			return organizationrole.NewCustomRoleLimitError()
		}
		if err = checkNamedBundle(txCtx, s.permissions, tree, input.ProductID, scope.Slug, names); err != nil {
			return err
		}
		role := organizationrole.Role{
			RowID:          organizationrole.NewRowID(),
			ProductID:      input.ProductID,
			OrganizationID: input.OrganizationID,
			Key:            key,
			Name:           input.Name,
			Description:    functional.FromPtr(input.Description).OrElse(""),
			ResourceTypeID: &scope.ID,
			Permissions:    names,
		}
		err = s.repo.CreateCustom(txCtx, role)
		if errors.Is(err, orgrolerepository.ErrKeyExists) {
			return organizationrole.NewKeyExistsError(key)
		}
		if err != nil {
			return err
		}
		if err = s.emit(txCtx, events.OrganizationRoleCreated, input.ProductID, role); err != nil {
			return err
		}
		created, err = s.roles.Read(txCtx, input.ProductID, input.OrganizationID, role.RowID)
		return err
	})
	if err != nil {
		return organizationrole.Role{}, s.unexpected(err, input.ProductID, "create")
	}
	return created, nil
}

func (s *organizationCustomRoleService) Update(
	ctx context.Context, input organizationrole.UpdateInput,
) (organizationrole.Role, error) {
	if !input.ChangesBundle() {
		return s.roles.Update(ctx, input)
	}
	if err := validate.ValidateStruct(input); err != nil {
		return organizationrole.Role{}, err
	}
	names, resolveErr := resolvePermissionNames(ctx, s.permissions, input.ProductID, input.Permissions, s.logger)
	if resolveErr != nil {
		return organizationrole.Role{}, resolveErr
	}
	var updated organizationrole.Role
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, txErr := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if txErr != nil {
			return txErr
		}
		var err error
		if err = s.lockOrganization(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		if err = s.roles.EnsureCopies(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		current, err := s.lockedRead(txCtx, input.ProductID, input.OrganizationID, input.RoleID)
		if err != nil {
			return err
		}
		if !customRolesOn(tree, current.ResourceType) {
			return organizationrole.ErrCustomRolesDisabled
		}
		rewritten := current
		rewritten.Customized = current.TemplateID != nil
		if input.Name != nil {
			rewritten.Name = *input.Name
		}
		if input.Description != nil {
			rewritten.Description = *input.Description
		}
		if err = s.repo.Rewrite(txCtx, rewritten); err != nil {
			return err
		}
		if input.PermissionsSet {
			if err = checkNamedBundle(
				txCtx,
				s.permissions,
				tree,
				input.ProductID,
				current.ResourceType,
				names,
			); err != nil {
				return err
			}
			if err = s.repo.ReplacePermissions(txCtx, input.ProductID, current.RowID, names); err != nil {
				return err
			}
		}
		if err = s.emit(txCtx, events.OrganizationRoleUpdated, input.ProductID, current); err != nil {
			return err
		}
		if err = s.roles.ApplyDefault(txCtx, input); err != nil {
			return err
		}
		updated, err = s.roles.Read(txCtx, input.ProductID, input.OrganizationID, input.RoleID)
		return err
	})
	if err != nil {
		return organizationrole.Role{}, s.unexpected(err, input.ProductID, "update")
	}
	return updated, nil
}

func (s *organizationCustomRoleService) Delete(ctx context.Context, productID, organizationID, roleID string) error {
	if productID == "" || organizationID == "" || roleID == "" {
		return organizationrole.ErrNotFound
	}
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.lockOrganization(txCtx, productID, organizationID); err != nil {
			return err
		}
		current, err := s.lockedRead(txCtx, productID, organizationID, roleID)
		if err != nil {
			return err
		}
		if current.TemplateID != nil {
			return organizationrole.ErrTemplateCopyNotDeletable
		}
		previousDefault, err := s.roles.EffectiveDefault(txCtx, productID, organizationID)
		if err != nil {
			return err
		}
		holders, err := s.repo.CountHolders(txCtx, productID, organizationID, current)
		if err != nil {
			return err
		}
		if holders > 0 {
			return NewRoleInUseError(current.PublicID())
		}
		if err = releaseRoleInvitations(
			txCtx,
			s.invitations,
			s.events,
			productID,
			current.PublicID(),
			s.logger,
		); err != nil {
			return err
		}
		if err = s.repo.DeleteCustom(txCtx, productID, organizationID, current.RowID); err != nil {
			return err
		}
		if err = s.emit(txCtx, events.OrganizationRoleDeleted, productID, current); err != nil {
			return err
		}
		return s.roles.ReportDefaultMove(txCtx, productID, organizationID, previousDefault)
	})
	return s.unexpected(err, productID, "delete")
}

func (s *organizationCustomRoleService) Reset(
	ctx context.Context, productID, organizationID, roleID string,
) (organizationrole.Role, error) {
	if productID == "" || organizationID == "" || roleID == "" {
		return organizationrole.Role{}, organizationrole.ErrNotFound
	}
	var reset organizationrole.Role
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.lockOrganization(txCtx, productID, organizationID); err != nil {
			return err
		}
		current, err := s.lockedRead(txCtx, productID, organizationID, roleID)
		if err != nil {
			return err
		}
		if current.TemplateID == nil {
			return organizationrole.ErrNotATemplateCopy
		}
		if current.Customized {
			if err = s.repo.Reset(txCtx, productID, current.RowID, *current.TemplateID); err != nil {
				return err
			}
			if err = s.emit(txCtx, events.OrganizationRoleUpdated, productID, current); err != nil {
				return err
			}
		}
		reset, err = s.roles.Read(txCtx, productID, organizationID, roleID)
		return err
	})
	if err != nil {
		return organizationrole.Role{}, s.unexpected(err, productID, "reset")
	}
	return reset, nil
}

// lockedRead locks the role's row, then reads it, so a write works from the
// role as it is once no other write holds it: a template change waiting on
// the copy, or a member added to a custom role being deleted.
func (s *organizationCustomRoleService) lockedRead(
	ctx context.Context, productID, organizationID, roleID string,
) (organizationrole.Role, error) {
	found, err := s.roles.Read(ctx, productID, organizationID, roleID)
	if err != nil {
		return organizationrole.Role{}, err
	}
	if err = s.repo.LockRole(ctx, productID, organizationID, found.RowID); err != nil {
		return organizationrole.Role{}, err
	}
	return s.roles.Read(ctx, productID, organizationID, roleID)
}

func (s *organizationCustomRoleService) lockOrganization(ctx context.Context, productID, organizationID string) error {
	found, err := s.repo.LockOrganization(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	if !found {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

// claimKey holds the key for the transaction, then refuses it when a role
// template of the product or a role of the Organization holds it: every
// template's copy in the Organization takes its template's key.
func (s *organizationCustomRoleService) claimKey(ctx context.Context, productID, organizationID, key string) error {
	if err := s.repo.LockKey(ctx, productID, key); err != nil {
		return err
	}
	taken, err := s.repo.KeyTaken(ctx, productID, organizationID, key, true)
	if err != nil {
		return err
	}
	if taken {
		return organizationrole.NewKeyExistsError(key)
	}
	return nil
}

func (s *organizationCustomRoleService) emit(
	ctx context.Context, eventType events.Type, productID string, role organizationrole.Role,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: productID,
		Data: events.Data{
			events.FieldOrganizationID: role.OrganizationID,
			events.FieldRoleID:         role.PublicID(),
		},
	})
}

func (s *organizationCustomRoleService) unexpected(err error, productID, operation string) error {
	if err == nil {
		return nil
	}
	if _, isFault := fault.As(err); isFault {
		return err
	}
	s.logger.Error().Err(err).Str("product_id", productID).Str("operation", operation).
		Msg("organization custom role operation failed")
	return fault.ErrUnexpected
}

// customRolesOn reports whether the product lets Organizations write roles
// scoped to the type.
func customRolesOn(tree resourcetype.Tree, slug string) bool {
	return tree.BySlug(slug).Map(func(rt resourcetype.ResourceType) bool { return rt.CustomRoles }).OrElse(false)
}
