// Package service assigns the Organization's roles to its members on
// registered resources (ADR-0022 D3, D4), and releases those assignments when
// a member leaves or a resource goes.
package service

import (
	"context"
	"errors"
	"slices"

	"github.com/nanostack-dev/nanostack-framework/pkg/db/transactor"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	"github.com/rs/zerolog"

	"anchor/internal/domain/organization"
	"anchor/internal/domain/resource"
	"anchor/internal/domain/resourcetype"
	"anchor/internal/domain/roleassignment"
	"anchor/internal/events"
	orgroleservice "anchor/internal/organizationrole/service"
	orgrepository "anchor/internal/repository"
	resourcerepository "anchor/internal/resource/repository"
	rtservice "anchor/internal/resourcetype/service"
	"anchor/internal/roleassignment/repository"
	"anchor/internal/searchx"
)

type Service interface {
	// Put replaces the roles a member holds on a resource, and answers them.
	Put(ctx context.Context, input roleassignment.PutInput) (roleassignment.Member, error)
	Get(ctx context.Context, target roleassignment.Target) (roleassignment.Member, error)
	// Delete removes every role a member holds on a resource.
	Delete(ctx context.Context, target roleassignment.Target) error
	Search(ctx context.Context, input roleassignment.SearchInput) (search.Result[roleassignment.Member], error)

	// ReleaseMember emits a deleted event for every assignment the member
	// holds in the Organization, inside the transaction that removes the
	// membership; the membership's foreign key removes the rows.
	ReleaseMember(txCtx context.Context, productID, organizationID, productUserID string) error
	// ReleaseResources emits a deleted event for every assignment held on the
	// resources, inside the transaction that deletes them under the
	// Organization's resource lock; the resource foreign key removes the rows.
	ReleaseResources(txCtx context.Context, productID, organizationID string, resourceRowIDs []string) error
	// CountOnResource counts the assignments held on one resource.
	CountOnResource(txCtx context.Context, resourceRowID string) (int, error)
}

type service struct {
	repo          repository.Repository
	organizations orgrepository.OrganizationRepository
	resources     resourcerepository.Repository
	resourceTypes rtservice.Service
	roles         orgroleservice.Service
	transactor    transactor.Transactor
	events        events.Emitter
	logger        zerolog.Logger
}

func NewService(
	repo repository.Repository,
	organizations orgrepository.OrganizationRepository,
	resources resourcerepository.Repository,
	resourceTypes rtservice.Service,
	roles orgroleservice.Service,
	tx transactor.Transactor,
	emitter events.Emitter,
	logger zerolog.Logger,
) Service {
	return &service{
		repo:          repo,
		organizations: organizations,
		resources:     resources,
		resourceTypes: resourceTypes,
		roles:         roles,
		transactor:    tx,
		events:        emitter,
		logger:        logger.With().Str("component", "role_assignment_service").Logger(),
	}
}

// The locks of a write, in the one order every write takes them: the
// Organization's row, the Organization's resources in share mode (a
// registration, a move or a delete holds them exclusively), the member on the
// resource, the membership, then each role (organizationrole Resolve).

func (s *service) Put(ctx context.Context, input roleassignment.PutInput) (roleassignment.Member, error) {
	if err := input.Validate(); err != nil {
		return roleassignment.Member{}, err
	}
	var result roleassignment.Member
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		target, tree, err := s.lockedResource(txCtx, input.Target)
		if err != nil {
			return err
		}
		if err = s.repo.LockMember(txCtx, target.RowID, input.ProductUserID); err != nil {
			return err
		}
		member, err := s.repo.ShareMembership(txCtx, input.OrganizationID, input.ProductUserID)
		if err != nil {
			return err
		}
		if !member {
			return organization.ErrMembershipNotFound
		}
		wanted, err := s.resolveRoles(txCtx, input, target, tree)
		if err != nil {
			return err
		}
		found, err := s.repo.Find(txCtx, target.RowID, input.ProductUserID)
		if err != nil {
			return err
		}
		current, held := found.Get()
		removed, added := difference(current.Roles, wanted)
		if len(removed) == 0 && len(added) == 0 {
			result = current
			return nil
		}
		if err = s.replace(txCtx, input.OrganizationID, target.RowID, input.ProductUserID, removed, added); err != nil {
			return err
		}
		if result, err = s.reread(txCtx, target.RowID, input.ProductUserID); err != nil {
			return err
		}
		eventType := events.ResourceRoleAssignmentCreated
		if held {
			eventType = events.ResourceRoleAssignmentUpdated
		}
		return s.emit(txCtx, eventType, input.ProductID, roleassignment.Held{
			ResourceRowID:      target.RowID,
			ResourceType:       target.Type,
			ResourceExternalID: target.ExternalID,
			OrganizationID:     input.OrganizationID,
			ProductUserID:      input.ProductUserID,
			RoleID:             result.Roles[0].ID,
		})
	})
	if err != nil {
		return roleassignment.Member{}, s.unexpected(err, input.ProductID, "put")
	}
	return result, nil
}

// replace moves removed assignments onto added roles first, so a member whose
// role changes keeps when they first held a role on the resource.
func (s *service) replace(
	txCtx context.Context, organizationID, resourceRowID, productUserID string, removed, added []string,
) error {
	moved := min(len(removed), len(added))
	for i := range moved {
		if err := s.repo.Reassign(txCtx, resourceRowID, productUserID, removed[i], added[i]); err != nil {
			return err
		}
	}
	if err := s.repo.Delete(txCtx, resourceRowID, productUserID, removed[moved:]); err != nil {
		return err
	}
	return s.repo.Insert(txCtx, organizationID, resourceRowID, productUserID, added[moved:])
}

// resolveRoles finds each role inside the Organization, a template's ID
// naming its copy, and refuses one scoped to another type than the resource.
func (s *service) resolveRoles(
	txCtx context.Context, input roleassignment.PutInput, target resource.Resource, tree resourcetype.Tree,
) ([]string, error) {
	var wanted []string
	for _, roleID := range input.RoleIDs {
		role, err := s.roles.Resolve(txCtx, input.ProductID, input.OrganizationID, roleID)
		if err != nil {
			return nil, err
		}
		if scope := functional.FromPtr(role.ResourceTypeID).OrElse(""); scope != target.TypeID {
			return nil, roleassignment.NewScopeMismatchError(roleID, scopeSlug(tree, scope), target.Type)
		}
		if !slices.Contains(wanted, role.RowID) {
			wanted = append(wanted, role.RowID)
		}
	}
	return wanted, nil
}

// difference lists the role rows to remove and to add, each sorted.
func difference(current []roleassignment.Role, wanted []string) ([]string, []string) {
	held := functional.Slice(current).Map(func(r roleassignment.Role) string { return r.RowID })
	var removed, added []string
	for _, rowID := range held {
		if !slices.Contains(wanted, rowID) {
			removed = append(removed, rowID)
		}
	}
	for _, rowID := range wanted {
		if !slices.Contains(held, rowID) {
			added = append(added, rowID)
		}
	}
	slices.Sort(removed)
	slices.Sort(added)
	return removed, added
}

func (s *service) Get(ctx context.Context, target roleassignment.Target) (roleassignment.Member, error) {
	if target.Empty() {
		return roleassignment.Member{}, roleassignment.ErrMemberNotFound
	}
	found, err := s.findResource(ctx, target.ProductID, target.OrganizationID, target.ResourceType, target.ResourceID)
	if err != nil {
		return roleassignment.Member{}, s.unexpected(err, target.ProductID, "get")
	}
	member, err := s.repo.Find(ctx, found.RowID, target.ProductUserID)
	if err != nil {
		return roleassignment.Member{}, s.unexpected(err, target.ProductID, "get")
	}
	return member.ToResult(roleassignment.ErrMemberNotFound).Value()
}

func (s *service) Delete(ctx context.Context, target roleassignment.Target) error {
	if target.Empty() {
		return roleassignment.ErrMemberNotFound
	}
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		res, _, err := s.lockedResource(txCtx, target)
		if err != nil {
			return err
		}
		if err = s.repo.LockMember(txCtx, res.RowID, target.ProductUserID); err != nil {
			return err
		}
		found, err := s.repo.Find(txCtx, res.RowID, target.ProductUserID)
		if err != nil {
			return err
		}
		current, ok := found.Get()
		if !ok {
			return roleassignment.ErrMemberNotFound
		}
		rowIDs := functional.Slice(current.Roles).Map(func(r roleassignment.Role) string { return r.RowID })
		if err = s.repo.Delete(txCtx, res.RowID, target.ProductUserID, rowIDs); err != nil {
			return err
		}
		for _, role := range current.Roles {
			if err = s.emit(txCtx, events.ResourceRoleAssignmentDeleted, target.ProductID, roleassignment.Held{
				ResourceRowID:      res.RowID,
				ResourceType:       res.Type,
				ResourceExternalID: res.ExternalID,
				OrganizationID:     target.OrganizationID,
				ProductUserID:      target.ProductUserID,
				RoleRowID:          role.RowID,
				RoleID:             role.ID,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return s.unexpected(err, target.ProductID, "delete")
}

func (s *service) Search(
	ctx context.Context, input roleassignment.SearchInput,
) (search.Result[roleassignment.Member], error) {
	if err := input.Validate(); err != nil {
		return search.Result[roleassignment.Member]{}, err
	}
	found, err := s.findResource(ctx, input.ProductID, input.OrganizationID, input.ResourceType, input.ResourceID)
	if err != nil {
		return search.Result[roleassignment.Member]{}, s.unexpected(err, input.ProductID, "search")
	}
	result, err := s.repo.Search(ctx, found.RowID, input.Request)
	if err != nil {
		return search.Result[roleassignment.Member]{}, s.unexpected(err, input.ProductID, "search")
	}
	return result, nil
}

func (s *service) ReleaseMember(txCtx context.Context, productID, organizationID, productUserID string) error {
	if err := s.resources.ShareOrganization(txCtx, organizationID); err != nil {
		return err
	}
	if err := s.repo.LockMembership(txCtx, organizationID, productUserID); err != nil {
		return err
	}
	held, err := s.repo.HeldByMember(txCtx, organizationID, productUserID)
	if err != nil {
		return err
	}
	return s.emitDeleted(txCtx, productID, held)
}

func (s *service) ReleaseResources(
	txCtx context.Context, productID, organizationID string, resourceRowIDs []string,
) error {
	held, err := s.repo.HeldOnResources(txCtx, organizationID, resourceRowIDs)
	if err != nil {
		return err
	}
	return s.emitDeleted(txCtx, productID, held)
}

func (s *service) CountOnResource(txCtx context.Context, resourceRowID string) (int, error) {
	return s.repo.CountOnResource(txCtx, resourceRowID)
}

func (s *service) emitDeleted(txCtx context.Context, productID string, held []roleassignment.Held) error {
	for _, assignment := range held {
		if err := s.emit(txCtx, events.ResourceRoleAssignmentDeleted, productID, assignment); err != nil {
			return err
		}
	}
	return nil
}

// lockedResource finds the resource a write names, after holding its
// Organization and, in share mode, the Organization's resources.
func (s *service) lockedResource(
	txCtx context.Context, target roleassignment.Target,
) (resource.Resource, resourcetype.Tree, error) {
	exists, err := s.resources.HoldOrganization(txCtx, target.ProductID, target.OrganizationID)
	if err != nil {
		return resource.Resource{}, resourcetype.Tree{}, err
	}
	if !exists {
		return resource.Resource{}, resourcetype.Tree{}, organization.ErrOrganizationNotFound
	}
	tree, err := s.resourceTypes.Tree(txCtx, target.ProductID)
	if err != nil {
		return resource.Resource{}, resourcetype.Tree{}, err
	}
	rt, err := resource.RegistrableType(tree, target.ResourceType)
	if err != nil {
		return resource.Resource{}, resourcetype.Tree{}, err
	}
	if err = s.resources.ShareOrganization(txCtx, target.OrganizationID); err != nil {
		return resource.Resource{}, resourcetype.Tree{}, err
	}
	found, err := s.resources.Find(txCtx, target.OrganizationID, rt.ID, target.ResourceID)
	if err != nil {
		return resource.Resource{}, resourcetype.Tree{}, err
	}
	res, err := found.ToResult(resource.ErrNotFound).Value()
	return res, tree, err
}

func (s *service) findResource(
	ctx context.Context, productID, organizationID, resourceType, resourceID string,
) (resource.Resource, error) {
	org, err := s.organizations.FindByID(ctx, productID, organizationID)
	if err != nil {
		return resource.Resource{}, err
	}
	if org.IsAbsent() {
		return resource.Resource{}, organization.ErrOrganizationNotFound
	}
	tree, err := s.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return resource.Resource{}, err
	}
	rt, err := resource.RegistrableType(tree, resourceType)
	if err != nil {
		return resource.Resource{}, err
	}
	found, err := s.resources.Find(ctx, organizationID, rt.ID, resourceID)
	if err != nil {
		return resource.Resource{}, err
	}
	return found.ToResult(resource.ErrNotFound).Value()
}

func (s *service) reread(txCtx context.Context, resourceRowID, productUserID string) (roleassignment.Member, error) {
	found, err := s.repo.Find(txCtx, resourceRowID, productUserID)
	if err != nil {
		return roleassignment.Member{}, err
	}
	member, ok := found.Get()
	if !ok {
		return roleassignment.Member{}, errors.New("role assignment missing after its write")
	}
	return member, nil
}

func (s *service) emit(txCtx context.Context, eventType events.Type, productID string, held roleassignment.Held) error {
	return s.events.Emit(txCtx, events.Event{
		Type:      eventType,
		ProductID: productID,
		Data: events.Data{
			events.FieldOrganizationID: held.OrganizationID,
			events.FieldResourceType:   held.ResourceType,
			events.FieldResourceID:     held.ResourceExternalID,
			events.FieldResourceRowID:  held.ResourceRowID,
			events.FieldProductUserID:  held.ProductUserID,
			events.FieldRoleID:         held.RoleID,
		},
	})
}

// scopeSlug names a role's scope; an empty one is organization, as the
// resource type backfill writes.
func scopeSlug(tree resourcetype.Tree, typeID string) string {
	if typeID == "" {
		return resourcetype.OrganizationSlug
	}
	return tree.ByID(typeID).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
		OrElse(resourcetype.OrganizationSlug)
}

// unexpected passes client errors through and logs anything else once.
func (s *service) unexpected(err error, productID, operation string) error {
	if err == nil || searchx.IsClientError(err) {
		return err
	}
	if _, isFault := fault.As(err); isFault {
		return err
	}
	s.logger.Error().Err(err).Str("product_id", productID).Str("operation", operation).
		Msg("role assignment operation failed")
	return fault.ErrUnexpected
}
