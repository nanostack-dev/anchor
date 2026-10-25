package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/organizationrole"
)

func mapOrganizationRoleToResponse(role organizationrole.Role) OrganizationRoleResponse {
	return OrganizationRoleResponse{
		Id:             role.PublicID(),
		OrganizationId: role.OrganizationID,
		TemplateId:     role.TemplateID,
		Key:            role.Key,
		Name:           role.Name,
		Description:    functional.OptionOf(role.Description, role.Description != "").ToPtr(),
		Origin:         role.Origin(),
		ResourceType:   role.ResourceType,
		IsDefault:      role.IsDefault,
		Permissions:    append([]string{}, role.Permissions...),
		CreatedAt:      role.CreatedAt,
		UpdatedAt:      role.UpdatedAt,
	}
}

func (s *AnchorAPI) SearchOrganizationRoles(
	ctx context.Context, request SearchOrganizationRolesRequestObject,
) (SearchOrganizationRolesResponseObject, error) {
	filter := functional.FromPtr(request.Body.Filter).
		Map(func(f OrganizationRoleFilter) organizationrole.Filter {
			return organizationrole.Filter{IDs: f.Ids, Keys: f.Keys, ResourceTypes: f.ResourceTypes}
		}).
		ToPtr()
	result, err := s.OrganizationRoleService.Search(ctx, organizationrole.SearchInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Request: search.NewRequest[organizationrole.Filter, organizationrole.SortField]().
			WithFullTextSearch(request.Body.FullTextSearch).
			WithFilter(filter).
			WithSort(request.Body.SortBy, request.Body.SortDirection).
			WithPagination(request.Body.Pagination),
	})
	if err != nil {
		return nil, err
	}
	return SearchOrganizationRoles200JSONResponse(OrganizationRoleListResponse{
		Items:      functional.Slice(result.Items).Map(mapOrganizationRoleToResponse),
		Count:      result.Count,
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
	}), nil
}

func (s *AnchorAPI) GetOrganizationRole(
	ctx context.Context, request GetOrganizationRoleRequestObject,
) (GetOrganizationRoleResponseObject, error) {
	role, err := s.OrganizationRoleService.Get(ctx, request.ProductId, request.OrganizationId, request.RoleId)
	if err != nil {
		return nil, err
	}
	return GetOrganizationRole200JSONResponse(mapOrganizationRoleToResponse(role)), nil
}

func (s *AnchorAPI) UpdateOrganizationRole(
	ctx context.Context, request UpdateOrganizationRoleRequestObject,
) (UpdateOrganizationRoleResponseObject, error) {
	role, err := s.OrganizationRoleService.Update(ctx, organizationrole.UpdateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		RoleID:         request.RoleId,
		IsDefault:      request.Body.IsDefault,
	})
	if err != nil {
		return nil, err
	}
	return UpdateOrganizationRole200JSONResponse(mapOrganizationRoleToResponse(role)), nil
}
