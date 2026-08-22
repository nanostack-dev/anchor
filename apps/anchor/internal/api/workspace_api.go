package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/workspace"
)

func mapWorkspaceToResponse(item workspace.Workspace) ProductWorkspaceResponse {
	return ProductWorkspaceResponse{
		Id:             item.ID,
		OrganizationId: item.OrganizationID,
		Name:           item.Name,
		Description:    item.Description,
		CreatedAt:      item.CreatedAt,
		UpdatedAt:      item.UpdatedAt,
	}
}

func (s *AnchorAPI) CreateOrganizationWorkspace(
	ctx context.Context,
	request CreateOrganizationWorkspaceRequestObject,
) (CreateOrganizationWorkspaceResponseObject, error) {
	created, err := s.WorkspaceService.Create(
		ctx,
		workspace.CreateWorkspaceInput{
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			Name:           request.Body.Name,
			Description:    request.Body.Description,
		},
	)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to create organization workspace")
		return nil, err
	}

	return CreateOrganizationWorkspace201JSONResponse(mapWorkspaceToResponse(created)), nil
}

func (s *AnchorAPI) SearchOrganizationWorkspaces(
	ctx context.Context,
	request SearchOrganizationWorkspacesRequestObject,
) (SearchOrganizationWorkspacesResponseObject, error) {
	searchReqBody := request.Body
	if searchReqBody == nil {
		searchReqBody = &SearchOrganizationWorkspacesJSONRequestBody{}
	}

	result, err := s.WorkspaceService.Search(
		ctx,
		workspace.SearchWorkspacesInput{
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			Request:        mapToSearchWorkspaceInput(searchReqBody),
		},
	)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to search organization workspaces")
		return nil, err
	}

	return SearchOrganizationWorkspaces200JSONResponse{
		Items: functional.Slice(result.Items).Map(mapWorkspaceToResponse),
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func (s *AnchorAPI) GetOrganizationWorkspace(
	ctx context.Context,
	request GetOrganizationWorkspaceRequestObject,
) (GetOrganizationWorkspaceResponseObject, error) {
	found, err := s.WorkspaceService.Find(
		ctx,
		workspace.FindWorkspaceInput{
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			WorkspaceID:    request.WorkspaceId,
		},
	)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to find organization workspace")
		return nil, err
	}
	if found == nil {
		return nil, fault.ErrNotFound
	}

	return GetOrganizationWorkspace200JSONResponse(mapWorkspaceToResponse(*found)), nil
}

func (s *AnchorAPI) UpdateOrganizationWorkspace(
	ctx context.Context,
	request UpdateOrganizationWorkspaceRequestObject,
) (UpdateOrganizationWorkspaceResponseObject, error) {
	updated, err := s.WorkspaceService.Update(
		ctx,
		workspace.UpdateWorkspaceInput{
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			WorkspaceID:    request.WorkspaceId,
			Name:           &request.Body.Name,
			Description:    request.Body.Description,
		},
	)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to update organization workspace")
		return nil, err
	}

	return UpdateOrganizationWorkspace200JSONResponse(mapWorkspaceToResponse(updated)), nil
}

func (s *AnchorAPI) DeleteOrganizationWorkspace(
	ctx context.Context,
	request DeleteOrganizationWorkspaceRequestObject,
) (DeleteOrganizationWorkspaceResponseObject, error) {
	if err := s.WorkspaceService.Delete(
		ctx,
		workspace.DeleteWorkspaceInput{
			ProductID:      request.ProductId,
			OrganizationID: request.OrganizationId,
			WorkspaceID:    request.WorkspaceId,
		},
	); err != nil {
		logAPIError(s.logger, err).Msg("failed to delete organization workspace")
		return nil, err
	}

	return DeleteOrganizationWorkspace204Response{}, nil
}

func mapToSearchWorkspaceInput(
	searchReqBody *SearchOrganizationWorkspacesJSONRequestBody,
) search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace] {
	var filter *workspace.SearchWorkspaceFilter
	if searchReqBody.Filter != nil {
		filter = &workspace.SearchWorkspaceFilter{
			IDs:   searchReqBody.Filter.Ids,
			Names: searchReqBody.Filter.Names,
		}
	}

	var req search.Request[workspace.SearchWorkspaceFilter, workspace.SortFieldProductWorkspace]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}
