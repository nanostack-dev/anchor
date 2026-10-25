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
		Customized:     role.Customized,
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
	role, err := s.OrganizationCustomRoleService.Update(ctx, organizationrole.UpdateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		RoleID:         request.RoleId,
		IsDefault:      request.Body.IsDefault,
		Name:           request.Body.Name,
		Description:    request.Body.Description,
		Permissions:    functional.FromPtr(request.Body.Permissions).OrElse(nil),
		PermissionsSet: request.Body.Permissions != nil,
	})
	if err != nil {
		return nil, err
	}
	return UpdateOrganizationRole200JSONResponse(mapOrganizationRoleToResponse(role)), nil
}

func (s *AnchorAPI) CreateOrganizationRole(
	ctx context.Context, request CreateOrganizationRoleRequestObject,
) (CreateOrganizationRoleResponseObject, error) {
	role, err := s.OrganizationCustomRoleService.Create(ctx, organizationrole.CreateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Key:            functional.FromPtr(request.Body.Key).OrElse(""),
		Name:           request.Body.Name,
		Description:    request.Body.Description,
		ResourceType:   request.Body.ResourceType,
		Permissions:    request.Body.Permissions,
	})
	if err != nil {
		return nil, err
	}
	return CreateOrganizationRole201JSONResponse(mapOrganizationRoleToResponse(role)), nil
}

func (s *AnchorAPI) DeleteOrganizationRole(
	ctx context.Context, request DeleteOrganizationRoleRequestObject,
) (DeleteOrganizationRoleResponseObject, error) {
	if err := s.OrganizationCustomRoleService.Delete(
		ctx, request.ProductId, request.OrganizationId, request.RoleId,
	); err != nil {
		return nil, err
	}
	return DeleteOrganizationRole204Response{}, nil
}

func (s *AnchorAPI) ResetOrganizationRole(
	ctx context.Context, request ResetOrganizationRoleRequestObject,
) (ResetOrganizationRoleResponseObject, error) {
	role, err := s.OrganizationCustomRoleService.Reset(ctx, request.ProductId, request.OrganizationId, request.RoleId)
	if err != nil {
		return nil, err
	}
	return ResetOrganizationRole200JSONResponse(mapOrganizationRoleToResponse(role)), nil
}
