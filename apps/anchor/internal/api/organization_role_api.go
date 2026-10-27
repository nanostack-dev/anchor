package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/product/role"
)

func (s *AnchorAPI) SearchOrganizationRoles(
	ctx context.Context, request SearchOrganizationRolesRequestObject,
) (SearchOrganizationRolesResponseObject, error) {
	body := request.Body
	filter := functional.FromPtr(body.Filter).Map(func(f OrganizationRoleFilter) role.SearchProductRoleFilter {
		return role.SearchProductRoleFilter{
			ProductRoleIDs: f.Ids,
			Names:          f.Names,
			Permissions:    f.Permissions,
			ResourceTypes:  f.ResourceTypes,
			Types:          f.Types,
		}
	}).ToPtr()
	result, err := s.OrganizationRoleService.SearchOrganizationRoles(ctx, role.SearchOrganizationRolesInput{
		OrganizationID: request.OrganizationId,
		ProductID:      request.ProductId,
		Request: search.NewRequest[role.SearchProductRoleFilter, role.SortFieldProductRole]().
			WithFilter(filter).
			WithSort(body.SortBy, body.SortDirection).
			WithFullTextSearch(body.FullTextSearch).
			WithPagination(body.Pagination),
	})
	if err != nil {
		return nil, err
	}
	return SearchOrganizationRoles200JSONResponse(ProductRoleListResponse{
		Count:      result.Count,
		Items:      functional.Slice(result.Items).Map(mapProductRoleToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}

func (s *AnchorAPI) GetOrganizationRole(
	ctx context.Context, request GetOrganizationRoleRequestObject,
) (GetOrganizationRoleResponseObject, error) {
	found, err := s.OrganizationRoleService.GetOrganizationRole(ctx, role.OrganizationRoleRef{
		ProductID: request.ProductId, OrganizationID: request.OrganizationId, ID: request.RoleId,
	})
	if err != nil {
		return nil, err
	}
	return GetOrganizationRole200JSONResponse(mapProductRoleToResponse(found)), nil
}

func (s *AnchorAPI) CreateOrganizationRole(
	ctx context.Context, request CreateOrganizationRoleRequestObject,
) (CreateOrganizationRoleResponseObject, error) {
	created, err := s.OrganizationRoleService.CreateOrganizationRole(ctx, role.CreateOrganizationRoleInput{
		OrganizationID: request.OrganizationId,
		ProductID:      request.ProductId,
		Name:           request.Body.Name,
		Description:    functional.FromPtr(request.Body.Description).OrElse(""),
		ResourceType:   request.Body.ResourceType,
		Permissions:    request.Body.Permissions,
	})
	if err != nil {
		return nil, err
	}
	return CreateOrganizationRole201JSONResponse(mapProductRoleToResponse(created)), nil
}

func (s *AnchorAPI) UpdateOrganizationRole(
	ctx context.Context, request UpdateOrganizationRoleRequestObject,
) (UpdateOrganizationRoleResponseObject, error) {
	updated, err := s.OrganizationRoleService.UpdateOrganizationRole(ctx, role.UpdateOrganizationRoleInput{
		OrganizationID: request.OrganizationId,
		ProductID:      request.ProductId,
		ID:             request.RoleId,
		Name:           &request.Body.Name,
		Description:    request.Body.Description,
		ResourceType:   request.Body.ResourceType,
		Permissions:    s.mapToProductRolePermissionSlice(request.ProductId, request.RoleId, request.Body.Permissions),
	})
	if err != nil {
		return nil, err
	}
	return UpdateOrganizationRole200JSONResponse(mapProductRoleToResponse(updated)), nil
}

func (s *AnchorAPI) DeleteOrganizationRole(
	ctx context.Context, request DeleteOrganizationRoleRequestObject,
) (DeleteOrganizationRoleResponseObject, error) {
	if err := s.OrganizationRoleService.DeleteOrganizationRole(ctx, role.OrganizationRoleRef{
		ProductID: request.ProductId, OrganizationID: request.OrganizationId, ID: request.RoleId,
	}); err != nil {
		return nil, err
	}
	return DeleteOrganizationRole204Response{}, nil
}
