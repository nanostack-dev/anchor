// Package service registers, moves and deletes the resources a product
// protects (ADR-0022 B1-B4), and keeps every chain within the parent rules.
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
	"anchor/internal/events"
	orgrepository "anchor/internal/repository"
	"anchor/internal/resource/repository"
	rtservice "anchor/internal/resourcetype/service"
	"anchor/internal/searchx"
)

type Service interface {
	// Put creates, renames or moves one resource, and answers it as stored.
	Put(ctx context.Context, input resource.PutInput) (resource.Resource, error)
	Get(ctx context.Context, productID, organizationID, resourceType, id string) (resource.Resource, error)
	// Delete removes one resource; with cascade, its whole subtree.
	Delete(ctx context.Context, productID, organizationID, resourceType, id string, cascade bool) error
	Search(ctx context.Context, input resource.SearchInput) (search.Result[resource.Resource], error)
}

type service struct {
	repo          repository.Repository
	organizations orgrepository.OrganizationRepository
	resourceTypes rtservice.Service
	transactor    transactor.Transactor
	events        events.Emitter
	logger        zerolog.Logger
}

func NewService(
	repo repository.Repository,
	organizations orgrepository.OrganizationRepository,
	resourceTypes rtservice.Service,
	tx transactor.Transactor,
	emitter events.Emitter,
	logger zerolog.Logger,
) Service {
	return &service{
		repo:          repo,
		organizations: organizations,
		resourceTypes: resourceTypes,
		transactor:    tx,
		events:        emitter,
		logger:        logger.With().Str("component", "resource_service").Logger(),
	}
}

func (s *service) Put(ctx context.Context, input resource.PutInput) (resource.Resource, error) {
	if err := input.Validate(); err != nil {
		return resource.Resource{}, err
	}
	var result resource.Resource
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.holdOrganization(txCtx, input.ProductID, input.OrganizationID); err != nil {
			return err
		}
		tree, err := s.resourceTypes.LockedTree(txCtx, input.ProductID)
		if err != nil {
			return err
		}
		rt, err := registrableType(tree, input.Type)
		if err != nil {
			return err
		}
		parentType, err := allowedParentType(tree, rt, input.Parent.Type)
		if err != nil {
			return err
		}
		if err = s.repo.LockOrganization(txCtx, input.OrganizationID); err != nil {
			return err
		}
		found, err := s.repo.Find(txCtx, input.OrganizationID, rt.ID, input.ID)
		if err != nil {
			return err
		}
		parentRowID, err := s.resolveParent(txCtx, input, parentType)
		if err != nil {
			return err
		}
		existing, exists := found.Get()
		moves := !exists || !samePointer(existing.ParentRowID, parentRowID)
		if moves {
			if err = s.checkChain(txCtx, input.OrganizationID, existing, exists, parentRowID); err != nil {
				return err
			}
		}
		eventType := events.ResourceCreated
		switch {
		case !exists:
			err = s.repo.Create(txCtx, resource.Resource{
				RowID:          resource.NewRowID(),
				ProductID:      input.ProductID,
				OrganizationID: input.OrganizationID,
				TypeID:         rt.ID,
				ExternalID:     input.ID,
				Name:           input.Name,
				ParentRowID:    parentRowID,
			})
		case moves || existing.Name != input.Name:
			eventType = events.ResourceUpdated
			err = s.repo.Update(txCtx, input.OrganizationID, existing.RowID, input.Name, parentRowID)
		default:
			result = existing
			return nil
		}
		if err != nil {
			return err
		}
		if result, err = s.reread(txCtx, input.OrganizationID, rt.ID, input.ID); err != nil {
			return err
		}
		return s.emit(txCtx, eventType, result.ProductID, result.OrganizationID, result.RowID, result.Ref())
	})
	if err != nil {
		return resource.Resource{}, s.unexpected(err, input.ProductID, "put")
	}
	return result, nil
}

// checkChain refuses a parent that would put the resource under itself, or
// make its deepest descendant sit deeper than the limit. The chain is read
// after the Organization lock, so no concurrent move can change it.
func (s *service) checkChain(
	ctx context.Context, organizationID string, existing resource.Resource, exists bool, parentRowID *string,
) error {
	var chain []repository.Node
	if parentRowID != nil {
		var err error
		if chain, err = s.repo.Ancestors(ctx, organizationID, *parentRowID, resource.MaxDepth); err != nil {
			return err
		}
	}
	height := 0
	if exists {
		if at := slices.IndexFunc(chain, func(n repository.Node) bool { return n.RowID == existing.RowID }); at >= 0 {
			return resource.NewCycleError(functional.Slice(chain[:at+1]).Map(
				func(n repository.Node) resource.Ref { return n.Ref },
			))
		}
		var err error
		if height, err = s.repo.SubtreeHeight(ctx, organizationID, existing.RowID); err != nil {
			return err
		}
	}
	// The Organization, the parent's chain, the resource, and its deepest
	// descendant.
	if depth := 1 + len(chain) + 1 + height; depth > resource.MaxDepth {
		return resource.NewTooDeepError(depth)
	}
	return nil
}

func (s *service) resolveParent(
	ctx context.Context, input resource.PutInput, parentType resourcetype.ResourceType,
) (*string, error) {
	if input.Parent.IsOrganization() {
		if input.Parent.ID != input.OrganizationID {
			return nil, resource.NewParentNotRegisteredError(input.Parent)
		}
		return nil, nil
	}
	found, err := s.repo.Find(ctx, input.OrganizationID, parentType.ID, input.Parent.ID)
	if err != nil {
		return nil, err
	}
	parent, ok := found.Get()
	if !ok {
		return nil, resource.NewParentNotRegisteredError(input.Parent)
	}
	return &parent.RowID, nil
}

func (s *service) Get(
	ctx context.Context, productID, organizationID, resourceType, id string,
) (resource.Resource, error) {
	if productID == "" || organizationID == "" || resourceType == "" || id == "" {
		return resource.Resource{}, resource.ErrNotFound
	}
	if err := s.requireOrganization(ctx, productID, organizationID); err != nil {
		return resource.Resource{}, s.unexpected(err, productID, "get")
	}
	tree, err := s.resourceTypes.Tree(ctx, productID)
	if err != nil {
		return resource.Resource{}, s.unexpected(err, productID, "get")
	}
	rt, err := registrableType(tree, resourceType)
	if err != nil {
		return resource.Resource{}, err
	}
	found, err := s.repo.Find(ctx, organizationID, rt.ID, id)
	if err != nil {
		return resource.Resource{}, s.unexpected(err, productID, "get")
	}
	return found.ToResult(resource.ErrNotFound).Value()
}

func (s *service) Delete(
	ctx context.Context, productID, organizationID, resourceType, id string, cascade bool,
) error {
	if productID == "" || organizationID == "" || resourceType == "" || id == "" {
		return resource.ErrNotFound
	}
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		if err := s.holdOrganization(txCtx, productID, organizationID); err != nil {
			return err
		}
		tree, err := s.resourceTypes.LockedTree(txCtx, productID)
		if err != nil {
			return err
		}
		rt, err := registrableType(tree, resourceType)
		if err != nil {
			return err
		}
		if err = s.repo.LockOrganization(txCtx, organizationID); err != nil {
			return err
		}
		found, err := s.repo.Find(txCtx, organizationID, rt.ID, id)
		if err != nil {
			return err
		}
		target, ok := found.Get()
		if !ok {
			return resource.ErrNotFound
		}
		if !cascade {
			children, countErr := s.repo.CountChildren(txCtx, organizationID, target.RowID)
			if countErr != nil {
				return countErr
			}
			if children > 0 {
				return resource.NewInUseError(children, 0)
			}
		}
		subtree, err := s.repo.Subtree(txCtx, organizationID, target.RowID)
		if err != nil {
			return err
		}
		if err = s.repo.Delete(txCtx, organizationID, target.RowID); err != nil {
			return err
		}
		for _, node := range subtree {
			if err = s.emit(
				txCtx,
				events.ResourceDeleted,
				productID,
				organizationID,
				node.RowID,
				node.Ref,
			); err != nil {
				return err
			}
		}
		return nil
	})
	return s.unexpected(err, productID, "delete")
}

func (s *service) Search(
	ctx context.Context, input resource.SearchInput,
) (search.Result[resource.Resource], error) {
	if err := input.Validate(); err != nil {
		return search.Result[resource.Resource]{}, err
	}
	if err := s.requireOrganization(ctx, input.ProductID, input.OrganizationID); err != nil {
		return search.Result[resource.Resource]{}, s.unexpected(err, input.ProductID, "search")
	}
	result, err := s.repo.Search(ctx, input.ProductID, input.OrganizationID, input.Request)
	if err != nil {
		return search.Result[resource.Resource]{}, s.unexpected(err, input.ProductID, "search")
	}
	return result, nil
}

// registrableType resolves the type a path names. Organization is the root
// of every chain, never a registered resource.
func registrableType(tree resourcetype.Tree, slug string) (resourcetype.ResourceType, error) {
	rt, ok := tree.BySlug(slug).Get()
	if !ok {
		return resourcetype.ResourceType{}, resource.NewTypeNotFoundError(slug)
	}
	if rt.BuiltIn() {
		return resourcetype.ResourceType{}, resource.ErrTypeNotRegistrable
	}
	return rt, nil
}

// allowedParentType checks the parent's type against the type's parent types
// and its nesting (ADR-0022 A3, B3).
func allowedParentType(
	tree resourcetype.Tree, rt resourcetype.ResourceType, parentSlug string,
) (resourcetype.ResourceType, error) {
	allowed := slices.Clone(rt.ParentTypes)
	if rt.NestsInItself {
		allowed = append(allowed, rt.Slug)
	}
	slices.Sort(allowed)
	parent, ok := tree.BySlug(parentSlug).Get()
	if !ok || !slices.Contains(allowed, parentSlug) {
		return resourcetype.ResourceType{}, resource.NewInvalidParentError(rt.Slug, parentSlug, allowed)
	}
	return parent, nil
}

// holdOrganization keeps the Organization from being deleted until the write
// commits: a delete racing it would otherwise trip the resource's foreign key.
func (s *service) holdOrganization(ctx context.Context, productID, organizationID string) error {
	exists, err := s.repo.OrganizationExists(transactor.ForKeyShare(ctx), productID, organizationID)
	if err != nil {
		return err
	}
	if !exists {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

func (s *service) requireOrganization(ctx context.Context, productID, organizationID string) error {
	found, err := s.organizations.FindByID(ctx, productID, organizationID)
	if err != nil {
		return err
	}
	if found.IsAbsent() {
		return organization.ErrOrganizationNotFound
	}
	return nil
}

func (s *service) reread(ctx context.Context, organizationID, typeID, id string) (resource.Resource, error) {
	found, err := s.repo.Find(ctx, organizationID, typeID, id)
	if err != nil {
		return resource.Resource{}, err
	}
	written, ok := found.Get()
	if !ok {
		return resource.Resource{}, errors.New("resource missing after its write")
	}
	return written, nil
}

func (s *service) emit(
	ctx context.Context, eventType events.Type, productID, organizationID, rowID string, ref resource.Ref,
) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: productID,
		Data: events.Data{
			events.FieldOrganizationID: organizationID,
			events.FieldResourceType:   ref.Type,
			events.FieldResourceID:     ref.ID,
			events.FieldResourceRowID:  rowID,
		},
	})
}

func samePointer(a, b *string) bool {
	return functional.FromPtr(a).OrElse("") == functional.FromPtr(b).OrElse("")
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
		Msg("resource operation failed")
	return fault.ErrUnexpected
}
