package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/platform"
	"anchor/internal/security"
)

func (s *AnchorAPI) SearchPlatformUsers(
	ctx context.Context, request SearchPlatformUsersRequestObject,
) (SearchPlatformUsersResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	searchReqBody := request.Body
	if searchReqBody == nil {
		searchReqBody = &SearchPlatformUsersJSONRequestBody{}
	}

	searchRequest := mapToSearchPlatformUserInput(searchReqBody)

	input := platform.SearchPlatformUsersInput{
		TenantID: tenantID,
		Request:  searchRequest,
	}

	result, err := s.PlatformUserService.SearchPlatformUsers(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to search platform users")
		return nil, err
	}

	users := make([]PlatformUserResponse, len(result.Items))
	for i, item := range result.Items {
		users[i] = mapPlatformUserToResponse(item)
	}

	return SearchPlatformUsers200JSONResponse{
		Items: users,
		Total: result.Total,
		Count: result.Count,
	}, nil
}

func (s *AnchorAPI) DeletePlatformUser(
	ctx context.Context, request DeletePlatformUserRequestObject,
) (DeletePlatformUserResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := platform.DeletePlatformUserInput{
		TenantID:       tenantID,
		PlatformUserID: request.PlatformUserId,
	}

	err = s.PlatformUserService.DeletePlatformUser(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Msg("failed to delete platform user")
		return nil, err
	}

	return DeletePlatformUser204Response{}, nil
}

func (s *AnchorAPI) GetPlatformUser(
	ctx context.Context, request GetPlatformUserRequestObject,
) (GetPlatformUserResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := platform.GetPlatformUserInput{
		TenantID:       tenantID,
		PlatformUserID: request.PlatformUserId,
	}

	user, err := s.PlatformUserService.GetPlatformUser(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str(
			"platformUserId", request.PlatformUserId,
		).Msg("failed to get platform user")
		return nil, err
	}

	if user == nil {
		return GetPlatformUser404Response{}, nil
	}

	return GetPlatformUser200JSONResponse(
		mapPlatformUserToResponse(*user),
	), nil
}

func (s *AnchorAPI) GetCurrentUser(
	ctx context.Context, _ GetCurrentUserRequestObject,
) (GetCurrentUserResponseObject, error) {
	userID, err := security.GetCurrentUserID(ctx)
	if err != nil {
		return nil, err
	}
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	input := platform.GetPlatformUserByUserIDInput{
		TenantID: tenantID,
		UserID:   userID,
	}

	user, err := s.PlatformUserService.GetPlatformUserByUserID(ctx, input)
	if err != nil {
		logAPIError(s.logger, err).Str("userID", userID).Str(
			"tenantID", tenantID,
		).Msg("failed to get current user")
		return nil, err
	}

	if user == nil {
		return GetCurrentUser401JSONResponse{}, nil
	}

	return GetCurrentUser200JSONResponse(mapAuthUserToAPIUserResponse(user)), nil
}

// mapToSearchPlatformUserInput builds a search.Request for platform users.
func mapToSearchPlatformUserInput(
	searchReqBody *SearchPlatformUsersJSONRequestBody,
) search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser] {
	var filter *platform.SearchPlatformUserFilter
	if searchReqBody.Filter != nil {
		filter = &platform.SearchPlatformUserFilter{
			Emails: searchReqBody.Filter.Emails,
			IDs:    searchReqBody.Filter.Ids,
			Roles:  searchReqBody.Filter.Roles,
		}
	}
	var req search.Request[platform.SearchPlatformUserFilter, platform.SortFieldPlatformUser]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}

func mapPlatformUserToResponse(
	user platform.User,
) PlatformUserResponse {
	return PlatformUserResponse{
		Id:        user.ID,
		Email:     user.Email,
		Role:      user.Role,
		UserId:    user.UserID,
		TenantId:  user.PlatformTenantID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}
