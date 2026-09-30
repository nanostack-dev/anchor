package api

import (
	"context"

	"github.com/nanostack-dev/nanostack-framework/pkg/fault"
	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
	"github.com/nanostack-dev/nanostack-framework/pkg/search"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"anchor/internal/domain/organizationinvitation"
)

func (s *AnchorAPI) CreateOrganizationInvitation(
	ctx context.Context, request CreateOrganizationInvitationRequestObject,
) (CreateOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	created, err := s.OrganizationInvitationService.Create(ctx, organizationinvitation.CreateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Email:          string(request.Body.Email),
		RoleID:         request.Body.RoleId,
		ExpiresAt:      request.Body.ExpiresAt,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to create organization invitation")
		return nil, err
	}

	return CreateOrganizationInvitation201JSONResponse(mapCreatedInvitationToResponse(created)), nil
}

func (s *AnchorAPI) SearchOrganizationInvitations(
	ctx context.Context, request SearchOrganizationInvitationsRequestObject,
) (SearchOrganizationInvitationsResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	result, err := s.OrganizationInvitationService.Search(ctx, organizationinvitation.SearchInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		Request:        mapSearchOrganizationInvitationsRequestToInput(*request.Body),
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Msg("failed to search organization invitations")
		return nil, err
	}

	return SearchOrganizationInvitations200JSONResponse(OrganizationInvitationListResponse{
		Items: functional.Slice(result.Items).Map(mapInvitationToResponse),
		Total: result.Total,
		Count: len(result.Items),
	}), nil
}

func (s *AnchorAPI) GetOrganizationInvitation(
	ctx context.Context, request GetOrganizationInvitationRequestObject,
) (GetOrganizationInvitationResponseObject, error) {
	invitation, err := s.OrganizationInvitationService.Get(ctx, organizationinvitation.GetInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("invitation_id", request.InvitationId).
			Msg("failed to get organization invitation")
		return nil, err
	}

	return GetOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

func (s *AnchorAPI) UpdateOrganizationInvitation(
	ctx context.Context, request UpdateOrganizationInvitationRequestObject,
) (UpdateOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	invitation, err := s.OrganizationInvitationService.Update(ctx, organizationinvitation.UpdateInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
		RoleID:         request.Body.RoleId,
		ExpiresAt:      request.Body.ExpiresAt,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("invitation_id", request.InvitationId).
			Msg("failed to update organization invitation")
		return nil, err
	}

	return UpdateOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

func (s *AnchorAPI) DeleteOrganizationInvitation(
	ctx context.Context, request DeleteOrganizationInvitationRequestObject,
) (DeleteOrganizationInvitationResponseObject, error) {
	err := s.OrganizationInvitationService.Delete(ctx, organizationinvitation.DeleteInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("invitation_id", request.InvitationId).
			Msg("failed to delete organization invitation")
		return nil, err
	}

	return DeleteOrganizationInvitation204Response{}, nil
}

func (s *AnchorAPI) ResendOrganizationInvitation(
	ctx context.Context, request ResendOrganizationInvitationRequestObject,
) (ResendOrganizationInvitationResponseObject, error) {
	resent, err := s.OrganizationInvitationService.Resend(ctx, organizationinvitation.ResendInput{
		ProductID:      request.ProductId,
		OrganizationID: request.OrganizationId,
		InvitationID:   request.InvitationId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("organization_id", request.OrganizationId).
			Str("invitation_id", request.InvitationId).
			Msg("failed to resend organization invitation")
		return nil, err
	}

	return ResendOrganizationInvitation200JSONResponse(mapCreatedInvitationToResponse(resent)), nil
}

func (s *AnchorAPI) LookupOrganizationInvitation(
	ctx context.Context, request LookupOrganizationInvitationRequestObject,
) (LookupOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	invitation, err := s.OrganizationInvitationService.Lookup(ctx, organizationinvitation.LookupInput{
		ProductID: request.ProductId,
		Token:     request.Body.Token,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Msg("failed to look up organization invitation")
		return nil, err
	}

	return LookupOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

func (s *AnchorAPI) AcceptOrganizationInvitation(
	ctx context.Context, request AcceptOrganizationInvitationRequestObject,
) (AcceptOrganizationInvitationResponseObject, error) {
	if request.Body == nil {
		return nil, fault.BadRequest("INVALID_REQUEST", "request body is required")
	}

	invitation, err := s.OrganizationInvitationService.Accept(ctx, organizationinvitation.AcceptInput{
		ProductID:     request.ProductId,
		Token:         request.Body.Token,
		ProductUserID: request.Body.ProductUserId,
	})
	if err != nil {
		logAPIError(s.logger, err).
			Str("product_id", request.ProductId).
			Str("product_user_id", request.Body.ProductUserId).
			Msg("failed to accept organization invitation")
		return nil, err
	}

	return AcceptOrganizationInvitation200JSONResponse(mapInvitationToResponse(invitation)), nil
}

func mapSearchOrganizationInvitationsRequestToInput(
	body SearchOrganizationInvitationsJSONRequestBody,
) search.Request[organizationinvitation.SearchFilter, organizationinvitation.SortField] {
	var req search.Request[organizationinvitation.SearchFilter, organizationinvitation.SortField]

	var filter *organizationinvitation.SearchFilter
	if body.Filter != nil {
		filter = &organizationinvitation.SearchFilter{
			Statuses: functional.FromPtr(body.Filter.Statuses).OrElse(nil),
		}
	}

	return req.WithFilter(filter).
		WithSort(body.SortBy, body.SortDirection).
		WithFullTextSearch(body.FullTextSearch).
		WithPagination(body.Pagination)
}

func mapInvitationToResponse(invitation organizationinvitation.Invitation) OrganizationInvitationResponse {
	return OrganizationInvitationResponse{
		Id:             invitation.ID,
		OrganizationId: invitation.OrganizationID,
		Email:          openapi_types.Email(invitation.Email),
		RoleId:         invitation.RoleID,
		Status:         invitation.Status,
		ExpiresAt:      invitation.ExpiresAt,
		AcceptedAt:     invitation.AcceptedAt,
		CreatedAt:      invitation.CreatedAt,
		UpdatedAt:      invitation.UpdatedAt,
	}
}

func mapCreatedInvitationToResponse(created organizationinvitation.Created) CreatedOrganizationInvitationResponse {
	invitation := mapInvitationToResponse(created.Invitation)
	return CreatedOrganizationInvitationResponse{
		Id:             invitation.Id,
		OrganizationId: invitation.OrganizationId,
		Email:          invitation.Email,
		RoleId:         invitation.RoleId,
		Status:         invitation.Status,
		ExpiresAt:      invitation.ExpiresAt,
		AcceptedAt:     invitation.AcceptedAt,
		CreatedAt:      invitation.CreatedAt,
		UpdatedAt:      invitation.UpdatedAt,
		Token:          created.Token,
	}
}
