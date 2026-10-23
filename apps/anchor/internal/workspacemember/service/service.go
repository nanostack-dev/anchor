package service

import (
	"context"

	"github.com/go-jet/jet/v2/postgres"
	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/validate"

	"anchor/internal/domain/workspace"
	"anchor/internal/events"
	anchorrepository "anchor/internal/repository"
	anchorservice "anchor/internal/service"
	"anchor/internal/workspacemember/repository"
)

// WorkspaceMemberService manages the roles organization members hold in a
// workspace. In that workspace a member holds the permissions of their
// organization role and of the workspace role together.
type WorkspaceMemberService interface {
	// Assign gives an organization member a role in the workspace, or replaces
	// the role they hold there. Assigning the role they hold already writes
	// nothing and emits no event.
	Assign(ctx context.Context, input workspace.AssignMemberRoleInput) (workspace.Membership, error)
	Find(ctx context.Context, input workspace.GetMemberInput) (workspace.Membership, error)
	Remove(ctx context.Context, input workspace.RemoveMemberInput) error
	Search(ctx context.Context, input workspace.SearchMembersInput) (search.Result[workspace.Membership], error)
}

type workspaceMemberService struct {
	memberRepo      repository.Repository
	workspaceRepo   anchorrepository.WorkspaceRepository
	productRoleRepo anchorrepository.ProductRoleRepository
	transactor      transactor.Transactor
	events          events.Emitter
}

func NewWorkspaceMemberService(
	memberRepo repository.Repository,
	workspaceRepo anchorrepository.WorkspaceRepository,
	productRoleRepo anchorrepository.ProductRoleRepository,
	tx transactor.Transactor,
	eventEmitter events.Emitter,
) WorkspaceMemberService {
	return &workspaceMemberService{
		memberRepo:      memberRepo,
		workspaceRepo:   workspaceRepo,
		productRoleRepo: productRoleRepo,
		transactor:      tx,
		events:          eventEmitter,
	}
}

func (s *workspaceMemberService) Assign(
	ctx context.Context, input workspace.AssignMemberRoleInput,
) (workspace.Membership, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workspace.Membership{}, err
	}

	var membership workspace.Membership
	txErr := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.lockWorkspace(txCtx, input.ProductID, input.OrganizationID, input.WorkspaceID); err != nil {
			return err
		}
		if err := s.ensureRoleExists(txCtx, input.ProductID, input.RoleID); err != nil {
			return err
		}
		if err := s.lockOrganizationMember(
			txCtx, input.ProductID, input.OrganizationID, input.ProductUserID,
		); err != nil {
			return err
		}

		current, err := s.memberRepo.Find(
			txCtx, input.ProductID, input.OrganizationID, input.WorkspaceID, input.ProductUserID,
		)
		if err != nil {
			return err
		}
		if current.Exists(func(m workspace.Membership) bool { return m.RoleID == input.RoleID }) {
			membership = current.Value()
			return nil
		}

		membership, err = s.memberRepo.Upsert(
			txCtx, input.ProductID, input.OrganizationID, input.WorkspaceID, input.ProductUserID, input.RoleID,
		)
		if err != nil {
			return err
		}
		eventType := events.WorkspaceMembershipCreated
		if current.IsPresent() {
			eventType = events.WorkspaceMembershipUpdated
		}
		return s.events.Emit(txCtx, events.Event{
			Type:      eventType,
			ProductID: input.ProductID,
			Data: events.WorkspaceMembershipData(
				membership.OrganizationID, membership.WorkspaceID, membership.ProductUserID, membership.RoleID,
			),
		})
	})
	if txErr != nil {
		return workspace.Membership{}, txErr
	}

	return membership, nil
}

func (s *workspaceMemberService) Find(
	ctx context.Context, input workspace.GetMemberInput,
) (workspace.Membership, error) {
	if err := validate.ValidateStruct(input); err != nil {
		return workspace.Membership{}, err
	}

	found, err := s.memberRepo.Find(
		ctx, input.ProductID, input.OrganizationID, input.WorkspaceID, input.ProductUserID,
	)
	if err != nil {
		return workspace.Membership{}, err
	}
	if found.IsAbsent() {
		return workspace.Membership{}, s.notFound(ctx, input.ProductID, input.OrganizationID, input.WorkspaceID)
	}

	return found.Value(), nil
}

func (s *workspaceMemberService) Remove(ctx context.Context, input workspace.RemoveMemberInput) error {
	if err := validate.ValidateStruct(input); err != nil {
		return err
	}

	return s.transactor.InTx(ctx, func(txCtx context.Context) error {
		removed, err := s.memberRepo.Delete(
			txCtx, input.ProductID, input.OrganizationID, input.WorkspaceID, input.ProductUserID,
		)
		if err != nil {
			return err
		}
		if removed.IsAbsent() {
			return s.notFound(txCtx, input.ProductID, input.OrganizationID, input.WorkspaceID)
		}
		return s.events.Emit(txCtx, events.Event{
			Type:      events.WorkspaceMembershipDeleted,
			ProductID: input.ProductID,
			Data: events.WorkspaceMembershipData(
				input.OrganizationID, input.WorkspaceID, input.ProductUserID, removed.Value().RoleID,
			),
		})
	})
}

func (s *workspaceMemberService) Search(
	ctx context.Context, input workspace.SearchMembersInput,
) (search.Result[workspace.Membership], error) {
	if err := input.Validate(); err != nil {
		return search.Result[workspace.Membership]{}, err
	}

	if err := s.ensureWorkspaceExists(ctx, input.ProductID, input.OrganizationID, input.WorkspaceID); err != nil {
		return search.Result[workspace.Membership]{}, err
	}

	return s.memberRepo.Search(ctx, input.ProductID, input.OrganizationID, input.WorkspaceID, input.Request)
}

// notFound explains a missing membership: the workspace is gone, or the product
// user holds no role in it.
func (s *workspaceMemberService) notFound(ctx context.Context, productID, organizationID, workspaceID string) error {
	if err := s.ensureWorkspaceExists(ctx, productID, organizationID, workspaceID); err != nil {
		return err
	}
	return workspace.ErrMembershipNotFound
}

func (s *workspaceMemberService) ensureWorkspaceExists(
	ctx context.Context, productID, organizationID, workspaceID string,
) error {
	found, err := s.workspaceRepo.FindByID(ctx, productID, organizationID, workspaceID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return workspace.ErrWorkspaceNotFound
	}
	return nil
}

// lockWorkspace fails when the workspace does not exist, and otherwise keeps it
// from being deleted until the transaction ends.
func (s *workspaceMemberService) lockWorkspace(
	ctx context.Context, productID, organizationID, workspaceID string,
) error {
	return s.ensureWorkspaceExists(
		transactor.WithRowLock(ctx, postgres.KEY_SHARE()), productID, organizationID, workspaceID,
	)
}

func (s *workspaceMemberService) lockOrganizationMember(
	ctx context.Context, productID, organizationID, productUserID string,
) error {
	isMember, err := s.memberRepo.LockOrganizationMember(ctx, productID, organizationID, productUserID)
	if err != nil {
		return err
	}
	if !isMember {
		return workspace.NewNotOrganizationMemberError(productUserID, organizationID)
	}
	return nil
}

func (s *workspaceMemberService) ensureRoleExists(ctx context.Context, productID, roleID string) error {
	found, err := s.productRoleRepo.FindByProductIDAndRoleID(ctx, productID, roleID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return anchorservice.NewBodyRoleNotFoundError(roleID)
	}
	return nil
}
