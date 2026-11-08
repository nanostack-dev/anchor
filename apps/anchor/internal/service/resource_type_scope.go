package service

import (
	"context"
	"log/slog"

	"github.com/nanostack-dev/nanostack-framework/core/log"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	resourcepermission "anchor/internal/domain/product/resource_permission"
	"anchor/internal/domain/product/role"
	"anchor/internal/domain/resourcetype"
	rtservice "anchor/internal/resourcetype/service"
	"anchor/internal/searchx"
)

// scopeSlug names the scope a stored role or permission holds.
func scopeSlug(tree resourcetype.Tree, id string) string {
	return tree.ByID(id).Map(func(rt resourcetype.ResourceType) string { return rt.Slug }).
		OrElse(resourcetype.OrganizationSlug)
}

func withPermissionScope(
	tree resourcetype.Tree,
) func(resourcepermission.ProductResourcePermission) resourcepermission.ProductResourcePermission {
	return func(perm resourcepermission.ProductResourcePermission) resourcepermission.ProductResourcePermission {
		perm.ResourceType = scopeSlug(tree, perm.ResourceTypeID)
		return perm
	}
}

func withRoleScope(tree resourcetype.Tree) func(role.ProductRole) role.ProductRole {
	return func(r role.ProductRole) role.ProductRole {
		r.ResourceType = scopeSlug(tree, r.ResourceTypeID)
		return r
	}
}

// checkPermissionScopes refuses a permission that is neither scoped to the role's
// type nor to a type below it (ADR-0022 D1).
func checkPermissionScopes(
	tree resourcetype.Tree, roleScope string, permissions []resourcepermission.ProductResourcePermission,
) error {
	below := tree.Below(roleScope)
	outside, found := functional.Slice(permissions).
		FindFirst(func(perm resourcepermission.ProductResourcePermission) bool {
			return !below[scopeSlug(tree, perm.ResourceTypeID)]
		}).
		Get()
	if found {
		return role.NewPermissionOutsideScopeError(outside.Name, scopeSlug(tree, outside.ResourceTypeID), roleScope)
	}
	return nil
}

// scopedSearch runs a role or permission search: it resolves the filter's
// resource_types against the product's tree, then names each item's scope.
func scopedSearch[F any, S ~string, T any](
	ctx context.Context, types rtservice.Service, logger *slog.Logger, productID string,
	request search.Request[F, S],
	resolve func(F, resourcetype.Tree) F,
	run func(context.Context, string, search.Request[F, S]) (search.Result[T], error),
	withScope func(resourcetype.Tree) func(T) T,
) (search.Result[T], error) {
	tree, err := types.Tree(ctx, productID)
	if err != nil {
		logger.ErrorContext(ctx, "failed to read resource types",
			slog.String("product_id", productID),
			log.Err(err),
		)
		return search.Result[T]{}, fault.ErrUnexpected
	}
	if request.Filter != nil {
		resolved := resolve(*request.Filter, tree)
		request.Filter = &resolved
	}
	result, err := run(ctx, productID, request)
	if err != nil {
		if searchx.IsClientError(err) {
			return search.Result[T]{}, err
		}
		logger.ErrorContext(ctx, "search failed", slog.String("product_id", productID), log.Err(err))
		return search.Result[T]{}, fault.ErrUnexpected
	}
	result.Items = functional.Slice(result.Items).Map(withScope(tree))
	return result, nil
}
