package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/search"

	"anchor/internal/domain/invitation"
	"anchor/internal/domain/platform"
	"anchor/internal/security"
)

func (s *AnchorAPI) CreatePlatformInvitation(
	ctx context.Context, request CreatePlatformInvitationRequestObject,
) (CreatePlatformInvitationResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	createInvitation, err := s.PlatformInvitationService.CreateInvitation(
		ctx, invitation.CreateInvitationInput{
			TenantID: tenantID,
			Email:    request.Body.Email,
			Role:     platform.TenantRoleAdmin,
		},
	)
	if err != nil {
		return nil, err
	}

	return CreatePlatformInvitation201JSONResponse(
		mapPlatformInvitationToResponse(
			createInvitation,
		),
	), nil
}

func (s *AnchorAPI) GetPlatformInvitation(
	ctx context.Context, request GetPlatformInvitationRequestObject,
) (GetPlatformInvitationResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	invitationID := request.InvitationId
	result, err := s.PlatformInvitationService.SearchInvitation(
		ctx, invitation.SearchInvitationInput{
			TenantID: tenantID,
			Request: search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]{
				Filter: &invitation.SearchPlatformInvitationFilter{
					IDs: []string{invitationID},
				},
				Pagination: search.Pagination{
					Limit:  1,
					Offset: 0,
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}
	if result.Count == 0 {
		return GetPlatformInvitation404Response{}, nil
	}
	return GetPlatformInvitation200JSONResponse(
		mapPlatformInvitationToResponse(result.Items[0]),
	), nil
}

func (s *AnchorAPI) DeletePlatformInvitation(
	ctx context.Context, request DeletePlatformInvitationRequestObject,
) (DeletePlatformInvitationResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	err = s.PlatformInvitationService.DeleteInvitation(
		ctx, invitation.DeleteInvitationInput{
			TenantID:     tenantID,
			InvitationID: request.InvitationId,
		},
	)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to delete invitation")
		return nil, err
	}
	return DeletePlatformInvitation204Response{}, nil
}

func (s *AnchorAPI) SearchPlatformInvitations(
	ctx context.Context, request SearchPlatformInvitationsRequestObject,
) (SearchPlatformInvitationsResponseObject, error) {
	tenantID, err := security.GetTenantID(ctx)
	if err != nil {
		return nil, err
	}
	searchReqBody := request.Body

	searchRequest := mapToSearchPlatformInvitationRequest(
		searchReqBody,
	)
	result, err := s.PlatformInvitationService.SearchInvitation(
		ctx, invitation.SearchInvitationInput{
			TenantID: tenantID,
			Request:  searchRequest,
		},
	)
	if err != nil {
		return nil, err
	}
	invitations := make([]PlatformInvitationResponse, len(result.Items))
	for i, item := range result.Items {
		invitations[i] = mapPlatformInvitationToResponse(item)
	}
	return SearchPlatformInvitations200JSONResponse{
		Items: invitations,
		Total: result.Total,
		Count: result.Count,
	}, nil
}

// mapToSearchPlatformInvitationRequest builds a search.Request for platform invitations.
func mapToSearchPlatformInvitationRequest(
	searchReqBody *SearchPlatformInvitationsJSONRequestBody,
) search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation] {
	var filter *invitation.SearchPlatformInvitationFilter
	if searchReqBody.Filter != nil {
		filter = &invitation.SearchPlatformInvitationFilter{
			Emails: searchReqBody.Filter.Emails,
		}
		if searchReqBody.Filter.Ids != nil {
			filter.IDs = searchReqBody.Filter.Ids
		}
	}

	var req search.Request[invitation.SearchPlatformInvitationFilter, invitation.SortFieldPlatformInvitation]
	return req.WithFilter(filter).
		WithSort(
			searchReqBody.SortBy,
			searchReqBody.SortDirection,
		).WithFullTextSearch(searchReqBody.FullTextSearch).WithPagination(
		searchReqBody.Pagination,
	)
}

func mapPlatformInvitationToResponse(
	invitation invitation.PlatformInvitation,
) PlatformInvitationResponse {
	return PlatformInvitationResponse{
		Id:        invitation.ID,
		Code:      invitation.Code,
		Email:     invitation.Email,
		TenantId:  invitation.PlatformTenantID,
		CreatedAt: invitation.CreatedAt,
		UpdatedAt: invitation.UpdatedAt,
	}
}
