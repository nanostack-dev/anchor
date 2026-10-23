package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"anchor/internal/domain/workspace"
	"anchor/internal/searchx"
)

func mapWorkspaceMemberToResponse(m workspace.Membership) WorkspaceMemberResponse {
	response := WorkspaceMemberResponse{
		WorkspaceId:    m.WorkspaceID,
		OrganizationId: m.OrganizationID,
		ProductUserId:  m.ProductUserID,
		Email:          openapi_types.Email(m.Email),
		Name:           m.Name,
		AssignedAt:     m.AssignedAt,
	}
	response.Role.Id = m.RoleID
	response.Role.Name = m.RoleName
	return response
}

func (s *AnchorAPI) GetWorkspaceMember(
	ctx context.Context, request GetWorkspaceMemberRequestObject,
) (GetWorkspaceMemberResponseObject, error) {
	member, err := s.WorkspaceMemberService.Find(ctx, workspace.GetMemberInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		WorkspaceID:    request.WorkspaceId,
		ProductUserID:  request.ProductUserId,
	})
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to get workspace member")
		return nil, err
	}

	return GetWorkspaceMember200JSONResponse(mapWorkspaceMemberToResponse(member)), nil
}

func (s *AnchorAPI) AssignWorkspaceMemberRole(
	ctx context.Context, request AssignWorkspaceMemberRoleRequestObject,
) (AssignWorkspaceMemberRoleResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	member, err := s.WorkspaceMemberService.Assign(ctx, workspace.AssignMemberRoleInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		WorkspaceID:    request.WorkspaceId,
		ProductUserID:  request.ProductUserId,
		RoleID:         request.Body.RoleId,
	})
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to assign workspace member role")
		return nil, err
	}

	return AssignWorkspaceMemberRole200JSONResponse(mapWorkspaceMemberToResponse(member)), nil
}

func (s *AnchorAPI) RemoveWorkspaceMember(
	ctx context.Context, request RemoveWorkspaceMemberRequestObject,
) (RemoveWorkspaceMemberResponseObject, error) {
	err := s.WorkspaceMemberService.Remove(ctx, workspace.RemoveMemberInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		WorkspaceID:    request.WorkspaceId,
		ProductUserID:  request.ProductUserId,
	})
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to remove workspace member")
		return nil, err
	}

	return RemoveWorkspaceMember204Response{}, nil
}

func (s *AnchorAPI) SearchWorkspaceMembers(
	ctx context.Context, request SearchWorkspaceMembersRequestObject,
) (SearchWorkspaceMembersResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	result, err := s.WorkspaceMemberService.Search(ctx, workspace.SearchMembersInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		WorkspaceID:    request.WorkspaceId,
		Request:        mapToSearchWorkspaceMembersInput(request.Body),
	})
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to search workspace members")
		return nil, err
	}

	return SearchWorkspaceMembers200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapWorkspaceMemberToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      len(result.Items),
	}, nil
}

func mapToSearchWorkspaceMembersInput(
	body *SearchWorkspaceMembersJSONRequestBody,
) search.Request[workspace.SearchMembersFilter, workspace.SortFieldMember] {
	filter := functional.FromPtr(body.Filter).Map(func(f WorkspaceMemberFilter) workspace.SearchMembersFilter {
		return workspace.SearchMembersFilter{
			ProductUserIDs: functional.FromPtr(f.ProductUserIds).OrElse(nil),
			RoleIDs:        functional.FromPtr(f.RoleIds).OrElse(nil),
		}
	}).ToPtr()

	sortBy, sortDirection := searchx.Sort(
		body.SortBy, body.SortDirection, workspace.SortFieldMemberAssignedAt, search.SortDescending,
	)

	return search.NewRequest[workspace.SearchMembersFilter, workspace.SortFieldMember]().
		WithFilter(filter).
		WithSort(sortBy, sortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}
