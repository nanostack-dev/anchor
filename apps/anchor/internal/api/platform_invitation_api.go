package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/core/log"
	"github.com/nanostack-dev/nanostack-framework/db/search"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"

	"anchor/internal/domain/invitation"
	"anchor/internal/domain/platform"
	"anchor/internal/repository"
	"anchor/internal/security"

	"github.com/nanostack-dev/nanostack-framework/db/searchexec"
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
	q := repository.NewPlatformInvitationSearch(tenantID).ID().In(invitationID).Limit(1)
	result, err := s.PlatformInvitationService.SearchInvitation(ctx, q)
	if err != nil {
		return nil, err
	}
	if result.Count == 0 {
		return GetPlatformInvitation404JSONResponse{NotFoundJSONResponse(
			notFoundBody("PLATFORM_INVITATION_NOT_FOUND", "Platform Invitation does not exist."),
		)}, nil
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
		log.ErrorTo(ctx, s.logger, err, "failed to delete invitation")
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
	result, err := s.PlatformInvitationService.SearchInvitation(
		ctx, platformInvitationSearch(tenantID, request.Body),
	)
	if err != nil {
		return nil, err
	}
	return SearchPlatformInvitations200JSONResponse{
		Items:      functional.Slice(result.Items).Map(mapPlatformInvitationToResponse),
		Total:      result.Total,
		NextCursor: nextCursor(result.NextCursor),
		Count:      result.Count,
	}, nil
}

// platformInvitationSearch builds the search from the request body. The tenant
// scope comes from the authenticated principal; the body never sets it.
func platformInvitationSearch(
	tenantID string, body *SearchPlatformInvitationsJSONRequestBody,
) repository.PlatformInvitationSearch {
	b := searchexec.OrZero(body)
	f := searchexec.OrZero(b.Filter)
	q := repository.NewPlatformInvitationSearch(tenantID)
	q = searchexec.Some(q, q.ID().In, f.Ids)
	q = searchexec.Some(q, q.Email().In, f.Emails)
	q = searchexec.Opt(q, q.FullText, b.FullTextSearch)
	q = searchexec.APISort(q, q.SortBy, b.SortBy, b.SortDirection,
		repository.PlatformInvitationSortCreatedAt, search.SortAscending)
	return searchexec.APIPage(q.Query, b.Pagination)
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
