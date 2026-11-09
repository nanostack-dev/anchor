package api

import (
	"context"
	"log/slog"
	"strings"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/platform"
	"anchor/internal/repository"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
)

func (s *AnchorAPI) SearchPlatformUsers(
	ctx context.Context, request SearchPlatformUsersRequestObject,
) (SearchPlatformUsersResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}

	body := request.Body
	if body == nil {
		body = &SearchPlatformUsersJSONRequestBody{}
	}
	if err = validatePlatformUsersSearch(tenantID, body); err != nil {
		return nil, err
	}
	q := platformUserSearch(tenantID, body)

	result, err := s.PlatformUserService.SearchPlatformUsers(ctx, q)
	if err != nil {
		log.ErrorTo(ctx, s.logger, err, "failed to search platform users")
		return nil, err
	}

	return SearchPlatformUsers200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapPlatformUserToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      result.Count,
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
		log.ErrorTo(ctx, s.logger, err, "failed to delete platform user")
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
		log.ErrorTo(ctx, s.logger, err, "failed to get platform user",
			slog.String("platformUserId", request.PlatformUserId),
		)
		return nil, err
	}

	if user == nil {
		return GetPlatformUser404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PLATFORM_USER_NOT_FOUND", "Platform User does not exist."),
		)}, nil
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
		log.ErrorTo(ctx, s.logger, err, "failed to get current user",
			slog.String("userID", userID),
			slog.String("tenantID", tenantID),
		)
		return nil, err
	}

	if user == nil {
		return GetCurrentUser401JSONResponse{UnauthorizedJSONResponse(*fault.ErrUnauthorized)}, nil
	}

	return GetCurrentUser200JSONResponse(mapAuthUserToAPIUserResponse(user)), nil
}

// platformUserSearch builds the search from the request body. The tenant scope
// comes from the authenticated principal; the body never sets it.
func platformUserSearch(
	tenantID string, body *SearchPlatformUsersJSONRequestBody,
) repository.PlatformUserSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	// A role compares without regard to letter case.
	roles := functional.Slice(f.Roles).Map(func(role platform.TenantRole) string {
		return strings.ToLower(string(role))
	})
	q := repository.NewPlatformUserSearch(tenantID)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Email().In, f.Emails)
	q = searchexec.Some(q, q.Role().In, roles)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.PlatformUserSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
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
