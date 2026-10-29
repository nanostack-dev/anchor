// Package service manages a product's tree of resource types (ADR-0022 A1-A5)
// and gives the role and permission services the tree their scopes live in.
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

	"anchor/internal/domain/resourcetype"
	"anchor/internal/events"
	"anchor/internal/resourcetype/repository"
	"anchor/internal/searchx"
)

type Service interface {
	Create(ctx context.Context, input resourcetype.CreateInput) (resourcetype.ResourceType, error)
	Get(ctx context.Context, productID, id string) (resourcetype.ResourceType, error)
	Update(ctx context.Context, input resourcetype.UpdateInput) (resourcetype.ResourceType, error)
	Delete(ctx context.Context, productID, id string) error
	Search(ctx context.Context, input resourcetype.SearchInput) (search.Result[resourcetype.ResourceType], error)
	ReplaceTree(ctx context.Context, input resourcetype.ReplaceTreeInput) ([]resourcetype.ResourceType, error)

	// Tree reads the product's tree, seeding organization when a product
	// created by an earlier release lacks it.
	Tree(ctx context.Context, productID string) (resourcetype.Tree, error)
	// LockedTree is Tree inside a role or permission write: it holds the
	// catalog lock in share mode, so no tree write can move a scope boundary
	// before the write commits.
	LockedTree(txCtx context.Context, productID string) (resourcetype.Tree, error)
	// SeedOrganization creates the root type of a new product inside its
	// create transaction.
	SeedOrganization(txCtx context.Context, productID string) error
}

type service struct {
	repo       repository.Repository
	transactor transactor.Transactor
	events     events.Emitter
	logger     zerolog.Logger
}

func NewService(
	repo repository.Repository, tx transactor.Transactor, emitter events.Emitter, logger zerolog.Logger,
) Service {
	return &service{
		repo:       repo,
		transactor: tx,
		events:     emitter,
		logger:     logger.With().Str("component", "resource_type_service").Logger(),
	}
}

func (s *service) Tree(ctx context.Context, productID string) (resourcetype.Tree, error) {
	types, err := s.repo.FindTree(ctx, productID)
	if err != nil {
		return resourcetype.Tree{}, err
	}
	tree := resourcetype.NewTree(types)
	if tree.Organization().IsPresent() {
		return tree, nil
	}
	if err = s.repo.EnsureOrganization(ctx, productID); err != nil {
		return resourcetype.Tree{}, err
	}
	types, err = s.repo.FindTree(ctx, productID)
	if err != nil {
		return resourcetype.Tree{}, err
	}
	return resourcetype.NewTree(types), nil
}

// LockedTree holds the product's organization type FOR SHARE, so role and
// permission writes check scopes side by side while no tree write runs.
func (s *service) LockedTree(txCtx context.Context, productID string) (resourcetype.Tree, error) {
	if err := s.repo.EnsureOrganization(txCtx, productID); err != nil {
		return resourcetype.Tree{}, err
	}
	if err := s.repo.FindOrganization(transactor.ForShare(txCtx), productID); err != nil {
		return resourcetype.Tree{}, err
	}
	return s.tree(txCtx, productID)
}

// writableTree holds the product's organization type FOR NO KEY UPDATE for a
// tree write: it waits for the role and permission writes holding it FOR SHARE
// and keeps new ones out, while rows that reference the type still insert.
func (s *service) writableTree(txCtx context.Context, productID string) (resourcetype.Tree, error) {
	if err := s.repo.EnsureOrganization(txCtx, productID); err != nil {
		return resourcetype.Tree{}, err
	}
	if err := s.repo.FindOrganization(transactor.ForNoKeyUpdate(txCtx), productID); err != nil {
		return resourcetype.Tree{}, err
	}
	return s.tree(txCtx, productID)
}

func (s *service) tree(txCtx context.Context, productID string) (resourcetype.Tree, error) {
	types, err := s.repo.FindTree(txCtx, productID)
	if err != nil {
		return resourcetype.Tree{}, err
	}
	return resourcetype.NewTree(types), nil
}

func (s *service) SeedOrganization(txCtx context.Context, productID string) error {
	return s.repo.EnsureOrganization(txCtx, productID)
}

func (s *service) Create(
	ctx context.Context, input resourcetype.CreateInput,
) (resourcetype.ResourceType, error) {
	if err := input.Validate(); err != nil {
		return resourcetype.ResourceType{}, err
	}
	created := resourcetype.ResourceType{
		ProductID:     input.ProductID,
		Slug:          input.Slug,
		Name:          input.Name,
		Description:   input.Description,
		ParentTypes:   input.ParentTypes,
		NestsInItself: input.NestsInItself,
	}
	created.ID = resourcetype.NewID()

	var result resourcetype.ResourceType
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, err := s.writableTree(txCtx, input.ProductID)
		if err != nil {
			return err
		}
		if tree.BySlug(input.Slug).IsPresent() {
			return resourcetype.NewSlugExistsError(input.Slug)
		}
		if len(tree.Types())-1 >= resourcetype.MaxOwnTypes {
			return resourcetype.ErrLimitReached
		}
		if err = resourcetype.NewTree(append(tree.Types(), created)).Validate(); err != nil {
			return err
		}
		if err = s.repo.Create(txCtx, created); err != nil {
			if errors.Is(err, repository.ErrSlugExists) {
				return resourcetype.NewSlugExistsError(input.Slug)
			}
			return err
		}
		result, err = s.reread(txCtx, input.ProductID, created.ID)
		if err != nil {
			return err
		}
		return s.emit(txCtx, events.ProductResourceTypeCreated, result)
	})
	if err != nil {
		return resourcetype.ResourceType{}, s.unexpected(err, input.ProductID, "create")
	}
	return result, nil
}

func (s *service) Get(ctx context.Context, productID, id string) (resourcetype.ResourceType, error) {
	if productID == "" || id == "" {
		return resourcetype.ResourceType{}, resourcetype.ErrNotFound
	}
	tree, err := s.Tree(ctx, productID)
	if err != nil {
		return resourcetype.ResourceType{}, s.unexpected(err, productID, "get")
	}
	return tree.ByID(id).ToResult(resourcetype.ErrNotFound).Value()
}

func (s *service) Update(
	ctx context.Context, input resourcetype.UpdateInput,
) (resourcetype.ResourceType, error) {
	if err := input.Validate(); err != nil {
		return resourcetype.ResourceType{}, err
	}
	var result resourcetype.ResourceType
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, err := s.writableTree(txCtx, input.ProductID)
		if err != nil {
			return err
		}
		current, found := tree.ByID(input.ID).Get()
		if !found {
			return resourcetype.ErrNotFound
		}
		changed := applyUpdate(current, input)
		if !differs(current, changed) {
			result = current
			return nil
		}
		if current.BuiltIn() && !onlySwitchChanged(current, changed) {
			return resourcetype.ErrBuiltIn
		}
		if reshapes(current, changed) {
			if err = s.checkReshape(txCtx, input.ProductID, replaceType(tree, changed), changed.Slug); err != nil {
				return err
			}
			if err = s.checkResourceParents(txCtx, tree, current, changed); err != nil {
				return err
			}
		}
		if err = s.repo.Update(txCtx, changed); err != nil {
			return err
		}
		result, err = s.reread(txCtx, input.ProductID, input.ID)
		if err != nil {
			return err
		}
		return s.emit(txCtx, events.ProductResourceTypeUpdated, result)
	})
	if err != nil {
		return resourcetype.ResourceType{}, s.unexpected(err, input.ProductID, "update")
	}
	return result, nil
}

func (s *service) Delete(ctx context.Context, productID, id string) error {
	if productID == "" || id == "" {
		return resourcetype.ErrNotFound
	}
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, err := s.writableTree(txCtx, productID)
		if err != nil {
			return err
		}
		current, found := tree.ByID(id).Get()
		if !found {
			return resourcetype.ErrNotFound
		}
		if current.BuiltIn() {
			return resourcetype.ErrBuiltIn
		}
		if err = s.checkUnused(txCtx, current, true); err != nil {
			return err
		}
		if err = s.deleteTypes(txCtx, productID, current); err != nil {
			return err
		}
		return s.emit(txCtx, events.ProductResourceTypeDeleted, current)
	})
	return s.unexpected(err, productID, "delete")
}

func (s *service) Search(
	ctx context.Context, input resourcetype.SearchInput,
) (search.Result[resourcetype.ResourceType], error) {
	if err := input.Validate(); err != nil {
		return search.Result[resourcetype.ResourceType]{}, err
	}
	if _, err := s.Tree(ctx, input.ProductID); err != nil {
		return search.Result[resourcetype.ResourceType]{}, s.unexpected(err, input.ProductID, "search")
	}
	result, err := s.repo.Search(ctx, input.ProductID, input.Request)
	if err != nil {
		return search.Result[resourcetype.ResourceType]{}, s.unexpected(err, input.ProductID, "search")
	}
	return result, nil
}

// ReplaceTree applies a whole tree: new rows first without parents, so any
// order of parents in the body resolves, then every surviving type's row and
// edges, then the removed types in one statement.
func (s *service) ReplaceTree(
	ctx context.Context, input resourcetype.ReplaceTreeInput,
) ([]resourcetype.ResourceType, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	var result []resourcetype.ResourceType
	err := s.transactor.InTx(ctx, func(txCtx context.Context) error {
		tree, err := s.writableTree(txCtx, input.ProductID)
		if err != nil {
			return err
		}
		plan := planTree(tree, input)
		next := resourcetype.NewTree(plan.next)
		if err = s.checkReshape(txCtx, input.ProductID, next, ""); err != nil {
			return err
		}
		for _, removed := range plan.removed {
			if err = s.checkUnused(txCtx, removed, false); err != nil {
				return err
			}
		}
		for _, updated := range plan.updated {
			current, _ := tree.BySlug(updated.Slug).Get()
			if err = s.checkResourceParents(txCtx, tree, current, updated); err != nil {
				return err
			}
		}
		for _, created := range plan.created {
			withoutParents := created
			withoutParents.ParentTypes = nil
			if err = s.repo.Create(txCtx, withoutParents); err != nil {
				return err
			}
		}
		for _, rt := range append(plan.created, plan.updated...) {
			if err = s.repo.Update(txCtx, rt); err != nil {
				return err
			}
		}
		if err = s.deleteTypes(txCtx, input.ProductID, plan.removed...); err != nil {
			return err
		}
		stored, err := s.repo.FindTree(txCtx, input.ProductID)
		if err != nil {
			return err
		}
		storedTree := resourcetype.NewTree(stored)
		for _, change := range []struct {
			eventType events.Type
			types     []resourcetype.ResourceType
		}{
			{events.ProductResourceTypeCreated, plan.created},
			{events.ProductResourceTypeUpdated, plan.updated},
		} {
			for _, rt := range change.types {
				written, found := storedTree.BySlug(rt.Slug).Get()
				if !found {
					return errors.New("resource type missing after the tree write")
				}
				if err = s.emit(txCtx, change.eventType, written); err != nil {
					return err
				}
			}
		}
		for _, removed := range plan.removed {
			if err = s.emit(txCtx, events.ProductResourceTypeDeleted, removed); err != nil {
				return err
			}
		}
		result = storedTree.Types()
		return nil
	})
	if err != nil {
		return nil, s.unexpected(err, input.ProductID, "replace tree")
	}
	return result, nil
}

type treePlan struct {
	next    []resourcetype.ResourceType
	created []resourcetype.ResourceType
	updated []resourcetype.ResourceType
	removed []resourcetype.ResourceType
}

func planTree(tree resourcetype.Tree, input resourcetype.ReplaceTreeInput) treePlan {
	var plan treePlan
	listed := map[string]bool{}
	root, _ := tree.Organization().Get()
	plan.next = append(plan.next, root)
	for _, item := range input.Types {
		listed[item.Slug] = true
		desired := resourcetype.ResourceType{
			ProductID:     input.ProductID,
			Slug:          item.Slug,
			Name:          item.Name,
			Description:   item.Description,
			ParentTypes:   item.ParentTypes,
			NestsInItself: item.NestsInItself,
		}
		current, exists := tree.BySlug(item.Slug).Get()
		switch {
		case !exists:
			desired.ID = resourcetype.NewID()
			plan.created = append(plan.created, desired)
		case differs(current, withIdentity(desired, current)):
			desired = withIdentity(desired, current)
			plan.updated = append(plan.updated, desired)
		default:
			desired = current
		}
		plan.next = append(plan.next, desired)
	}
	for _, rt := range tree.Types() {
		if !rt.BuiltIn() && !listed[rt.Slug] {
			plan.removed = append(plan.removed, rt)
		}
	}
	return plan
}

func withIdentity(desired, current resourcetype.ResourceType) resourcetype.ResourceType {
	desired.ID = current.ID
	desired.AllowsCustomRoles = current.AllowsCustomRoles
	desired.CreatedAt = current.CreatedAt
	desired.UpdatedAt = current.UpdatedAt
	return desired
}

func applyUpdate(current resourcetype.ResourceType, input resourcetype.UpdateInput) resourcetype.ResourceType {
	changed := current
	changed.ParentTypes = slices.Clone(current.ParentTypes)
	if input.Name != nil {
		changed.Name = *input.Name
	}
	if input.Description != nil {
		changed.Description = input.Description
	}
	if input.ParentTypes != nil {
		changed.ParentTypes = slices.Clone(input.ParentTypes)
	}
	if input.NestsInItself != nil {
		changed.NestsInItself = *input.NestsInItself
	}
	if input.AllowsCustomRoles != nil {
		changed.AllowsCustomRoles = *input.AllowsCustomRoles
	}
	return changed
}

// onlySwitchChanged lets the built-in organization type change its custom
// roles switch and nothing else.
func onlySwitchChanged(a, b resourcetype.ResourceType) bool {
	b.AllowsCustomRoles = a.AllowsCustomRoles
	return !differs(a, b)
}

func differs(a, b resourcetype.ResourceType) bool {
	pa, pb := slices.Clone(a.ParentTypes), slices.Clone(b.ParentTypes)
	slices.Sort(pa)
	slices.Sort(pb)
	return a.Name != b.Name ||
		functional.FromPtr(a.Description).OrElse("") != functional.FromPtr(b.Description).OrElse("") ||
		a.NestsInItself != b.NestsInItself ||
		a.AllowsCustomRoles != b.AllowsCustomRoles ||
		!slices.Equal(pa, pb)
}

// reshapes reports a change that moves a scope boundary: the types below a
// type depend only on parent types and nesting, never on names.
func reshapes(a, b resourcetype.ResourceType) bool {
	pa, pb := slices.Clone(a.ParentTypes), slices.Clone(b.ParentTypes)
	slices.Sort(pa)
	slices.Sort(pb)
	return a.NestsInItself != b.NestsInItself || !slices.Equal(pa, pb)
}

func replaceType(tree resourcetype.Tree, changed resourcetype.ResourceType) resourcetype.Tree {
	types := functional.Slice(tree.Types()).Map(func(rt resourcetype.ResourceType) resourcetype.ResourceType {
		if rt.ID == changed.ID {
			return changed
		}
		return rt
	})
	return resourcetype.NewTree(types)
}

// checkReshape validates a candidate tree and refuses it when a role would
// then hold a permission that is no longer at or below the role's scope.
func (s *service) checkReshape(
	ctx context.Context, productID string, next resourcetype.Tree, slug string,
) error {
	if err := next.Validate(); err != nil {
		return err
	}
	roles, err := s.repo.FindRolePermissionScopes(ctx, productID)
	if err != nil {
		return err
	}
	root, _ := next.Organization().Get()
	scopeOf := func(id string) string {
		return next.ByID(id).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).OrElse(root.Slug)
	}
	var outside []string
	for _, role := range roles {
		below := next.Below(scopeOf(role.ResourceTypeID))
		for _, permissionType := range role.PermissionTypeIDs {
			if !below[scopeOf(permissionType)] {
				outside = append(outside, role.RoleID)
				break
			}
		}
	}
	if len(outside) > 0 {
		slices.Sort(outside)
		if slug == "" {
			return resourcetype.NewTreeRolesOutOfScopeError(outside)
		}
		return resourcetype.NewRolesOutOfScopeError(slug, outside)
	}
	return nil
}

// checkResourceParents refuses a change that takes a parent type away from
// registered resources of the type (ADR-0022 A5): a parent type removed, or
// nesting turned off, while a resource still sits under one.
func (s *service) checkResourceParents(
	ctx context.Context, tree resourcetype.Tree, before, after resourcetype.ResourceType,
) error {
	var removed []string
	for _, parent := range before.ParentTypes {
		if !slices.Contains(after.ParentTypes, parent) {
			removed = append(removed, parent)
		}
	}
	if before.NestsInItself && !after.NestsInItself {
		removed = append(removed, before.Slug)
	}
	slices.Sort(removed)
	for _, slug := range removed {
		parent, found := tree.BySlug(slug).Get()
		if !found {
			continue
		}
		count, err := s.repo.CountResourcesUnder(ctx, before.ProductID, before.ID, parent)
		if err != nil {
			return err
		}
		if count > 0 {
			return resourcetype.NewResourcesUnderParentError(before.Slug, slug, count)
		}
	}
	return nil
}

// checkUnused refuses to delete a type that roles, permissions or registered
// resources use. withChildren also counts child types; a whole-tree replacement leaves
// them to the tree validation, because its removed children go with it.
func (s *service) checkUnused(ctx context.Context, rt resourcetype.ResourceType, withChildren bool) error {
	usage, err := s.repo.CountUsage(ctx, rt.ProductID, rt.ID)
	if err != nil {
		return err
	}
	if !withChildren {
		usage.ChildTypes = 0
	}
	if usage.InUse() {
		return resourcetype.NewInUseError(rt.Slug, usage)
	}
	return nil
}

func (s *service) deleteTypes(ctx context.Context, productID string, types ...resourcetype.ResourceType) error {
	ids := functional.Slice(types).Map(func(rt resourcetype.ResourceType) string { return rt.ID })
	err := s.repo.Delete(ctx, productID, ids...)
	if errors.Is(err, repository.ErrInUse) && len(types) > 0 {
		// The foreign key aborted the transaction, so nothing can be counted.
		return resourcetype.NewInUseError(types[0].Slug, resourcetype.Usage{})
	}
	return err
}

func (s *service) reread(ctx context.Context, productID, id string) (resourcetype.ResourceType, error) {
	types, err := s.repo.FindTree(ctx, productID)
	if err != nil {
		return resourcetype.ResourceType{}, err
	}
	found, ok := resourcetype.NewTree(types).ByID(id).Get()
	if !ok {
		return resourcetype.ResourceType{}, errors.New("resource type missing after its write")
	}
	return found, nil
}

func (s *service) emit(ctx context.Context, eventType events.Type, rt resourcetype.ResourceType) error {
	return s.events.Emit(ctx, events.Event{
		Type:      eventType,
		ProductID: rt.ProductID,
		Data: events.Data{
			events.FieldResourceTypeID: rt.ID,
			events.FieldResourceType:   rt.Slug,
		},
	})
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
		Msg("resource type operation failed")
	return fault.ErrUnexpected
}
